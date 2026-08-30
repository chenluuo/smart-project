package trade

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

// OrderService 意向订单查询服务（列表/详情 + 占用聚合）。
type OrderService struct {
	repository *OrderRepository
}

func NewOrderService(repository *OrderRepository) *OrderService {
	return &OrderService{repository: repository}
}

// ReservedByMaterials 实现 WarehouseService 的 ReservationReader：TRADING 状态占用量。
func (s *OrderService) ReservedByMaterials(ctx context.Context, materialIDs []uint64) (map[uint64]decimal.Decimal, error) {
	return s.repository.ReservedByMaterials(ctx, materialIDs)
}

// List 意向列表：每单明细 + 该物料可用数量（库存总量 − TRADING 占用）。
func (s *OrderService) List(ctx context.Context, f OrderFilter) (ListResult[OrderView], error) {
	headers, items, names, total, err := s.repository.ListOrders(ctx, f)
	if err != nil {
		return ListResult[OrderView]{}, fmt.Errorf("list orders: %w", err)
	}
	views, err := s.buildViews(ctx, headers, items, names)
	if err != nil {
		return ListResult[OrderView]{}, err
	}
	return ListResult[OrderView]{Items: views, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

// Get 意向详情。
func (s *OrderService) Get(ctx context.Context, id uint64) (*OrderView, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	header, items, customerName, err := s.repository.GetOrder(ctx, id)
	if err != nil {
		return nil, err
	}
	views, err := s.buildViews(ctx, []OrderHeader{*header}, items, map[uint64]string{header.ID: customerName})
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return nil, ErrNotFound
	}
	return &views[0], nil
}

func (s *OrderService) buildViews(ctx context.Context, headers []OrderHeader, items []OrderItem, names map[uint64]string) ([]OrderView, error) {
	if len(headers) == 0 {
		return []OrderView{}, nil
	}
	materialIDs := make([]uint64, 0)
	seen := map[uint64]bool{}
	for _, item := range items {
		if !seen[item.MaterialID] {
			materialIDs = append(materialIDs, item.MaterialID)
			seen[item.MaterialID] = true
		}
	}
	totals, err := s.repository.StockTotalsByMaterials(ctx, materialIDs)
	if err != nil {
		return nil, fmt.Errorf("read stock totals: %w", err)
	}
	reserved, err := s.repository.ReservedByMaterials(ctx, materialIDs)
	if err != nil {
		return nil, fmt.Errorf("read stock reservations: %w", err)
	}
	infos, err := s.repository.MaterialInfosByIDs(ctx, materialIDs)
	if err != nil {
		return nil, fmt.Errorf("read material infos: %w", err)
	}

	itemsByOrder := make(map[uint64][]OrderItem, len(headers))
	for _, item := range items {
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
	}

	views := make([]OrderView, 0, len(headers))
	for _, header := range headers {
		view := OrderView{
			ID: header.ID, OrderNo: header.OrderNo, Status: header.Status,
			CustomerID: header.CustomerID, CustomerName: names[header.ID],
			ExpectedTime: header.ExpectedTime, Remark: header.Remark, CreatedAt: header.CreatedAt,
			Items: make([]OrderItemView, 0, len(itemsByOrder[header.ID])),
		}
		for _, item := range itemsByOrder[header.ID] {
			available := decimal.Zero
			if total, ok := totals[item.MaterialID]; ok {
				available = total.Sub(reserved[item.MaterialID])
				if available.IsNegative() {
					available = decimal.Zero
				}
			}
			info := infos[item.MaterialID]
			view.Items = append(view.Items, OrderItemView{
				MaterialID: item.MaterialID, MaterialName: info.Name, Unit: info.Unit,
				Quantity: item.Quantity, AvailableQuantity: available,
			})
		}
		views = append(views, view)
	}
	return views, nil
}
