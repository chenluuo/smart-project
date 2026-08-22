package notification

import (
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
)

type Channel string
type Status string

const (
	ChannelInApp  Channel = "IN_APP"
	ChannelSMS    Channel = "SMS"
	ChannelEmail  Channel = "EMAIL"
	StatusPending Status  = "PENDING"
	StatusSent    Status  = "SENT"
	StatusFailed  Status  = "FAILED"
)

type Notification struct {
	ID         uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	AlertID    uint64     `json:"alertId" gorm:"column:alert_id;not null;index:idx_notifications_alert"`
	UserID     uint64     `json:"userId" gorm:"column:user_id;not null;index:idx_notifications_user_created,priority:1"`
	Channel    Channel    `json:"channel" gorm:"size:32;not null"`
	Content    string     `json:"content" gorm:"type:text;not null"`
	Status     Status     `json:"status" gorm:"size:32;not null;index:idx_notifications_status_created,priority:1"`
	RetryCount int        `json:"retryCount" gorm:"column:retry_count;not null;default:0"`
	SentAt     *time.Time `json:"sentAt,omitempty" gorm:"column:sent_at"`
	persistence.Auditable
}

func (Notification) TableName() string { return "notifications" }
