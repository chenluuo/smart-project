package control

import (
	"context"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
)

type Repository struct {
	*persistence.Repository[Command]
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{Repository: persistence.NewRepository[Command](db), db: db}
}

func (r *Repository) FindByCommandID(ctx context.Context, commandID string) (*Command, error) {
	return r.findOne(ctx, "command_id = ?", commandID)
}

func (r *Repository) FindByIdempotencyKey(ctx context.Context, key string) (*Command, error) {
	return r.findOne(ctx, "idempotency_key = ?", key)
}

func (r *Repository) FindByDeviceAndStatuses(ctx context.Context, deviceID uint64, statuses []Status) ([]Command, error) {
	var commands []Command
	err := r.db.WithContext(ctx).Where("device_id = ? AND status IN ?", deviceID, statuses).Find(&commands).Error
	return commands, err
}

func (r *Repository) findOne(ctx context.Context, query string, value any) (*Command, error) {
	var command Command
	if err := r.db.WithContext(ctx).Where(query, value).First(&command).Error; err != nil {
		return nil, err
	}
	return &command, nil
}
