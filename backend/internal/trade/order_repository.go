package trade

import (
	"context"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// OrderRepository 意向订单仓储（只读查询；写路径由后续订单流转接口补充）。
type OrderRepository struct{ db *gorm.DB }

func NewOrderRepository(db *gorm.DB) *OrderRepository { return &OrderRepository{db: db} }

// orderRow 列表查询用的中间行（header + customer 名）。
type orderRow struct {
	OrderHeader
	CustomerName string `gorm:"column:customer_name"`
}

// ListOrders 意向列表：状态过滤 + 可选归属过滤（CUSTOMER 仅自己），分页。
func (r *OrderRepository) ListOrders(ctx context.Context, f OrderFilter) ([]OrderHeader, []OrderItem, map[uint64]string, int64, error) {
	f.Page, f.PageSize = normalizePage(f.Page, f.PageSize)
	q := r.db.WithContext(ctx).Table("order_headers o").
		Where("o.status <> ?", OrderStatusDeleted)
	if f.Status != nil {
		q = q.Where("o.status = ?", *f.Status)
	}
	if f.CustomerID != nil {
		q = q.Where("o.customer_id = ?", *f.CustomerID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, nil, nil, 0, err
	}
	var rows []orderRow
	if err := q.Select("o.*, u.name AS customer_name").
		Joins("JOIN users u ON u.id = o.customer_id").
		Order("o.created_at DESC, o.id DESC").
		Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).
		Scan(&rows).Error; err != nil {
		return nil, nil, nil, 0, err
	}
	headers := make([]OrderHeader, 0, len(rows))
	names := make(map[uint64]string, len(rows))
	orderIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		headers = append(headers, row.OrderHeader)
		names[row.ID] = row.CustomerName
		orderIDs = append(orderIDs, row.ID)
	}
	items, err := r.itemsByOrderIDs(ctx, orderIDs)
	return headers, items, names, total, err
}

// GetOrder 意向详情：单订单头 + 明细；不存在返回 ErrNotFound。
func (r *OrderRepository) GetOrder(ctx context.Context, id uint64) (*OrderHeader, []OrderItem, string, error) {
	var row orderRow
	if err := r.db.WithContext(ctx).Table("order_headers o").
		Select("o.*, u.name AS customer_name").
		Joins("JOIN users u ON u.id = o.customer_id").
		Where("o.id = ? AND o.status <> ?", id, OrderStatusDeleted).
		Take(&row).Error; err != nil {
		return nil, nil, "", mapNotFound(err)
	}
	items, err := r.itemsByOrderIDs(ctx, []uint64{id})
	if err != nil {
		return nil, nil, "", err
	}
	return &row.OrderHeader, items, row.CustomerName, nil
}

func (r *OrderRepository) itemsByOrderIDs(ctx context.Context, orderIDs []uint64) ([]OrderItem, error) {
	if len(orderIDs) == 0 {
		return nil, nil
	}
	var items []OrderItem
	if err := r.db.WithContext(ctx).Where("order_id IN ?", orderIDs).
		Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// StockTotalsByMaterials 物料库存总量（跨仓库汇总，仅 ACTIVE 库存行）。
func (r *OrderRepository) StockTotalsByMaterials(ctx context.Context, materialIDs []uint64) (map[uint64]decimal.Decimal, error) {
	result := make(map[uint64]decimal.Decimal, len(materialIDs))
	if len(materialIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.WithContext(ctx).Table("stocks").
		Select("material_id, SUM(quantity) AS total").
		Where("material_id IN ? AND status = ?", materialIDs, StatusActive).
		Group("material_id").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var materialID uint64
		var total decimal.Decimal
		if err := rows.Scan(&materialID, &total); err != nil {
			return nil, err
		}
		result[materialID] = total
	}
	return result, rows.Err()
}

// MaterialInfo 物料基础信息（名称/单位），用于订单明细展示。
type MaterialInfo struct {
	Name string
	Unit string
}

func (r *OrderRepository) MaterialInfosByIDs(ctx context.Context, materialIDs []uint64) (map[uint64]MaterialInfo, error) {
	result := make(map[uint64]MaterialInfo, len(materialIDs))
	if len(materialIDs) == 0 {
		return result, nil
	}
	var rows []struct {
		ID   uint64
		Name string
		Unit string
	}
	if err := r.db.WithContext(ctx).Model(&Material{}).
		Select("id, name, unit").
		Where("id IN ? AND status <> ?", materialIDs, StatusDeleted).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.ID] = MaterialInfo{Name: row.Name, Unit: row.Unit}
	}
	return result, nil
}

// ReservedByMaterials 聚合 TRADING（待交易）状态订单对物料的占用量——实现上游预留的 ReservationReader。
func (r *OrderRepository) ReservedByMaterials(ctx context.Context, materialIDs []uint64) (map[uint64]decimal.Decimal, error) {
	result := make(map[uint64]decimal.Decimal, len(materialIDs))
	if len(materialIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.WithContext(ctx).Table("order_items oi").
		Select("oi.material_id, SUM(oi.quantity) AS reserved").
		Joins("JOIN order_headers oh ON oh.id = oi.order_id").
		Where("oh.status = ? AND oi.material_id IN ?", OrderStatusTrading, materialIDs).
		Group("oi.material_id").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var materialID uint64
		var reserved decimal.Decimal
		if err := rows.Scan(&materialID, &reserved); err != nil {
			return nil, err
		}
		result[materialID] = reserved
	}
	return result, rows.Err()
}
