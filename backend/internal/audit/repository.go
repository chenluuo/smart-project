package audit

import (
	"context"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
)

type Repository struct {
	*persistence.Repository[Log]
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{Repository: persistence.NewRepository[Log](db), db: db}
}

func (r *Repository) FindByTraceID(ctx context.Context, traceID string) ([]Log, error) {
	var logs []Log
	err := r.db.WithContext(ctx).Where("trace_id = ?", traceID).Order("created_at").Find(&logs).Error
	return logs, err
}
