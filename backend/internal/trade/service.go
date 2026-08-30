package trade

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrInvalidInput = errors.New("invalid trade input")
	ErrNotFound     = errors.New("trade resource not found")
	ErrConflict     = errors.New("trade state conflict or insufficient stock")
)

// MaterialInput 物料主数据入参。
type MaterialInput struct {
	Name     string
	Category string
	Unit     string
	Spec     *string
}

// WarehouseInput 仓库主数据入参。
type WarehouseInput struct {
	Name   string
	Code   string
	Remark *string
}

// MarketMaterialView 在售商品 + 可售数量。
type MarketMaterialView struct {
	Material
	Available decimal.Decimal `json:"available"`
}

// StockView 库存视图：单仓总量 + 物料维度的占用与可用。
type StockView struct {
	WarehouseID   uint64          `json:"warehouseId"`
	WarehouseName string          `json:"warehouseName"`
	MaterialID    uint64          `json:"materialId"`
	MaterialName  string          `json:"materialName"`
	Category      string          `json:"category"`
	Unit          string          `json:"unit"`
	Quantity      decimal.Decimal `json:"quantity"`
	Reserved      decimal.Decimal `json:"reserved"`
	Available     decimal.Decimal `json:"available"`
}

// OrderItemInput 发意向明细入参。
type OrderItemInput struct {
	MaterialID uint64
	Quantity   decimal.Decimal
}

// CreateOrderInput 发意向入参。
type CreateOrderInput struct {
	CustomerID   uint64
	ExpectedTime *time.Time
	Remark       string
	Items        []OrderItemInput
}

// ConfirmItem 成交实成交量。
type ConfirmItem struct {
	MaterialID uint64
	Quantity   decimal.Decimal
}

// OrderListFilter 订单列表过滤条件。
type OrderListFilter struct {
	Status   *OrderStatus
	Keyword  string
	CustomerID uint64
	Page     int
	PageSize int
}

// OrderView 订单列表项（含顾客名与明细）。
type OrderView struct {
	OrderHeader
	CustomerName string      `json:"customerName"`
	Items        []OrderItem `json:"items"`
}

// OrderItemDetail 订单详情明细（含物料信息与可用数量）。
type OrderItemDetail struct {
	OrderItem
	MaterialName string          `json:"materialName"`
	Category     string          `json:"category"`
	Unit         string          `json:"unit"`
	Available    decimal.Decimal `json:"available"`
}

// OrderDetail 订单详情（含明细与可用数量）。
type OrderDetail struct {
	OrderHeader
	CustomerName string            `json:"customerName"`
	Items        []OrderItemDetail `json:"items"`
}

// StockRecordFilter 流水查询过滤条件。
type StockRecordFilter struct {
	MaterialID  uint64
	WarehouseID uint64
	Direction   string
	Page        int
	PageSize    int
}

// StockRecordView 流水列表项（含物料/仓库名）。
type StockRecordView struct {
	StockRecord
	MaterialName  string `json:"materialName"`
	WarehouseName string `json:"warehouseName"`
}

// Store 贸易数据访问接口（由 Repository 实现，便于测试 mock）。
type Store interface {
	CreateMaterial(context.Context, *Material) error
	UpdateMaterial(context.Context, *Material) error
	DeleteMaterial(context.Context, uint64) error
	FindMaterialByID(context.Context, uint64) (*Material, error)
	ListMaterials(context.Context) ([]Material, error)
	ListMarketMaterials(context.Context) ([]MarketMaterialView, error)

	CreateWarehouse(context.Context, *Warehouse) error
	UpdateWarehouse(context.Context, *Warehouse) error
	DeleteWarehouse(context.Context, uint64) error
	FindWarehouseByID(context.Context, uint64) (*Warehouse, error)
	ListWarehouses(context.Context) ([]Warehouse, error)

	StockIn(context.Context, uint64, uint64, decimal.Decimal) (*Stock, error)
	ListStockViews(context.Context) ([]StockView, error)
	ListStockRecords(context.Context, StockRecordFilter) ([]StockRecordView, int64, error)

	CreateOrder(context.Context, *OrderHeader, []OrderItem) (*OrderHeader, []OrderItem, error)
	ListOrders(context.Context, OrderListFilter) ([]OrderView, int64, error)
	GetOrder(context.Context, uint64) (*OrderDetail, error)
	Review(context.Context, uint64, OrderStatus) (*OrderHeader, error)
	StartTrade(context.Context, uint64) (*OrderHeader, error)
	Terminate(context.Context, uint64, OrderStatus) (*OrderHeader, error)
	Confirm(context.Context, uint64, []ConfirmItem) (*OrderHeader, error)
}

// Service 贸易业务服务。
type Service struct {
	store Store
	now   func() time.Time
}

func NewService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// 主数据：物料

func (s *Service) CreateMaterial(ctx context.Context, input MaterialInput) (*Material, error) {
	input = trimMaterialInput(input)
	if !validMaterialInput(input) {
		return nil, ErrInvalidInput
	}
	material := &Material{Name: input.Name, Category: input.Category, Unit: input.Unit, Spec: input.Spec}
	if err := s.store.CreateMaterial(ctx, material); err != nil {
		return nil, fmt.Errorf("create material: %w", err)
	}
	return material, nil
}

func (s *Service) UpdateMaterial(ctx context.Context, id uint64, input MaterialInput) (*Material, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	input = trimMaterialInput(input)
	if !validMaterialInput(input) {
		return nil, ErrInvalidInput
	}
	if err := s.store.UpdateMaterial(ctx, &Material{ID: id, Name: input.Name, Category: input.Category, Unit: input.Unit, Spec: input.Spec}); err != nil {
		return nil, fmt.Errorf("update material: %w", err)
	}
	return s.store.FindMaterialByID(ctx, id)
}

func (s *Service) DeleteMaterial(ctx context.Context, id uint64) error {
	if id == 0 {
		return ErrInvalidInput
	}
	if err := s.store.DeleteMaterial(ctx, id); err != nil {
		return fmt.Errorf("delete material: %w", err)
	}
	return nil
}

func (s *Service) ListMaterials(ctx context.Context) ([]Material, error) {
	materials, err := s.store.ListMaterials(ctx)
	if err != nil {
		return nil, fmt.Errorf("list materials: %w", err)
	}
	return materials, nil
}

func (s *Service) ListMarketMaterials(ctx context.Context) ([]MarketMaterialView, error) {
	views, err := s.store.ListMarketMaterials(ctx)
	if err != nil {
		return nil, fmt.Errorf("list market materials: %w", err)
	}
	return views, nil
}

// 主数据：仓库

func (s *Service) CreateWarehouse(ctx context.Context, input WarehouseInput) (*Warehouse, error) {
	input = trimWarehouseInput(input)
	if !validWarehouseInput(input) {
		return nil, ErrInvalidInput
	}
	warehouse := &Warehouse{Name: input.Name, Code: input.Code, Remark: input.Remark}
	if err := s.store.CreateWarehouse(ctx, warehouse); err != nil {
		return nil, fmt.Errorf("create warehouse: %w", err)
	}
	return warehouse, nil
}

func (s *Service) UpdateWarehouse(ctx context.Context, id uint64, input WarehouseInput) (*Warehouse, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	input = trimWarehouseInput(input)
	if !validWarehouseInput(input) {
		return nil, ErrInvalidInput
	}
	if err := s.store.UpdateWarehouse(ctx, &Warehouse{ID: id, Name: input.Name, Code: input.Code, Remark: input.Remark}); err != nil {
		return nil, fmt.Errorf("update warehouse: %w", err)
	}
	return s.store.FindWarehouseByID(ctx, id)
}

func (s *Service) DeleteWarehouse(ctx context.Context, id uint64) error {
	if id == 0 {
		return ErrInvalidInput
	}
	if err := s.store.DeleteWarehouse(ctx, id); err != nil {
		return fmt.Errorf("delete warehouse: %w", err)
	}
	return nil
}

func (s *Service) ListWarehouses(ctx context.Context) ([]Warehouse, error) {
	warehouses, err := s.store.ListWarehouses(ctx)
	if err != nil {
		return nil, fmt.Errorf("list warehouses: %w", err)
	}
	return warehouses, nil
}

// 库存

func (s *Service) StockIn(ctx context.Context, warehouseID, materialID uint64, quantity decimal.Decimal) (*Stock, error) {
	if warehouseID == 0 || materialID == 0 || quantity.LessThanOrEqual(decimal.Zero) {
		return nil, ErrInvalidInput
	}
	stock, err := s.store.StockIn(ctx, warehouseID, materialID, quantity)
	if err != nil {
		return nil, fmt.Errorf("stock in: %w", err)
	}
	return stock, nil
}

func (s *Service) ListStocks(ctx context.Context) ([]StockView, error) {
	views, err := s.store.ListStockViews(ctx)
	if err != nil {
		return nil, fmt.Errorf("list stocks: %w", err)
	}
	return views, nil
}

func (s *Service) ListStockRecords(ctx context.Context, filter StockRecordFilter) ([]StockRecordView, int64, error) {
	filter.Direction = strings.ToUpper(strings.TrimSpace(filter.Direction))
	if filter.Direction != "" && filter.Direction != string(DirectionIn) && filter.Direction != string(DirectionOut) {
		return nil, 0, ErrInvalidInput
	}
	views, total, err := s.store.ListStockRecords(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list stock records: %w", err)
	}
	return views, total, nil
}

// 订单

func (s *Service) CreateOrder(ctx context.Context, input CreateOrderInput) (*OrderHeader, []OrderItem, error) {
	if input.CustomerID == 0 || len(input.Items) == 0 {
		return nil, nil, ErrInvalidInput
	}
	seen := make(map[uint64]struct{}, len(input.Items))
	items := make([]OrderItem, 0, len(input.Items))
	for _, item := range input.Items {
		if item.MaterialID == 0 || item.Quantity.LessThanOrEqual(decimal.Zero) {
			return nil, nil, ErrInvalidInput
		}
		if _, dup := seen[item.MaterialID]; dup {
			return nil, nil, ErrInvalidInput
		}
		seen[item.MaterialID] = struct{}{}
		items = append(items, OrderItem{MaterialID: item.MaterialID, Quantity: item.Quantity})
	}
	order := &OrderHeader{
		OrderNo:      newOrderNo(s.now()),
		Status:       OrderStatusPending,
		CustomerID:   input.CustomerID,
		ExpectedTime: input.ExpectedTime,
		Remark:       optionalString(input.Remark),
	}
	order, items, err := s.store.CreateOrder(ctx, order, items)
	if err != nil {
		return nil, nil, fmt.Errorf("create order: %w", err)
	}
	return order, items, nil
}

func (s *Service) ListOrders(ctx context.Context, userID uint64, role string, filter OrderListFilter) ([]OrderView, int64, error) {
	if role == "CUSTOMER" {
		filter.CustomerID = userID
	}
	views, total, err := s.store.ListOrders(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list orders: %w", err)
	}
	return views, total, nil
}

func (s *Service) GetOrder(ctx context.Context, id uint64) (*OrderDetail, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	detail, err := s.store.GetOrder(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	return detail, nil
}

func (s *Service) Review(ctx context.Context, id uint64, approve bool) (*OrderHeader, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	target := OrderStatusRejected
	if approve {
		target = OrderStatusApproved
	}
	order, err := s.store.Review(ctx, id, target)
	if err != nil {
		return nil, fmt.Errorf("review order: %w", err)
	}
	return order, nil
}

func (s *Service) StartTrade(ctx context.Context, id uint64) (*OrderHeader, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	order, err := s.store.StartTrade(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("start trade: %w", err)
	}
	return order, nil
}

func (s *Service) Terminate(ctx context.Context, id uint64, cancel bool) (*OrderHeader, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	target := OrderStatusClosed
	if cancel {
		target = OrderStatusDeleted
	}
	order, err := s.store.Terminate(ctx, id, target)
	if err != nil {
		return nil, fmt.Errorf("terminate order: %w", err)
	}
	return order, nil
}

func (s *Service) Confirm(ctx context.Context, id uint64, items []ConfirmItem) (*OrderHeader, error) {
	if id == 0 || len(items) == 0 {
		return nil, ErrInvalidInput
	}
	for _, item := range items {
		if item.MaterialID == 0 || item.Quantity.LessThanOrEqual(decimal.Zero) {
			return nil, ErrInvalidInput
		}
	}
	order, err := s.store.Confirm(ctx, id, items)
	if err != nil {
		return nil, fmt.Errorf("confirm order: %w", err)
	}
	return order, nil
}

func trimMaterialInput(input MaterialInput) MaterialInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Category = strings.TrimSpace(input.Category)
	input.Unit = strings.TrimSpace(input.Unit)
	return input
}

func validMaterialInput(input MaterialInput) bool {
	if input.Name == "" || len(input.Name) > 128 {
		return false
	}
	if input.Category == "" || len(input.Category) > 64 {
		return false
	}
	if input.Unit == "" || len(input.Unit) > 32 {
		return false
	}
	if input.Spec != nil && len(*input.Spec) > 255 {
		return false
	}
	return true
}

func trimWarehouseInput(input WarehouseInput) WarehouseInput {
	input.Name = strings.TrimSpace(input.Name)
	input.Code = strings.TrimSpace(input.Code)
	return input
}

func validWarehouseInput(input WarehouseInput) bool {
	if input.Name == "" || len(input.Name) > 128 {
		return false
	}
	if input.Code == "" || len(input.Code) > 64 {
		return false
	}
	if input.Remark != nil && len(*input.Remark) > 255 {
		return false
	}
	return true
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func newOrderNo(now time.Time) string {
	var random [4]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "SO" + strconv.FormatInt(now.UnixNano(), 10)
	}
	return "SO" + strconv.FormatInt(now.UnixNano(), 10) + hex.EncodeToString(random[:])
}
