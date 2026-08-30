package trade

import (
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"github.com/shopspring/decimal"
)

type MasterStatus string

const (
	StatusActive   MasterStatus = "ACTIVE"
	StatusDisabled MasterStatus = "DISABLED"
	StatusDeleted  MasterStatus = "DELETED"
)

type RecordType string

const (
	RecordTypeIn  RecordType = "IN"
	RecordTypeOut RecordType = "OUT"
)

type ReferenceType string

const (
	ReferenceHarvest    ReferenceType = "HARVEST"
	ReferenceOrder      ReferenceType = "ORDER"
	ReferenceAdjustment ReferenceType = "ADJUSTMENT"
)

type Material struct {
	ID       uint64       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name     string       `json:"name" gorm:"size:128;not null;uniqueIndex:uk_materials_name"`
	Category string       `json:"category" gorm:"size:64;not null"`
	Unit     string       `json:"unit" gorm:"size:32;not null"`
	Spec     *string      `json:"spec,omitempty" gorm:"size:255"`
	Status   MasterStatus `json:"status" gorm:"size:32;not null"`
	persistence.Auditable
}

func (Material) TableName() string { return "materials" }

type Warehouse struct {
	ID       uint64       `json:"id" gorm:"primaryKey;autoIncrement"`
	Name     string       `json:"name" gorm:"size:128;not null;uniqueIndex:uk_warehouses_name"`
	Location *string      `json:"location,omitempty" gorm:"size:255"`
	Status   MasterStatus `json:"status" gorm:"size:32;not null"`
	persistence.Auditable
}

func (Warehouse) TableName() string { return "warehouses" }

type Stock struct {
	ID          uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	WarehouseID uint64          `json:"warehouseId" gorm:"column:warehouse_id;not null;uniqueIndex:uk_stocks_warehouse_material,priority:1"`
	MaterialID  uint64          `json:"materialId" gorm:"column:material_id;not null;uniqueIndex:uk_stocks_warehouse_material,priority:2"`
	Quantity    decimal.Decimal `json:"quantity" gorm:"type:decimal(18,3);not null"`
	Status      MasterStatus    `json:"status" gorm:"size:32;not null"`
	persistence.Auditable
}

func (Stock) TableName() string { return "stocks" }

type StockRecord struct {
	ID          uint64          `json:"id" gorm:"primaryKey;autoIncrement"`
	WarehouseID uint64          `json:"warehouseId" gorm:"column:warehouse_id;not null"`
	MaterialID  uint64          `json:"materialId" gorm:"column:material_id;not null"`
	Type        RecordType      `json:"type" gorm:"size:8;not null"`
	Quantity    decimal.Decimal `json:"quantity" gorm:"type:decimal(18,3);not null"`
	RefType     ReferenceType   `json:"refType" gorm:"column:ref_type;size:32;not null"`
	RefID       string          `json:"refId" gorm:"column:ref_id;size:128;not null"`
	PlotID      *uint64         `json:"plotId,omitempty" gorm:"column:plot_id"`
	OperatorID  uint64          `json:"operatorId" gorm:"column:operator_id;not null"`
	Remark      *string         `json:"remark,omitempty" gorm:"size:500"`
	persistence.Auditable
}

func (StockRecord) TableName() string { return "stock_records" }

type PageFilter struct {
	Keyword  string
	Status   *MasterStatus
	Page     int
	PageSize int
}

type StockFilter struct {
	WarehouseID *uint64
	MaterialID  *uint64
	Page        int
	PageSize    int
}

type RecordFilter struct {
	WarehouseID *uint64
	MaterialID  *uint64
	Type        *RecordType
	PlotID      *uint64
	StartAt     *time.Time
	EndAt       *time.Time
	Page        int
	PageSize    int
}

type StockView struct {
	StockID           uint64          `json:"stockId"`
	WarehouseID       uint64          `json:"warehouseId"`
	WarehouseName     string          `json:"warehouseName"`
	MaterialID        uint64          `json:"materialId"`
	MaterialName      string          `json:"materialName"`
	Unit              string          `json:"unit"`
	TotalQuantity     decimal.Decimal `json:"totalQuantity"`
	ReservedQuantity  decimal.Decimal `json:"reservedQuantity"`
	AvailableQuantity decimal.Decimal `json:"availableQuantity"`
}

type RecordView struct {
	StockRecord
	WarehouseName string `json:"warehouseName"`
	MaterialName  string `json:"materialName"`
	Unit          string `json:"unit"`
}

type ListResult[T any] struct {
	Items    []T   `json:"items"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	Total    int64 `json:"total"`
}

type MaterialInput struct {
	Name, Category, Unit string
	Spec                 *string
	Status               MasterStatus
}

type WarehouseInput struct {
	Name, Location string
	Status         MasterStatus
}

type InboundInput struct {
	WarehouseID    uint64
	MaterialID     uint64
	Quantity       decimal.Decimal
	PlotID         uint64
	Remark         *string
	IdempotencyKey string
	OperatorID     uint64
}

type InboundResult struct {
	RecordID      uint64          `json:"recordId"`
	StockQuantity decimal.Decimal `json:"stockQuantity"`
}

type OutboundItem struct {
	WarehouseID uint64
	MaterialID  uint64
	Quantity    decimal.Decimal
}
