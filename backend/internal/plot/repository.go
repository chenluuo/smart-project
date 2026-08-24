package plot

import (
	"context"
	"strings"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"github.com/shopspring/decimal"
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

// AdminPlotItem 管理后台地块列表项（含归属用户与设备数）。
type AdminPlotItem struct {
	Plot        Plot
	OwnerName   *string
	DeviceCount int64
}

// AdminListFilter 管理后台地块列表过滤条件。
type AdminListFilter struct {
	Keyword  string
	OwnerID  *uint64
	Status   *Status
	Page     int
	PageSize int
}

// CreateInput 管理后台新建地块入参。
type CreateInput struct {
	Code     string
	Name     string
	Area     *decimal.Decimal
	Location *string
	OwnerID  uint64
}

type adminPlotRow struct {
	Plot
	OwnerName   *string `gorm:"column:owner_name"`
	DeviceCount int64   `gorm:"column:device_count"`
}

// AdminList 管理后台全量地块列表（不过滤归属，含 owner 用户名与设备数）。
func (r Repositories) AdminList(ctx context.Context, filter AdminListFilter) ([]AdminPlotItem, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	keyword := strings.TrimSpace(filter.Keyword)

	countQuery := r.db.WithContext(ctx).Model(&Plot{})
	if keyword != "" {
		like := "%" + keyword + "%"
		countQuery = countQuery.Where("(code LIKE ? OR name LIKE ?)", like, like)
	}
	if filter.OwnerID != nil {
		countQuery = countQuery.Where("owner_id = ?", *filter.OwnerID)
	}
	if filter.Status != nil {
		countQuery = countQuery.Where("status = ?", *filter.Status)
	}
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	listQuery := r.db.WithContext(ctx).Table("plots p").
		Select("p.*, u.name AS owner_name, (SELECT COUNT(*) FROM device_bindings b WHERE b.plot_id = p.id AND b.unbound_at IS NULL) AS device_count").
		Joins("LEFT JOIN users u ON u.id = p.owner_id")
	if keyword != "" {
		like := "%" + keyword + "%"
		listQuery = listQuery.Where("(p.code LIKE ? OR p.name LIKE ?)", like, like)
	}
	if filter.OwnerID != nil {
		listQuery = listQuery.Where("p.owner_id = ?", *filter.OwnerID)
	}
	if filter.Status != nil {
		listQuery = listQuery.Where("p.status = ?", *filter.Status)
	}
	var rows []adminPlotRow
	if err := listQuery.Order("p.id ASC").Limit(filter.PageSize).Offset((filter.Page - 1) * filter.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	items := make([]AdminPlotItem, 0, len(rows))
	for index := range rows {
		items = append(items, AdminPlotItem{
			Plot: rows[index].Plot, OwnerName: rows[index].OwnerName, DeviceCount: rows[index].DeviceCount,
		})
	}
	return items, total, nil
}

// FindPlotByID 按 ID 查任意归属的地块（管理后台使用，不做 owner 过滤）。
func (r Repositories) FindPlotByID(ctx context.Context, plotID uint64) (*Plot, error) {
	var result Plot
	if err := r.db.WithContext(ctx).First(&result, plotID).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

// CreatePlot 管理后台新建地块（owner_id 传 0 表示未分配）。
func (r Repositories) CreatePlot(ctx context.Context, input CreateInput) (*Plot, error) {
	now := time.Now().UTC()
	plot := &Plot{
		OwnerID: input.OwnerID, Code: strings.TrimSpace(input.Code), Name: strings.TrimSpace(input.Name),
		Area: input.Area, Location: input.Location, Status: StatusActive,
		Auditable: persistence.Auditable{CreatedAt: now, UpdatedAt: now},
	}
	if err := r.db.WithContext(ctx).Create(plot).Error; err != nil {
		return nil, err
	}
	return plot, nil
}

// AssignOwner 分配/解绑地块归属（ownerID 传 0 表示解绑为未分配）。
func (r Repositories) AssignOwner(ctx context.Context, plotID, ownerID uint64) (*Plot, error) {
	result := r.db.WithContext(ctx).Model(&Plot{}).Where("id = ?", plotID).Update("owner_id", ownerID)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return r.FindPlotByID(ctx, plotID)
}
