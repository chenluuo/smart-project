package alert

import (
	"context"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
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
