package trade

import (
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"github.com/shopspring/decimal"
)

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

// OrderHeader 采购意向单主表（供应链最开端：顾客发意向 → 审批 → 生产 → 成交）。
type OrderHeader struct {
	ID           uint64       `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderNo      string       `json:"orderNo" gorm:"column:order_no;size:32;not null;uniqueIndex:uk_order_headers_order_no"`
	Status       OrderStatus  `json:"status" gorm:"size:16;not null;index:idx_order_headers_status,priority:1"`
	CustomerID   uint64       `json:"customerId" gorm:"column:customer_id;not null;index:idx_order_headers_customer,priority:1"`
	ExpectedTime *time.Time   `json:"expectedTime,omitempty" gorm:"column:expected_time"`
	Remark       *string      `json:"remark,omitempty" gorm:"size:500"`
	persistence.Auditable
}

func (OrderHeader) TableName() string { return "order_headers" }

type OrderItem struct {
	ID          uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	OrderID     uint64          `json:"orderId" gorm:"column:order_id;not null;uniqueIndex:uk_order_items_order_material,priority:1"`
	MaterialID  uint64          `json:"materialId" gorm:"column:material_id;not null;uniqueIndex:uk_order_items_order_material,priority:2"`
	Quantity    decimal.Decimal `json:"quantity" gorm:"type:decimal(18,3);not null"`
	WarehouseID *uint64         `json:"warehouseId,omitempty" gorm:"column:warehouse_id"`
	persistence.Auditable
}

func (OrderItem) TableName() string { return "order_items" }

// OrderFilter 意向列表查询条件。
type OrderFilter struct {
	Status   *OrderStatus
	CustomerID *uint64 // CUSTOMER 归属过滤：仅看自己
	Page     int
	PageSize int
}

type OrderItemView struct {
	MaterialID        uint64          `json:"materialId"`
	MaterialName      string          `json:"materialName"`
	Unit              string          `json:"unit"`
	Quantity          decimal.Decimal `json:"quantity"`
	AvailableQuantity decimal.Decimal `json:"availableQuantity"`
}

type OrderView struct {
	ID           uint64          `json:"id"`
	OrderNo      string          `json:"orderNo"`
	Status       OrderStatus     `json:"status"`
	CustomerID   uint64          `json:"customerId"`
	CustomerName string          `json:"customerName"`
	ExpectedTime *time.Time      `json:"expectedTime,omitempty"`
	Remark       *string         `json:"remark,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	Items        []OrderItemView `json:"items"`
}
