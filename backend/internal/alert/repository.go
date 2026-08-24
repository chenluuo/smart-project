package alert

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/audit"
	"github.com/chenluuo/smart-project/backend/internal/notification"
	"github.com/chenluuo/smart-project/backend/internal/outbox"
	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repositories struct {
	Rules  *persistence.Repository[Rule]
	Alerts *persistence.Repository[Alert]
	db     *gorm.DB
}

func NewRepositories(db *gorm.DB) Repositories {
	return Repositories{Rules: persistence.NewRepository[Rule](db), Alerts: persistence.NewRepository[Alert](db), db: db}
}

func (r Repositories) FindEnabledRules(ctx context.Context, plotID uint64, metric string) ([]Rule, error) {
	var rules []Rule
	err := r.db.WithContext(ctx).Where("plot_id = ? AND metric = ? AND enabled = ?", plotID, metric, true).Find(&rules).Error
	return rules, err
}

func (r Repositories) FindAlertsByStatus(ctx context.Context, status Status) ([]Alert, error) {
	var alerts []Alert
	err := r.db.WithContext(ctx).Where("status = ?", status).Find(&alerts).Error
	return alerts, err
}

func (r Repositories) ListRulesByOwner(ctx context.Context, ownerID, plotID uint64) ([]Rule, error) {
	var plotCount int64
	if err := r.db.WithContext(ctx).Table("plots").Where("id = ? AND owner_id = ?", plotID, ownerID).Count(&plotCount).Error; err != nil {
		return nil, err
	}
	if plotCount == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var rules []Rule
	err := r.db.WithContext(ctx).Where("plot_id = ?", plotID).Order("id ASC").Find(&rules).Error
	return rules, err
}

func (r Repositories) UpsertRuleByOwner(ctx context.Context, ownerID uint64, rule *Rule, hysteresis *decimal.Decimal, expiresAt time.Time) (RulePersistenceResult, error) {
	var result RulePersistenceResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ownedPlot struct{ ID uint64 }
		if err := tx.Table("plots").Select("id").Where("id = ? AND owner_id = ?", rule.PlotID, ownerID).
			Clauses(clause.Locking{Strength: "UPDATE"}).Take(&ownedPlot).Error; err != nil {
			return err
		}

		var existing Rule
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", rule.ID).First(&existing).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if err := tx.Create(rule).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if existing.PlotID != rule.PlotID {
				return gorm.ErrRecordNotFound
			}
			fields := []string{"name", "metric", "comparison_operator", "threshold", "duration_seconds", "level", "enabled", "updated_at"}
			if hysteresis != nil {
				fields = append(fields, "hysteresis")
			} else {
				rule.Hysteresis = existing.Hysteresis
			}
			if err := tx.Model(&existing).Select(fields).Updates(rule).Error; err != nil {
				return err
			}
			rule.CreatedAt = existing.CreatedAt
		}

		version, err := nextThresholdConfigVersion(tx, rule.PlotID, rule.UpdatedAt)
		if err != nil {
			return err
		}
		result.ConfigVersion = version

		var rules []Rule
		if err := tx.Where("plot_id = ?", rule.PlotID).Order("id ASC").Find(&rules).Error; err != nil {
			return err
		}
		configRules := makeThresholdConfigRules(rules)

		var targets []struct {
			DeviceID uint64
			DeviceSN string
		}
		if err := tx.Table("devices AS d").
			Select("d.id AS device_id, d.serial_no AS device_sn").
			Joins("JOIN device_bindings AS b ON b.device_id = d.id AND b.unbound_at IS NULL").
			Where("b.plot_id = ?", rule.PlotID).Order("d.id ASC").Scan(&targets).Error; err != nil {
			return err
		}

		result.Deliveries = make([]ThresholdDelivery, 0, len(targets))
		for _, target := range targets {
			messageID, err := newThresholdMessageID()
			if err != nil {
				return err
			}
			delivery := ThresholdDelivery{
				MessageID: messageID, PlotID: rule.PlotID, ChangedRuleID: rule.ID,
				DeviceID: target.DeviceID, ConfigVersion: version, Status: ThresholdSyncPending,
				ExpiresAt: expiresAt,
				Auditable: persistence.Auditable{CreatedAt: rule.UpdatedAt, UpdatedAt: rule.UpdatedAt},
			}
			if err := tx.Create(&delivery).Error; err != nil {
				return err
			}
			envelope := thresholdOutboxPayload{
				OwnerID: ownerID, DeviceSN: target.DeviceSN,
				Config: ThresholdConfigMessage{
					MessageID: messageID, PlotID: rule.PlotID, ConfigVersion: version,
					Rules: configRules, IssuedAt: rule.UpdatedAt, ExpiresAt: expiresAt,
				},
			}
			payload, err := json.Marshal(envelope)
			if err != nil {
				return err
			}
			outboxEvent := outbox.Event{
				AggregateType: "threshold_config_delivery", AggregateID: messageID,
				EventType: "THRESHOLD_CONFIG_REQUESTED", Payload: datatypes.JSON(payload),
				Status: outbox.StatusPending, AvailableAt: rule.UpdatedAt,
				Auditable: persistence.Auditable{CreatedAt: rule.UpdatedAt, UpdatedAt: rule.UpdatedAt},
			}
			if err := tx.Create(&outboxEvent).Error; err != nil {
				return err
			}
			result.Deliveries = append(result.Deliveries, delivery)
		}

		resourceID := strconv.FormatUint(rule.ID, 10)
		log := audit.Log{
			ActorID: &ownerID, Action: "THRESHOLD_RULE_UPDATE", ResourceType: "alert_rule", ResourceID: &resourceID,
			Result: "SUCCESS", Auditable: persistence.Auditable{CreatedAt: rule.UpdatedAt, UpdatedAt: rule.UpdatedAt},
		}
		return tx.Create(&log).Error
	})
	return result, err
}

func nextThresholdConfigVersion(tx *gorm.DB, plotID uint64, now time.Time) (uint64, error) {
	var config PlotThresholdConfig
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("plot_id = ?", plotID).Take(&config).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		config = PlotThresholdConfig{
			PlotID: plotID, ConfigVersion: 1,
			Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
		}
		return config.ConfigVersion, tx.Create(&config).Error
	}
	if err != nil {
		return 0, err
	}
	config.ConfigVersion++
	config.UpdatedAt = now
	return config.ConfigVersion, tx.Model(&config).Updates(map[string]any{
		"config_version": config.ConfigVersion, "updated_at": now,
	}).Error
}

func makeThresholdConfigRules(rules []Rule) []ThresholdConfigRule {
	result := make([]ThresholdConfigRule, 0, len(rules))
	for _, rule := range rules {
		value, _ := rule.Threshold.Float64()
		hysteresis, _ := rule.Hysteresis.Float64()
		result = append(result, ThresholdConfigRule{
			ID: rule.ID, Metric: rule.Metric, Operator: rule.ComparisonOperator, Value: value,
			Hysteresis: hysteresis, DurationSeconds: rule.DurationSeconds, Level: rule.Level, Enabled: rule.Enabled,
		})
	}
	return result
}

func newThresholdMessageID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate threshold message id: %w", err)
	}
	return "thr_" + hex.EncodeToString(bytes), nil
}

func (r Repositories) ThresholdSyncByOwner(ctx context.Context, ownerID, plotID, ruleID uint64) (*ThresholdSyncView, error) {
	var ruleCount int64
	if err := r.db.WithContext(ctx).Table("alert_rules AS ar").
		Joins("JOIN plots AS p ON p.id = ar.plot_id").
		Where("ar.id = ? AND ar.plot_id = ? AND p.owner_id = ?", ruleID, plotID, ownerID).
		Count(&ruleCount).Error; err != nil {
		return nil, err
	}
	if ruleCount == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var config PlotThresholdConfig
	if err := r.db.WithContext(ctx).Where("plot_id = ?", plotID).Take(&config).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return &ThresholdSyncView{RuleID: ruleID, Status: ThresholdSyncApplied, Devices: []ThresholdDeviceSyncView{}}, nil
	} else if err != nil {
		return nil, err
	}
	devices := make([]ThresholdDeviceSyncView, 0)
	if err := r.db.WithContext(ctx).Table("threshold_config_deliveries AS t").
		Select("t.device_id, d.serial_no AS device_sn, t.message_id, t.status, t.sent_at, t.acknowledged_at, t.expires_at, t.last_error").
		Joins("JOIN devices AS d ON d.id = t.device_id").
		Where("t.plot_id = ? AND t.config_version = ?", plotID, config.ConfigVersion).
		Order("t.device_id ASC").Scan(&devices).Error; err != nil {
		return nil, err
	}
	return &ThresholdSyncView{
		RuleID: ruleID, ConfigVersion: config.ConfigVersion, Status: aggregateThresholdStatus(devices),
		TargetCount: len(devices), Devices: devices,
	}, nil
}

func (r Repositories) MarkThresholdSent(ctx context.Context, messageID string, now time.Time) error {
	return r.db.WithContext(ctx).Model(&ThresholdDelivery{}).
		Where("message_id = ? AND status = ?", messageID, ThresholdSyncPending).
		Updates(map[string]any{"status": ThresholdSyncSent, "sent_at": now, "last_error": nil, "updated_at": now}).Error
}

func (r Repositories) RecordThresholdPublishFailure(ctx context.Context, messageID, message string, now time.Time) error {
	if len(message) > 500 {
		message = message[:500]
	}
	return r.db.WithContext(ctx).Model(&ThresholdDelivery{}).
		Where("message_id = ? AND status = ?", messageID, ThresholdSyncPending).
		Updates(map[string]any{"last_error": message, "updated_at": now}).Error
}

func (r Repositories) ApplyThresholdAck(ctx context.Context, ownerID uint64, deviceSN string, input ThresholdAckInput, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var delivery ThresholdDelivery
		if err := tx.Table("threshold_config_deliveries AS t").Select("t.*").
			Joins("JOIN devices AS d ON d.id = t.device_id").
			Joins("JOIN plots AS p ON p.id = t.plot_id").
			Where("t.message_id = ? AND t.config_version = ? AND d.serial_no = ? AND p.owner_id = ?",
				input.MessageID, input.ConfigVersion, deviceSN, ownerID).
			Clauses(clause.Locking{Strength: "UPDATE"}).Take(&delivery).Error; err != nil {
			return err
		}
		if delivery.Status == ThresholdSyncApplied || delivery.Status == ThresholdSyncFailed || delivery.Status == ThresholdSyncTimeout {
			if delivery.Status == input.Status {
				return nil
			}
			return ErrConflict
		}
		updates := map[string]any{
			"status": input.Status, "acknowledged_at": now, "updated_at": now,
		}
		if input.Status == ThresholdSyncFailed {
			updates["last_error"] = input.Reason
		} else {
			updates["last_error"] = nil
		}
		return tx.Model(&ThresholdDelivery{}).Where("id = ?", delivery.ID).Updates(updates).Error
	})
}

func (r Repositories) ExpireThresholdDeliveries(ctx context.Context, now time.Time) (int64, error) {
	result := r.db.WithContext(ctx).Model(&ThresholdDelivery{}).
		Where("status IN ? AND expires_at <= ?", []ThresholdSyncStatus{ThresholdSyncPending, ThresholdSyncSent}, now).
		Updates(map[string]any{"status": ThresholdSyncTimeout, "last_error": "device acknowledgement timed out", "updated_at": now})
	return result.RowsAffected, result.Error
}

func (r Repositories) ListAlertsByOwner(ctx context.Context, ownerID uint64, filter ListFilter) ([]AlertListRow, int64, error) {
	query := r.db.WithContext(ctx).Table("alerts AS a").
		Joins("LEFT JOIN alert_rules AS ar ON ar.id = a.rule_id").
		Joins("JOIN plots AS p ON p.id = a.plot_id").
		Where("p.owner_id = ?", ownerID)
	if filter.PlotID != nil {
		query = query.Where("p.id = ?", *filter.PlotID)
	}
	if filter.Status != nil {
		if *filter.Status == StatusConfirmed {
			query = query.Where("a.status IN ?", []Status{StatusConfirmed, StatusAcknowledged})
		} else {
			query = query.Where("a.status = ?", *filter.Status)
		}
	}
	if filter.StartTime != nil {
		query = query.Where("a.triggered_at >= ?", *filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("a.triggered_at <= ?", *filter.EndTime)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []AlertListRow
	err := query.Select(`a.id, a.plot_id, p.code AS plot_code, COALESCE(ar.metric, '') AS metric,
		a.warning_type, COALESCE(ar.comparison_operator, '') AS operator, ar.threshold, COALESCE(ar.duration_seconds, 0) AS duration_seconds,
		a.level, a.status, a.trigger_value, a.triggered_at, a.acknowledged_at,
		a.confirmation_remark, a.resolved_at`).
		Order("a.triggered_at DESC, a.id DESC").
		Limit(filter.PageSize).Offset((filter.Page - 1) * filter.PageSize).
		Scan(&rows).Error
	return rows, total, err
}

func (r Repositories) ConfirmAlertByOwner(ctx context.Context, ownerID, alertID uint64, remark string, now time.Time) (*Alert, error) {
	var result Alert
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("alerts AS a").Select("a.*").
			Joins("JOIN plots AS p ON p.id = a.plot_id").
			Where("a.id = ? AND p.owner_id = ?", alertID, ownerID).
			Clauses(clause.Locking{Strength: "UPDATE"}).Take(&result).Error; err != nil {
			return err
		}
		if result.Status == StatusConfirmed || result.Status == StatusAcknowledged {
			return nil
		}
		if result.Status != StatusActive {
			return ErrConflict
		}
		result.Status = StatusConfirmed
		result.AcknowledgedBy = &ownerID
		result.AcknowledgedAt = &now
		result.ConfirmationRemark = &remark
		result.UpdatedAt = now
		return tx.Model(&Alert{}).Where("id = ?", result.ID).Updates(map[string]any{
			"status": StatusConfirmed, "acknowledged_by": ownerID,
			"acknowledged_at": now, "confirmation_remark": remark, "updated_at": now,
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

type triggerPlot struct {
	OwnerID uint64
	Code    string
}

func (r Repositories) CreateTriggeredAlert(ctx context.Context, input TriggerInput, now time.Time) (*TriggerRecord, error) {
	result := &TriggerRecord{}
	triggeredAt := now
	if input.TriggeredAt != nil {
		triggeredAt = input.TriggeredAt.UTC()
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rule Rule
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", input.RuleID).Take(&rule).Error; err != nil {
			return err
		}
		if !rule.Enabled {
			return ErrConflict
		}

		var plot triggerPlot
		if err := tx.Table("plots").Select("owner_id, code").Where("id = ?", rule.PlotID).Take(&plot).Error; err != nil {
			return err
		}
		result.OwnerID, result.PlotID, result.PlotCode = plot.OwnerID, rule.PlotID, plot.Code
		result.Metric, result.Operator = rule.Metric, rule.ComparisonOperator
		if input.DeviceID != nil {
			var bindingCount int64
			if err := tx.Table("device_bindings").Where(
				"device_id = ? AND plot_id = ? AND unbound_at IS NULL", *input.DeviceID, rule.PlotID,
			).Count(&bindingCount).Error; err != nil {
				return err
			}
			if bindingCount == 0 {
				return gorm.ErrRecordNotFound
			}
		}

		var existing Alert
		err := tx.Where("rule_id = ? AND status IN ?", rule.ID, []Status{StatusActive, StatusConfirmed, StatusAcknowledged}).
			Order("id DESC").Take(&existing).Error
		if err == nil {
			result.Alert, result.Created = existing, false
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		ruleID := rule.ID
		alert := Alert{
			RuleID: &ruleID, PlotID: rule.PlotID, DeviceID: input.DeviceID, Source: SourceRule, Level: rule.Level,
			Status: StatusActive, TriggerValue: decimalFromFloat(input.TriggerValue),
			TriggeredAt: triggeredAt,
			Auditable:   persistence.Auditable{CreatedAt: now, UpdatedAt: now},
		}
		if err := tx.Create(&alert).Error; err != nil {
			return err
		}

		content := fmt.Sprintf("%s 地块 %s 告警：当前值 %s%s，阈值 %s%s", plot.Code, rule.Metric,
			alert.TriggerValue.String(), metricUnit(rule.Metric), rule.Threshold.String(), metricUnit(rule.Metric))
		sentAt := now
		ownerNotification := notification.Notification{
			AlertID: alert.ID, UserID: plot.OwnerID, Channel: notification.ChannelInApp,
			Content: content, Status: notification.StatusSent, SentAt: &sentAt,
			Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
		}
		if err := tx.Create(&ownerNotification).Error; err != nil {
			return err
		}

		event := outbox.Event{
			AggregateType: "ALERT", AggregateID: strconv.FormatUint(alert.ID, 10),
			EventType: "ALERT_TRIGGERED", Payload: datatypes.JSON(append([]byte(nil), input.ForwardBody...)), Status: outbox.StatusPending,
			AvailableAt: now, Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
		}
		if err := tx.Create(&event).Error; err != nil {
			return err
		}
		result.Alert, result.Created = alert, true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// warningSpec 设备上报警告规格（告警类型→指标映射）。
type warningSpec struct {
	kind   WarningType
	metric string
	active bool
	value  float64
}

func (r Repositories) SyncDeviceWarnings(ctx context.Context, input DeviceWarningInput, now time.Time) ([]WarningTransition, error) {
	specs := []warningSpec{
		{kind: WarningTemperature, metric: "temperature", active: input.TemperatureWarning, value: input.Temperature},
		{kind: WarningSoilMoisture, metric: "soilMoisture", active: input.SoilMoistureWarning, value: input.SoilMoisture},
		{kind: WarningLight, metric: "light", active: input.LightWarning, value: input.Light},
	}
	transitions := make([]WarningTransition, 0, len(specs))
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var lockedDevice struct{ ID uint64 }
		if err := tx.Table("devices AS d").Select("d.id").
			Joins("JOIN device_bindings AS b ON b.device_id = d.id AND b.unbound_at IS NULL").
			Joins("JOIN plots AS p ON p.id = b.plot_id").
			Where("d.id = ? AND p.id = ? AND p.owner_id = ?", input.DeviceID, input.PlotID, input.OwnerID).
			Clauses(clause.Locking{Strength: "UPDATE"}).Take(&lockedDevice).Error; err != nil {
			return err
		}
		var plotCode string
		if err := tx.Table("plots").Select("code").Where("id = ?", input.PlotID).Scan(&plotCode).Error; err != nil {
			return err
		}
		for _, spec := range specs {
			dedupKey := fmt.Sprintf("device:%d:%s", input.DeviceID, spec.kind)
			var existing Alert
			err := tx.Where("active_dedup_key = ?", dedupKey).Take(&existing).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if spec.active {
				if err == nil {
					// 已有 ACTIVE 设备告警：更新触发值
					if err := tx.Model(&Alert{}).Where("id = ?", existing.ID).Updates(map[string]any{
						"trigger_value": decimalFromFloat(spec.value), "updated_at": now,
					}).Error; err != nil {
						return err
					}
					// 持续告警再推送给 agent：告警 ACTIVE 期间每 3 分钟推一次
					if shouldRepushDeviceAlert(tx, existing, now) {
						if err := createDeviceAlertOutbox(tx, existing, input, spec, now); err != nil {
							return err
						}
					}
					continue
				}
				kind, key := spec.kind, dedupKey
				alert := Alert{
					PlotID: input.PlotID, DeviceID: &input.DeviceID, Source: SourceDevice, WarningType: &kind,
					ActiveDedupKey: &key, Level: LevelMedium, Status: StatusActive,
					TriggerValue: decimalFromFloat(spec.value), TriggeredAt: input.OccurredAt.UTC(),
					Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
				}
				if err := tx.Create(&alert).Error; err != nil {
					return err
				}
				sentAt := now
				content := fmt.Sprintf("%s 地块设备上报%s警告，当前值 %s%s", plotCode, warningMetricLabel(spec.kind), alert.TriggerValue.String(), metricUnit(spec.metric))
				if err := tx.Create(&notification.Notification{
					AlertID: alert.ID, UserID: input.OwnerID, Channel: notification.ChannelInApp,
					Content: content, Status: notification.StatusSent, SentAt: &sentAt,
					Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
				}).Error; err != nil {
					return err
				}
				// 设备告警推送给 agent：写 outbox（ALERT_ 前缀，alert dispatcher 消费转发）
				if err := createDeviceAlertOutbox(tx, alert, input, spec, now); err != nil {
					return err
				}
				transitions = append(transitions, WarningTransition{Alert: alert, Created: true, OwnerID: input.OwnerID, PlotID: input.PlotID, PlotCode: plotCode, Metric: spec.metric})
				continue
			}
			if err == nil {
				resolvedAt := input.OccurredAt.UTC()
				if err := tx.Model(&Alert{}).Where("id = ?", existing.ID).Updates(map[string]any{
					"status": StatusResolved, "resolved_at": resolvedAt, "active_dedup_key": nil, "updated_at": now,
				}).Error; err != nil {
					return err
				}
				existing.Status, existing.ResolvedAt, existing.ActiveDedupKey = StatusResolved, &resolvedAt, nil
				transitions = append(transitions, WarningTransition{Alert: existing, Recovered: true, OwnerID: input.OwnerID, PlotID: input.PlotID, PlotCode: plotCode, Metric: spec.metric})
			}
		}
		return nil
	})
	return transitions, err
}

func decimalFromFloat(value float64) decimal.Decimal {
	return decimal.NewFromFloat(value)
}

// alertRepushInterval 持续告警再推送间隔：告警 ACTIVE 期间每 3 分钟推一次给 agent。
const alertRepushInterval = 3 * time.Minute

// createDeviceAlertOutbox 写设备告警 outbox 事件（dispatcher 转发给 agent）。
func createDeviceAlertOutbox(tx *gorm.DB, alert Alert, input DeviceWarningInput, spec warningSpec, now time.Time) error {
	devicePayload := map[string]any{
		"alertId": alert.ID, "ownerId": input.OwnerID, "plotId": input.PlotID,
		"deviceId": input.DeviceID, "metric": spec.metric, "level": string(alert.Level),
		"status": string(alert.Status), "triggerValue": spec.value,
		"warningType": string(spec.kind),
	}
	devicePayloadJSON, err := json.Marshal(devicePayload)
	if err != nil {
		return err
	}
	return tx.Create(&outbox.Event{
		AggregateType: "ALERT", AggregateID: strconv.FormatUint(alert.ID, 10),
		EventType: "ALERT_DEVICE_TRIGGERED", Payload: datatypes.JSON(devicePayloadJSON),
		Status: outbox.StatusPending, AvailableAt: now,
		Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
	}).Error
}

// shouldRepushDeviceAlert 判断持续告警是否再次推送：距上次推送已满 alertRepushInterval。
func shouldRepushDeviceAlert(tx *gorm.DB, alert Alert, now time.Time) bool {
	var lastPushAt time.Time
	if err := tx.Table("outbox_events").Select("MAX(available_at)").
		Where("aggregate_type = ? AND aggregate_id = ?", "ALERT", strconv.FormatUint(alert.ID, 10)).
		Scan(&lastPushAt).Error; err != nil {
		return false // 查询失败不推送，避免放大
	}
	return now.Sub(lastPushAt) >= alertRepushInterval
}
