package trade

import (
	"context"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// WithTx 返回绑定到给定事务的仓库（供在仓储事务内操作订单表）。
func (r *OrderRepository) WithTx(db *gorm.DB) *OrderRepository { return &OrderRepository{db: db} }

// CreateOrder 创建订单头与明细。
func (r *OrderRepository) CreateOrder(ctx context.Context, order *OrderHeader, items []OrderItem) (*OrderHeader, []OrderItem, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return mapWriteError(err)
		}
		for i := range items {
			items[i].OrderID = order.ID
			if err := tx.Create(&items[i]).Error; err != nil {
				return mapWriteError(err)
			}
		}
		return nil
	})
	return order, items, err
}

// LockOrder 行锁订单（排除已软删）。
func (r *OrderRepository) LockOrder(ctx context.Context, orderID uint64) (*OrderHeader, error) {
	var order OrderHeader
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ? AND status <> ?", orderID, OrderStatusDeleted).First(&order).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &order, nil
}

// Transition 纯状态流转（行锁 + 前置状态校验）。
func (r *OrderRepository) Transition(ctx context.Context, orderID uint64, target OrderStatus) (*OrderHeader, error) {
	var order *OrderHeader
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := r.WithTx(tx)
		o, err := repo.LockOrder(ctx, orderID)
		if err != nil {
			return err
		}
		if !validOrderTransition(o.Status, target) {
			return ErrConflict
		}
		if err := tx.Model(&OrderHeader{}).Where("id = ?", orderID).Update("status", target).Error; err != nil {
			return err
		}
		o.Status = target
		order = o
		return nil
	})
	return order, err
}

// UpdateItemQuantity 成交时更新明细为实成交量。
func (r *OrderRepository) UpdateItemQuantity(ctx context.Context, itemID uint64, quantity decimal.Decimal) error {
	result := r.db.WithContext(ctx).Model(&OrderItem{}).Where("id = ?", itemID).Update("quantity", quantity)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SoftDelete 软删订单（status=DELETED，释放 TRADING 占用）。
func (r *OrderRepository) SoftDelete(ctx context.Context, orderID uint64) error {
	result := r.db.WithContext(ctx).Model(&OrderHeader{}).Where("id = ?", orderID).Update("status", OrderStatusDeleted)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// StocksByMaterials 查询指定物料的启用库存（按 warehouse_id 升序，供成交分配仓库）。
func (r *OrderRepository) StocksByMaterials(ctx context.Context, materialIDs []uint64) ([]Stock, error) {
	var stocks []Stock
	if err := r.db.WithContext(ctx).
		Where("material_id IN ? AND status = ?", materialIDs, StatusActive).
		Order("warehouse_id ASC").Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func validOrderTransition(current, target OrderStatus) bool {
	switch current {
	case OrderStatusPending:
		return target == OrderStatusApproved || target == OrderStatusRejected || target == OrderStatusDeleted
	case OrderStatusApproved:
		return target == OrderStatusTrading
	case OrderStatusTrading:
		return target == OrderStatusClosed
	default:
		return false
	}
}
