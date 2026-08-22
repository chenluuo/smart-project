package outbox

import (
	"context"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
)

type Repository struct {
	*persistence.Repository[Event]
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{Repository: persistence.NewRepository[Event](db), db: db}
}

func (r *Repository) FindAvailable(ctx context.Context, now time.Time, limit int) ([]Event, error) {
	var events []Event
	err := r.db.WithContext(ctx).
		Where("status IN ? AND available_at <= ?", []Status{StatusPending, StatusFailed}, now).
		Order("available_at, id").Limit(limit).Find(&events).Error
	return events, err
}
