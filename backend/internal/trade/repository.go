package trade

import (
	"context"
	"errors"
	"strings"

	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository 贸易数据访问层。
type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// 主数据：物料

func (r *Repository) CreateMaterial(ctx context.Context, material *Material) error {
	return normalizeRepositoryError(r.db.WithContext(ctx).Create(material).Error)
}

func (r *Repository) UpdateMaterial(ctx context.Context, material *Material) error {
	result := r.db.WithContext(ctx).Model(&Material{}).Where("id = ?", material.ID).
		Updates(map[string]any{"name": material.Name, "category": material.Category, "unit": material.Unit, "spec": material.Spec})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteMaterial(ctx context.Context, id uint64) error {
	result := r.db.WithContext(ctx).Delete(&Material{}, id)
	if result.Error != nil {
		return normalizeRepositoryError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) FindMaterialByID(ctx context.Context, id uint64) (*Material, error) {
	var material Material
	if err := r.db.WithContext(ctx).First(&material, id).Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return &material, nil
}

func (r *Repository) ListMaterials(ctx context.Context) ([]Material, error) {
	var materials []Material
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&materials).Error; err != nil {
		return nil, err
	}
	return materials, nil
}

func (r *Repository) ListMarketMaterials(ctx context.Context) ([]MarketMaterialView, error) {
	materials, err := r.ListMaterials(ctx)
	if err != nil {
		return nil, err
	}
	if len(materials) == 0 {
		return []MarketMaterialView{}, nil
	}
	materialIDs := make([]uint64, 0, len(materials))
	for _, material := range materials {
		materialIDs = append(materialIDs, material.ID)
	}
	totalByMaterial, err := r.totalByMaterial(ctx, r.db, materialIDs)
	if err != nil {
		return nil, err
	}
	reservedByMaterial, err := r.reservedByMaterial(ctx, r.db, materialIDs)
	if err != nil {
		return nil, err
	}
	views := make([]MarketMaterialView, 0, len(materials))
	for _, material := range materials {
		available := decimalOrZero(totalByMaterial, material.ID).Sub(decimalOrZero(reservedByMaterial, material.ID))
		if available.IsNegative() {
			available = decimal.Zero
		}
		views = append(views, MarketMaterialView{Material: material, Available: available})
	}
	return views, nil
}

// 主数据：仓库

func (r *Repository) CreateWarehouse(ctx context.Context, warehouse *Warehouse) error {
	return normalizeRepositoryError(r.db.WithContext(ctx).Create(warehouse).Error)
}

func (r *Repository) UpdateWarehouse(ctx context.Context, warehouse *Warehouse) error {
	result := r.db.WithContext(ctx).Model(&Warehouse{}).Where("id = ?", warehouse.ID).
		Updates(map[string]any{"name": warehouse.Name, "code": warehouse.Code, "remark": warehouse.Remark})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteWarehouse(ctx context.Context, id uint64) error {
	result := r.db.WithContext(ctx).Delete(&Warehouse{}, id)
	if result.Error != nil {
		return normalizeRepositoryError(result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) FindWarehouseByID(ctx context.Context, id uint64) (*Warehouse, error) {
	var warehouse Warehouse
	if err := r.db.WithContext(ctx).First(&warehouse, id).Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	return &warehouse, nil
}

func (r *Repository) ListWarehouses(ctx context.Context) ([]Warehouse, error) {
	var warehouses []Warehouse
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&warehouses).Error; err != nil {
		return nil, err
	}
	return warehouses, nil
}

// 库存

// StockIn 入库：累加库存并写 IN 流水（事务 + 行锁）。
func (r *Repository) StockIn(ctx context.Context, warehouseID, materialID uint64, quantity decimal.Decimal) (*Stock, error) {
	var stock Stock
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("warehouse_id = ? AND material_id = ?", warehouseID, materialID).
			First(&stock).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				stock = Stock{WarehouseID: warehouseID, MaterialID: materialID, Quantity: decimal.Zero}
			} else {
				return err
			}
		}
		stock.Quantity = stock.Quantity.Add(quantity)
		if err := tx.Save(&stock).Error; err != nil {
			return err
		}
		record := StockRecord{
			WarehouseID: warehouseID, MaterialID: materialID,
			Direction: DirectionIn, Quantity: quantity,
		}
		return tx.Create(&record).Error
	})
	return &stock, normalizeRepositoryError(err)
}

// ListStockViews 返回单仓库存明细 + 物料维度的占用与可用。
func (r *Repository) ListStockViews(ctx context.Context) ([]StockView, error) {
	var rows []struct {
		WarehouseID   uint64          `gorm:"column:warehouse_id"`
		WarehouseName string          `gorm:"column:warehouse_name"`
		MaterialID    uint64          `gorm:"column:material_id"`
		MaterialName  string          `gorm:"column:material_name"`
		Category      string          `gorm:"column:category"`
		Unit          string          `gorm:"column:unit"`
		Quantity      decimal.Decimal `gorm:"column:quantity"`
	}
	err := r.db.WithContext(ctx).Table("stocks s").
		Select("s.warehouse_id, w.name AS warehouse_name, s.material_id, m.name AS material_name, m.category, m.unit, s.quantity").
		Joins("JOIN warehouses w ON w.id = s.warehouse_id").
		Joins("JOIN materials m ON m.id = s.material_id").
		Order("s.material_id ASC, s.warehouse_id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	materialIDs := make([]uint64, 0, len(rows))
	seen := make(map[uint64]struct{}, len(rows))
	for _, row := range rows {
		if _, ok := seen[row.MaterialID]; !ok {
			seen[row.MaterialID] = struct{}{}
			materialIDs = append(materialIDs, row.MaterialID)
		}
	}
	totalByMaterial, err := r.totalByMaterial(ctx, r.db, materialIDs)
	if err != nil {
		return nil, err
	}
	reservedByMaterial, err := r.reservedByMaterial(ctx, r.db, materialIDs)
	if err != nil {
		return nil, err
	}

	views := make([]StockView, 0, len(rows))
	for _, row := range rows {
		reserved := decimalOrZero(reservedByMaterial, row.MaterialID)
		available := decimalOrZero(totalByMaterial, row.MaterialID).Sub(reserved)
		if available.IsNegative() {
			available = decimal.Zero
		}
		views = append(views, StockView{
			WarehouseID: row.WarehouseID, WarehouseName: row.WarehouseName,
			MaterialID: row.MaterialID, MaterialName: row.MaterialName,
			Category: row.Category, Unit: row.Unit,
			Quantity: row.Quantity, Reserved: reserved, Available: available,
		})
	}
	return views, nil
}

// ListStockRecords 出入库流水列表。
func (r *Repository) ListStockRecords(ctx context.Context, filter StockRecordFilter) ([]StockRecordView, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	base := r.db.WithContext(ctx).Table("stock_records sr").
		Joins("JOIN materials m ON m.id = sr.material_id").
		Joins("JOIN warehouses w ON w.id = sr.warehouse_id")
	if filter.MaterialID != 0 {
		base = base.Where("sr.material_id = ?", filter.MaterialID)
	}
	if filter.WarehouseID != 0 {
		base = base.Where("sr.warehouse_id = ?", filter.WarehouseID)
	}
	if filter.Direction != "" {
		base = base.Where("sr.direction = ?", filter.Direction)
	}
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []StockRecordView
	if err := base.Select("sr.*, m.name AS material_name, w.name AS warehouse_name").
		Order("sr.id DESC").Limit(filter.PageSize).Offset((filter.Page - 1) * filter.PageSize).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// 订单

// CreateOrder 发意向：校验可售数量并创建订单与明细。
func (r *Repository) CreateOrder(ctx context.Context, order *OrderHeader, items []OrderItem) (*OrderHeader, []OrderItem, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		materialIDs := make([]uint64, 0, len(items))
		for _, item := range items {
			materialIDs = append(materialIDs, item.MaterialID)
		}
		totalByMaterial, err := r.totalByMaterial(ctx, tx, materialIDs)
		if err != nil {
			return err
		}
		reservedByMaterial, err := r.reservedByMaterial(ctx, tx, materialIDs)
		if err != nil {
			return err
		}
		for _, item := range items {
			available := decimalOrZero(totalByMaterial, item.MaterialID).Sub(decimalOrZero(reservedByMaterial, item.MaterialID))
			if available.LessThan(item.Quantity) {
				return ErrConflict // 意向数量超过可售数量
			}
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		for i := range items {
			items[i].OrderID = order.ID
			if err := tx.Create(&items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return order, items, normalizeRepositoryError(err)
}

// ListOrders 订单列表（CUSTOMER 仅本人，其余全量）。
func (r *Repository) ListOrders(ctx context.Context, filter OrderListFilter) ([]OrderView, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	keyword := strings.TrimSpace(filter.Keyword)

	applyFilter := func(query *gorm.DB) *gorm.DB {
		query = query.Where("oh.status <> ?", OrderStatusDeleted)
		if filter.CustomerID != 0 {
			query = query.Where("oh.customer_id = ?", filter.CustomerID)
		}
		if filter.Status != nil {
			query = query.Where("oh.status = ?", *filter.Status)
		}
		if keyword != "" {
			query = query.Where("oh.order_no LIKE ?", "%"+keyword+"%")
		}
		return query
	}

	countQuery := r.db.WithContext(ctx).Table("order_headers oh")
	countQuery = applyFilter(countQuery)
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	type orderRow struct {
		OrderHeader
		CustomerName string `gorm:"column:customer_name"`
	}
	listQuery := r.db.WithContext(ctx).Table("order_headers oh").
		Select("oh.*, u.name AS customer_name").
		Joins("JOIN users u ON u.id = oh.customer_id")
	listQuery = applyFilter(listQuery)
	var rows []orderRow
	if err := listQuery.Order("oh.id DESC").Limit(filter.PageSize).Offset((filter.Page - 1) * filter.PageSize).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	orderIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		orderIDs = append(orderIDs, row.ID)
	}
	var items []OrderItem
	if len(orderIDs) > 0 {
		if err := r.db.WithContext(ctx).Where("order_id IN ?", orderIDs).Find(&items).Error; err != nil {
			return nil, 0, err
		}
	}
	itemsByOrder := make(map[uint64][]OrderItem, len(rows))
	for _, item := range items {
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
	}

	views := make([]OrderView, 0, len(rows))
	for _, row := range rows {
		its := itemsByOrder[row.ID]
		if its == nil {
			its = []OrderItem{}
		}
		views = append(views, OrderView{OrderHeader: row.OrderHeader, CustomerName: row.CustomerName, Items: its})
	}
	return views, total, nil
}

// GetOrder 订单详情（含明细、物料信息与可用数量）。
func (r *Repository) GetOrder(ctx context.Context, orderID uint64) (*OrderDetail, error) {
	var order OrderHeader
	if err := r.db.WithContext(ctx).Where("status <> ?", OrderStatusDeleted).First(&order, orderID).Error; err != nil {
		return nil, normalizeRepositoryError(err)
	}
	var items []OrderItem
	if err := r.db.WithContext(ctx).Where("order_id = ?", orderID).Find(&items).Error; err != nil {
		return nil, err
	}
	materialIDs := make([]uint64, 0, len(items))
	for _, item := range items {
		materialIDs = append(materialIDs, item.MaterialID)
	}
	var materials []Material
	if len(materialIDs) > 0 {
		if err := r.db.WithContext(ctx).Where("id IN ?", materialIDs).Find(&materials).Error; err != nil {
			return nil, err
		}
	}
	materialByID := make(map[uint64]Material, len(materials))
	for _, material := range materials {
		materialByID[material.ID] = material
	}
	totalByMaterial, err := r.totalByMaterial(ctx, r.db, materialIDs)
	if err != nil {
		return nil, err
	}
	reservedByMaterial, err := r.reservedByMaterial(ctx, r.db, materialIDs)
	if err != nil {
		return nil, err
	}

	var customerName string
	_ = r.db.WithContext(ctx).Table("users").Select("name").Where("id = ?", order.CustomerID).Scan(&customerName).Error

	detail := &OrderDetail{OrderHeader: order, CustomerName: customerName}
	detail.Items = make([]OrderItemDetail, 0, len(items))
	for _, item := range items {
		available := decimalOrZero(totalByMaterial, item.MaterialID).Sub(decimalOrZero(reservedByMaterial, item.MaterialID))
		if available.IsNegative() {
			available = decimal.Zero
		}
		material := materialByID[item.MaterialID]
		detail.Items = append(detail.Items, OrderItemDetail{
			OrderItem:    item,
			MaterialName: material.Name, Category: material.Category, Unit: material.Unit,
			Available: available,
		})
	}
	return detail, nil
}

// Review 审批：PENDING → APPROVED / REJECTED。
func (r *Repository) Review(ctx context.Context, orderID uint64, target OrderStatus) (*OrderHeader, error) {
	return r.transition(ctx, orderID, target)
}

// StartTrade 进入交易：APPROVED → TRADING（开始占用库存）。
func (r *Repository) StartTrade(ctx context.Context, orderID uint64) (*OrderHeader, error) {
	return r.transition(ctx, orderID, OrderStatusTrading)
}

// Terminate 关闭（TRADING → CLOSED）或取消（PENDING → DELETED）。
func (r *Repository) Terminate(ctx context.Context, orderID uint64, target OrderStatus) (*OrderHeader, error) {
	return r.transition(ctx, orderID, target)
}

// transition 纯状态流转（行锁 + 前置状态校验）。
func (r *Repository) transition(ctx context.Context, orderID uint64, target OrderStatus) (*OrderHeader, error) {
	var order OrderHeader
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status <> ?", OrderStatusDeleted).First(&order, orderID).Error; err != nil {
			return err
		}
		if !validTransition(order.Status, target) {
			return ErrConflict
		}
		if err := tx.Model(&OrderHeader{}).Where("id = ?", orderID).Update("status", target).Error; err != nil {
			return err
		}
		order.Status = target
		return nil
	})
	return &order, normalizeRepositoryError(err)
}

// Confirm 成交：行锁 → 校验库存 → 扣库存 → 更新实成交量 → 写 OUT 流水 → 软删。
func (r *Repository) Confirm(ctx context.Context, orderID uint64, actualItems []ConfirmItem) (*OrderHeader, error) {
	var order OrderHeader
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status <> ?", OrderStatusDeleted).First(&order, orderID).Error; err != nil {
			return err
		}
		if order.Status != OrderStatusTrading {
			return ErrConflict
		}
		var items []OrderItem
		if err := tx.Where("order_id = ?", orderID).Find(&items).Error; err != nil {
			return err
		}

		actual := make(map[uint64]decimal.Decimal, len(actualItems))
		for _, item := range actualItems {
			actual[item.MaterialID] = item.Quantity
		}
		if len(actual) != len(items) {
			return ErrInvalidInput
		}
		for _, item := range items {
			qty, ok := actual[item.MaterialID]
			if !ok || qty.LessThanOrEqual(decimal.Zero) || qty.GreaterThan(item.Quantity) {
				return ErrInvalidInput
			}
		}

		materialIDs := make([]uint64, 0, len(items))
		for _, item := range items {
			materialIDs = append(materialIDs, item.MaterialID)
		}
		var stocks []Stock
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("material_id IN ?", materialIDs).Order("warehouse_id ASC").Find(&stocks).Error; err != nil {
			return err
		}
		stockByMaterial := make(map[uint64]decimal.Decimal)
		for _, stock := range stocks {
			stockByMaterial[stock.MaterialID] = decimalOrZero(stockByMaterial, stock.MaterialID).Add(stock.Quantity)
		}
		for _, item := range items {
			if decimalOrZero(stockByMaterial, item.MaterialID).LessThan(actual[item.MaterialID]) {
				return ErrConflict // 库存不足
			}
		}

		// 按 warehouse_id 升序逐仓扣减，并写 OUT 流水。
		for _, item := range items {
			remaining := actual[item.MaterialID]
			for i := range stocks {
				if stocks[i].MaterialID != item.MaterialID || remaining.LessThanOrEqual(decimal.Zero) {
					continue
				}
				deduct := decimal.Min(stocks[i].Quantity, remaining)
				stocks[i].Quantity = stocks[i].Quantity.Sub(deduct)
				if err := tx.Model(&Stock{}).Where("id = ?", stocks[i].ID).Update("quantity", stocks[i].Quantity).Error; err != nil {
					return err
				}
				record := StockRecord{
					WarehouseID: stocks[i].WarehouseID, MaterialID: item.MaterialID,
					Direction: DirectionOut, Quantity: deduct, OrderID: &orderID,
				}
				if err := tx.Create(&record).Error; err != nil {
					return err
				}
				remaining = remaining.Sub(deduct)
			}
		}

		for _, item := range items {
			if err := tx.Model(&OrderItem{}).Where("id = ?", item.ID).Update("quantity", actual[item.MaterialID]).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&OrderHeader{}).Where("id = ?", orderID).Update("status", OrderStatusDeleted).Error; err != nil {
			return err
		}
		order.Status = OrderStatusDeleted
		return nil
	})
	return &order, normalizeRepositoryError(err)
}

// validTransition 订单状态机流转校验。
func validTransition(current, target OrderStatus) bool {
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

// totalByMaterial 按物料聚合总库存。
func (r *Repository) totalByMaterial(ctx context.Context, q *gorm.DB, materialIDs []uint64) (map[uint64]decimal.Decimal, error) {
	query := q.WithContext(ctx).Table("stocks").Select("material_id, COALESCE(SUM(quantity), 0) AS total")
	if len(materialIDs) > 0 {
		query = query.Where("material_id IN ?", materialIDs)
	}
	var rows []struct {
		MaterialID uint64          `gorm:"column:material_id"`
		Total      decimal.Decimal `gorm:"column:total"`
	}
	if err := query.Group("material_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64]decimal.Decimal, len(rows))
	for _, row := range rows {
		result[row.MaterialID] = row.Total
	}
	return result, nil
}

// reservedByMaterial 按物料聚合 TRADING 订单占用。
func (r *Repository) reservedByMaterial(ctx context.Context, q *gorm.DB, materialIDs []uint64) (map[uint64]decimal.Decimal, error) {
	query := q.WithContext(ctx).Table("order_items oi").
		Select("oi.material_id, COALESCE(SUM(oi.quantity), 0) AS reserved").
		Joins("JOIN order_headers oh ON oh.id = oi.order_id").
		Where("oh.status = ?", OrderStatusTrading)
	if len(materialIDs) > 0 {
		query = query.Where("oi.material_id IN ?", materialIDs)
	}
	var rows []struct {
		MaterialID uint64          `gorm:"column:material_id"`
		Reserved   decimal.Decimal `gorm:"column:reserved"`
	}
	if err := query.Group("oi.material_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64]decimal.Decimal, len(rows))
	for _, row := range rows {
		result[row.MaterialID] = row.Reserved
	}
	return result, nil
}

func decimalOrZero(m map[uint64]decimal.Decimal, id uint64) decimal.Decimal {
	if v, ok := m[id]; ok {
		return v
	}
	return decimal.Zero
}

func normalizeRepositoryError(err error) error {
	if err == nil || errors.Is(err, ErrConflict) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidInput) {
		return err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	var mysqlErr *drivermysql.MySQLError
	if errors.As(err, &mysqlErr) {
		switch mysqlErr.Number {
		case 1062: // duplicate key
			return ErrConflict
		case 1451, 1452: // foreign key constraint
			return ErrConflict
		}
	}
	return err
}
