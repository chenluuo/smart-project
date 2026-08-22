package farm

import (
	"context"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
)

type Repositories struct {
	Farms   *persistence.Repository[Farm]
	Members *persistence.Repository[Member]
	Plots   *persistence.Repository[Plot]
	db      *gorm.DB
}

func NewRepositories(db *gorm.DB) Repositories {
	return Repositories{
		Farms: persistence.NewRepository[Farm](db), Members: persistence.NewRepository[Member](db),
		Plots: persistence.NewRepository[Plot](db), db: db,
	}
}

func (r Repositories) FindFarmsByOwner(ctx context.Context, ownerID uint64) ([]Farm, error) {
	var farms []Farm
	err := r.db.WithContext(ctx).Where("owner_id = ?", ownerID).Find(&farms).Error
	return farms, err
}

func (r Repositories) FindPlotsByFarm(ctx context.Context, farmID uint64) ([]Plot, error) {
	var plots []Plot
	err := r.db.WithContext(ctx).Where("farm_id = ?", farmID).Find(&plots).Error
	return plots, err
}
