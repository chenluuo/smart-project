package notification

import (
	"context"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
)

type Repository struct {
	*persistence.Repository[Notification]
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{Repository: persistence.NewRepository[Notification](db), db: db}
}

func (r *Repository) FindByStatus(ctx context.Context, status Status) ([]Notification, error) {
	var notifications []Notification
	err := r.db.WithContext(ctx).Where("status = ?", status).Order("created_at").Find(&notifications).Error
	return notifications, err
}
