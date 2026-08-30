package trade

import (
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"github.com/shopspring/decimal"
)

// OrderStatus 采购意向订单状态。
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "PENDING"
	OrderStatusApproved  OrderStatus = "APPROVED"
	OrderStatusTrading   OrderStatus = "TRADING"
	OrderStatusConfirmed OrderStatus = "CONFIRMED"
	OrderStatusClosed    OrderStatus = "CLOSED"
	OrderStatusRejected  OrderStatus = "REJECTED"
	OrderStatusDeleted   OrderStatus = "DELETED"
)

// Direction 库存流水方向。
type Direction string

const (
	DirectionIn  Direction = "IN"
	DirectionOut Direction = "OUT"
)

// Warehouse 仓库。
type Warehouse struct {
	ID     uint64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name   string  `json:"name" gorm:"size:128;not null"`
	Code   string  `json:"code" gorm:"size:64;not null;uniqueIndex:uk_warehouses_code"`
	Remark *string `json:"remark,omitempty" gorm:"size:255"`
	persistence.Auditable
}

func (Warehouse) TableName() string { return "warehouses" }

// Material 物料（商品）。
type Material struct {
	ID       uint64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Name     string  `json:"name" gorm:"size:128;not null"`
	Category string  `json:"category" gorm:"size:64;not null;index:idx_materials_category"`
	Unit     string  `json:"unit" gorm:"size:32;not null"`
	Spec     *string `json:"spec,omitempty" gorm:"size:255"`
	persistence.Auditable
}

func (Material) TableName() string { return "materials" }

// Stock 库存（warehouse_id + material_id 联合唯一）。
type Stock struct {
	ID          uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	WarehouseID uint64          `json:"warehouseId" gorm:"column:warehouse_id;not null;uniqueIndex:uk_stocks_warehouse_material,priority:1"`
	MaterialID  uint64          `json:"materialId" gorm:"column:material_id;not null;uniqueIndex:uk_stocks_warehouse_material,priority:2;index:idx_stocks_material"`
	Quantity    decimal.Decimal `json:"quantity" gorm:"type:decimal(18,3);not null"`
	persistence.Auditable
}

func (Stock) TableName() string { return "stocks" }

// OrderHeader 采购意向订单头。
type OrderHeader struct {
	ID           uint64      `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderNo      string      `json:"orderNo" gorm:"column:order_no;size:64;not null;uniqueIndex:uk_order_headers_no"`
	Status       OrderStatus `json:"status" gorm:"size:32;not null;index:idx_order_headers_status"`
	CustomerID   uint64      `json:"customerId" gorm:"column:customer_id;not null;index:idx_order_headers_customer"`
	ExpectedTime *time.Time  `json:"expectedTime,omitempty" gorm:"column:expected_time"`
	Remark       *string     `json:"remark,omitempty" gorm:"size:255"`
	persistence.Auditable
}

func (OrderHeader) TableName() string { return "order_headers" }

// OrderItem 订单明细（quantity 在成交时更新为实成交量）。
type OrderItem struct {
	ID         uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID    uint64          `json:"orderId" gorm:"column:order_id;not null;index:idx_order_items_order"`
	MaterialID uint64          `json:"materialId" gorm:"column:material_id;not null;index:idx_order_items_material"`
	Quantity   decimal.Decimal `json:"quantity" gorm:"type:decimal(18,3);not null"`
	persistence.Auditable
}

func (OrderItem) TableName() string { return "order_items" }

// StockRecord 出入库流水（只追加，无 updated_at）。
type StockRecord struct {
	ID          uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	WarehouseID uint64          `json:"warehouseId" gorm:"column:warehouse_id;not null;index:idx_stock_records_warehouse"`
	MaterialID  uint64          `json:"materialId" gorm:"column:material_id;not null;index:idx_stock_records_material"`
	Direction   Direction       `json:"direction" gorm:"size:16;not null"`
	Quantity    decimal.Decimal `json:"quantity" gorm:"type:decimal(18,3);not null"`
	OrderID     *uint64         `json:"orderId,omitempty" gorm:"column:order_id;index:idx_stock_records_order"`
	CreatedAt   time.Time       `json:"createdAt" gorm:"column:created_at;autoCreateTime"`
}

func (StockRecord) TableName() string { return "stock_records" }
