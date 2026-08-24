package alert

import (
	"context"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
)

type ThresholdSyncStatus string

const (
	ThresholdSyncPending ThresholdSyncStatus = "PENDING"
	ThresholdSyncSent    ThresholdSyncStatus = "SENT"
	ThresholdSyncApplied ThresholdSyncStatus = "APPLIED"
	ThresholdSyncFailed  ThresholdSyncStatus = "FAILED"
	ThresholdSyncTimeout ThresholdSyncStatus = "TIMEOUT"
)

const defaultThresholdAckTimeout = 2 * time.Minute

type PlotThresholdConfig struct {
	PlotID        uint64 `gorm:"column:plot_id;primaryKey"`
	ConfigVersion uint64 `gorm:"column:config_version;not null"`
	persistence.Auditable
}

func (PlotThresholdConfig) TableName() string { return "plot_threshold_configs" }

type ThresholdDelivery struct {
	ID             uint64              `gorm:"primaryKey;autoIncrement"`
	MessageID      string              `gorm:"column:message_id;size:64;not null;uniqueIndex:uk_threshold_delivery_message"`
	PlotID         uint64              `gorm:"column:plot_id;not null;uniqueIndex:uk_threshold_delivery_device_version,priority:1"`
	ChangedRuleID  uint64              `gorm:"column:changed_rule_id;not null;index:idx_threshold_delivery_rule_version,priority:1"`
	DeviceID       uint64              `gorm:"column:device_id;not null;uniqueIndex:uk_threshold_delivery_device_version,priority:2"`
	ConfigVersion  uint64              `gorm:"column:config_version;not null;uniqueIndex:uk_threshold_delivery_device_version,priority:3;index:idx_threshold_delivery_rule_version,priority:2"`
	Status         ThresholdSyncStatus `gorm:"size:16;not null;index:idx_threshold_delivery_status_expiry,priority:1"`
	ExpiresAt      time.Time           `gorm:"column:expires_at;not null;index:idx_threshold_delivery_status_expiry,priority:2"`
	SentAt         *time.Time          `gorm:"column:sent_at"`
	AcknowledgedAt *time.Time          `gorm:"column:acknowledged_at"`
	LastError      *string             `gorm:"column:last_error;size:500"`
	persistence.Auditable
}

func (ThresholdDelivery) TableName() string { return "threshold_config_deliveries" }

type ThresholdConfigRule struct {
	ID              uint64             `json:"id"`
	Metric          string             `json:"metric"`
	Operator        ComparisonOperator `json:"operator"`
	Value           float64            `json:"value"`
	Hysteresis      float64            `json:"hysteresis"`
	DurationSeconds int                `json:"durationSeconds"`
	Level           Level              `json:"level"`
	Enabled         bool               `json:"enabled"`
}

type ThresholdConfigMessage struct {
	MessageID     string                `json:"messageId"`
	PlotID        uint64                `json:"plotId"`
	ConfigVersion uint64                `json:"configVersion"`
	Rules         []ThresholdConfigRule `json:"rules"`
	IssuedAt      time.Time             `json:"issuedAt"`
	ExpiresAt     time.Time             `json:"expiresAt"`
}

type thresholdOutboxPayload struct {
	OwnerID  uint64                 `json:"ownerId"`
	DeviceSN string                 `json:"deviceSn"`
	Config   ThresholdConfigMessage `json:"config"`
}

type RulePersistenceResult struct {
	ConfigVersion uint64
	Deliveries    []ThresholdDelivery
}

type ThresholdDeviceSyncView struct {
	DeviceID       uint64              `json:"deviceId"`
	DeviceSN       string              `json:"deviceSn"`
	MessageID      string              `json:"messageId"`
	Status         ThresholdSyncStatus `json:"status"`
	SentAt         *time.Time          `json:"sentAt,omitempty"`
	AcknowledgedAt *time.Time          `json:"acknowledgedAt,omitempty"`
	ExpiresAt      time.Time           `json:"expiresAt"`
	LastError      *string             `json:"lastError,omitempty"`
}

type ThresholdSyncView struct {
	RuleID        uint64                    `json:"ruleId"`
	ConfigVersion uint64                    `json:"configVersion"`
	Status        ThresholdSyncStatus       `json:"status"`
	TargetCount   int                       `json:"targetCount"`
	Devices       []ThresholdDeviceSyncView `json:"devices"`
}

type ThresholdAckInput struct {
	MessageID     string
	ConfigVersion uint64
	Status        ThresholdSyncStatus
	Reason        string
}

type thresholdDeliveryStore interface {
	MarkThresholdSent(context.Context, string, time.Time) error
	RecordThresholdPublishFailure(context.Context, string, string, time.Time) error
	ApplyThresholdAck(context.Context, uint64, string, ThresholdAckInput, time.Time) error
	ExpireThresholdDeliveries(context.Context, time.Time) (int64, error)
}

func aggregateThresholdStatus(devices []ThresholdDeviceSyncView) ThresholdSyncStatus {
	if len(devices) == 0 {
		return ThresholdSyncApplied
	}
	allApplied := true
	hasSent := false
	hasFailed := false
	hasTimeout := false
	for _, device := range devices {
		switch device.Status {
		case ThresholdSyncFailed:
			hasFailed = true
			allApplied = false
		case ThresholdSyncTimeout:
			hasTimeout = true
			allApplied = false
		case ThresholdSyncSent:
			hasSent = true
			allApplied = false
		case ThresholdSyncPending:
			allApplied = false
		case ThresholdSyncApplied:
		default:
			allApplied = false
		}
	}
	if hasFailed {
		return ThresholdSyncFailed
	}
	if hasTimeout {
		return ThresholdSyncTimeout
	}
	if allApplied {
		return ThresholdSyncApplied
	}
	if hasSent {
		return ThresholdSyncSent
	}
	return ThresholdSyncPending
}
