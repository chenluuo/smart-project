package alert

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

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

func (r Repositories) UpsertRuleByOwner(ctx context.Context, ownerID uint64, rule *Rule) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var plotCount int64
		if err := tx.Table("plots").Where("id = ? AND owner_id = ?", rule.PlotID, ownerID).Count(&plotCount).Error; err != nil {
			return err
		}
		if plotCount == 0 {
			return gorm.ErrRecordNotFound
		}

		var existing Rule
		err := tx.Where("id = ?", rule.ID).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tx.Create(rule).Error
		}
		if err != nil {
			return err
		}
		if existing.PlotID != rule.PlotID {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&existing).Select(
			"name", "metric", "comparison_operator", "threshold", "duration_seconds", "hysteresis", "level", "enabled", "updated_at",
		).Updates(rule).Error
	})
}

func (r Repositories) ListAlertsByOwner(ctx context.Context, ownerID uint64, filter ListFilter) ([]AlertListRow, int64, error) {
	query := r.db.WithContext(ctx).Table("alerts AS a").
		Joins("JOIN alert_rules AS ar ON ar.id = a.rule_id").
		Joins("JOIN plots AS p ON p.id = ar.plot_id").
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
	err := query.Select(`a.id, ar.plot_id, p.code AS plot_code, ar.metric,
		ar.comparison_operator AS operator, ar.threshold, ar.duration_seconds,
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
			Joins("JOIN alert_rules AS ar ON ar.id = a.rule_id").
			Joins("JOIN plots AS p ON p.id = ar.plot_id").
			Where("a.id = ? AND p.owner_id = ?", alertID, ownerID).
			Clauses(clause.Locking{Strength: "UPDATE"}).Take(&result).Error; err != nil {
			return err
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

		alert := Alert{
			RuleID: rule.ID, DeviceID: input.DeviceID, Level: rule.Level,
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

func decimalFromFloat(value float64) decimal.Decimal {
	return decimal.NewFromFloat(value)
}
