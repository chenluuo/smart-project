package device

import (
	"context"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
)

type Repositories struct {
	Devices  *persistence.Repository[Device]
	Bindings *persistence.Repository[Binding]
	db       *gorm.DB
}

func NewRepositories(db *gorm.DB) Repositories {
	return Repositories{Devices: persistence.NewRepository[Device](db), Bindings: persistence.NewRepository[Binding](db), db: db}
}

func (r Repositories) FindByCode(ctx context.Context, code string) (*Device, error) {
	var result Device
	if err := r.db.WithContext(ctx).Where("device_code = ?", code).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (r Repositories) FindActiveBinding(ctx context.Context, deviceID uint64) (*Binding, error) {
	var result Binding
	if err := r.db.WithContext(ctx).Where("device_id = ? AND unbound_at IS NULL", deviceID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}
