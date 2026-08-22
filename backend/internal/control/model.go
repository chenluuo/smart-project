package control

import (
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/datatypes"
)

type Action string
type Status string

const (
	ActionIrrigationOn  Action = "IRRIGATION_ON"
	ActionIrrigationOff Action = "IRRIGATION_OFF"
	StatusPending       Status = "PENDING"
	StatusRejected      Status = "REJECTED"
	StatusSent          Status = "SENT"
	StatusAcknowledged  Status = "ACKNOWLEDGED"
	StatusSucceeded     Status = "SUCCEEDED"
	StatusFailed        Status = "FAILED"
	StatusTimeout       Status = "TIMEOUT"
	StatusExpired       Status = "EXPIRED"
)

type Command struct {
	ID             uint64         `json:"id" gorm:"primaryKey;autoIncrement"`
	CommandID      string         `json:"commandId" gorm:"column:command_id;size:64;not null;uniqueIndex:uk_device_commands_command_id"`
	DeviceID       uint64         `json:"deviceId" gorm:"column:device_id;not null;index:idx_device_commands_device_status,priority:1"`
	PlotID         uint64         `json:"plotId" gorm:"column:plot_id;not null;index:idx_device_commands_plot_created,priority:1"`
	IssuedBy       uint64         `json:"issuedBy" gorm:"column:issued_by;not null;index:idx_device_commands_issuer"`
	Action         Action         `json:"action" gorm:"size:32;not null"`
	ParametersJSON datatypes.JSON `json:"parameters" gorm:"column:parameters_json;type:json;not null"`
	IdempotencyKey string         `json:"idempotencyKey" gorm:"column:idempotency_key;size:64;not null;uniqueIndex:uk_device_commands_idempotency"`
	Status         Status         `json:"status" gorm:"size:32;not null;index:idx_device_commands_device_status,priority:2"`
	ErrorCode      *string        `json:"errorCode,omitempty" gorm:"column:error_code;size:64"`
	ErrorMessage   *string        `json:"errorMessage,omitempty" gorm:"column:error_message;size:500"`
	IssuedAt       time.Time      `json:"issuedAt" gorm:"column:issued_at;not null"`
	ExpiresAt      time.Time      `json:"expiresAt" gorm:"column:expires_at;not null"`
	ExecutedAt     *time.Time     `json:"executedAt,omitempty" gorm:"column:executed_at"`
	persistence.Auditable
}

func (Command) TableName() string { return "device_commands" }
