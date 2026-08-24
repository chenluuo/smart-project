package plot

import (
	"context"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
)

type Repositories struct {
	Plots *persistence.Repository[Plot]
	db    *gorm.DB
}

func NewRepositories(db *gorm.DB) Repositories {
	return Repositories{Plots: persistence.NewRepository[Plot](db), db: db}
}

func (r Repositories) FindByOwner(ctx context.Context, ownerID uint64) ([]Plot, error) {
	var plots []Plot
	err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Order("code ASC, id ASC").Find(&plots).Error
	return plots, err
}

func (r Repositories) FindByIDAndOwner(ctx context.Context, id, ownerID uint64) (*Plot, error) {
	var result Plot
	if err := r.db.WithContext(ctx).Where("id = ? AND owner_id = ?", id, ownerID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

func (r Repositories) UpdateCrop(ctx context.Context, plotID, ownerID uint64, cropType string, plantingTime time.Time) error {
	result := r.db.WithContext(ctx).
		Model(&Plot{}).
		Where("id = ? AND owner_id = ?", plotID, ownerID).
		Updates(map[string]interface{}{
			"crop_type":     cropType,
			"planting_time": plantingTime,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
