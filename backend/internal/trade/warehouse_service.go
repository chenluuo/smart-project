package trade

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

const maxQuantityScale = 3

type ReservationReader interface {
	ReservedByMaterials(context.Context, []uint64) (map[uint64]decimal.Decimal, error)
}

type NoReservations struct{}

func (NoReservations) ReservedByMaterials(context.Context, []uint64) (map[uint64]decimal.Decimal, error) {
	return map[uint64]decimal.Decimal{}, nil
}

type WarehouseService struct {
	repository   *Repository
	reservations ReservationReader
}

func NewWarehouseService(repository *Repository, reservations ReservationReader) *WarehouseService {
	if reservations == nil {
		reservations = NoReservations{}
	}
	return &WarehouseService{repository: repository, reservations: reservations}
}

func (s *WarehouseService) ListMaterials(ctx context.Context, f PageFilter) (ListResult[Material], error) {
	return s.repository.ListMaterials(ctx, f)
}
func (s *WarehouseService) GetMaterial(ctx context.Context, id uint64) (*Material, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	return s.repository.GetMaterial(ctx, id)
}

func (s *WarehouseService) CreateMaterial(ctx context.Context, in MaterialInput) (*Material, error) {
	if err := validateMaterial(&in); err != nil {
		return nil, err
	}
	v := &Material{Name: clean(in.Name), Category: clean(in.Category), Unit: clean(in.Unit), Spec: trimPointer(in.Spec), Status: in.Status}
	if err := s.repository.CreateMaterial(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}

func (s *WarehouseService) UpdateMaterial(ctx context.Context, id uint64, in MaterialInput) (*Material, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	if err := validateMaterial(&in); err != nil {
		return nil, err
	}
	v, err := s.repository.GetMaterial(ctx, id)
	if err != nil {
		return nil, err
	}
	v.Name, v.Category, v.Unit, v.Spec, v.Status = clean(in.Name), clean(in.Category), clean(in.Unit), trimPointer(in.Spec), in.Status
	if err := s.repository.UpdateMaterial(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}
func (s *WarehouseService) DeleteMaterial(ctx context.Context, id uint64) error {
	if id == 0 {
		return ErrInvalidInput
	}
	return s.repository.DeleteMaterial(ctx, id)
}

func (s *WarehouseService) ListWarehouses(ctx context.Context, f PageFilter) (ListResult[Warehouse], error) {
	return s.repository.ListWarehouses(ctx, f)
}
func (s *WarehouseService) GetWarehouse(ctx context.Context, id uint64) (*Warehouse, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	return s.repository.GetWarehouse(ctx, id)
}
func (s *WarehouseService) CreateWarehouse(ctx context.Context, in WarehouseInput) (*Warehouse, error) {
	if err := validateWarehouse(&in); err != nil {
		return nil, err
	}
	location := trimStringPointer(in.Location)
	v := &Warehouse{Name: clean(in.Name), Location: location, Status: in.Status}
	if err := s.repository.CreateWarehouse(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}
func (s *WarehouseService) UpdateWarehouse(ctx context.Context, id uint64, in WarehouseInput) (*Warehouse, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	if err := validateWarehouse(&in); err != nil {
		return nil, err
	}
	v, err := s.repository.GetWarehouse(ctx, id)
	if err != nil {
		return nil, err
	}
	v.Name, v.Location, v.Status = clean(in.Name), trimStringPointer(in.Location), in.Status
	if err := s.repository.UpdateWarehouse(ctx, v); err != nil {
		return nil, err
	}
	return v, nil
}
func (s *WarehouseService) DeleteWarehouse(ctx context.Context, id uint64) error {
	if id == 0 {
		return ErrInvalidInput
	}
	return s.repository.DeleteWarehouse(ctx, id)
}

func (s *WarehouseService) ListStocks(ctx context.Context, f StockFilter) (ListResult[StockView], error) {
	result, err := s.repository.ListStocks(ctx, f)
	if err != nil {
		return result, err
	}
	ids := make([]uint64, 0, len(result.Items))
	seen := map[uint64]bool{}
	for _, item := range result.Items {
		if !seen[item.MaterialID] {
			ids = append(ids, item.MaterialID)
			seen[item.MaterialID] = true
		}
	}
	reserved, err := s.reservations.ReservedByMaterials(ctx, ids)
	if err != nil {
		return result, fmt.Errorf("read stock reservations: %w", err)
	}
	// Rows are ordered by warehouse_id. Reservations consume stock in that fixed
	// order, so per-row availability is meaningful and aggregate availability
	// remains total minus reserved.
	remaining := make(map[uint64]decimal.Decimal, len(reserved))
	for id, value := range reserved {
		remaining[id] = value
	}
	for i := range result.Items {
		item := &result.Items[i]
		reserve := decimal.Zero
		left := remaining[item.MaterialID]
		if left.GreaterThan(decimal.Zero) {
			reserve = decimal.Min(left, item.TotalQuantity)
			remaining[item.MaterialID] = left.Sub(reserve)
		}
		item.ReservedQuantity = reserve
		item.AvailableQuantity = item.TotalQuantity.Sub(reserve)
		if item.AvailableQuantity.IsNegative() {
			item.AvailableQuantity = decimal.Zero
		}
	}
	return result, nil
}

func (s *WarehouseService) ListRecords(ctx context.Context, f RecordFilter) (ListResult[RecordView], error) {
	return s.repository.ListRecords(ctx, f)
}

func (s *WarehouseService) Inbound(ctx context.Context, in InboundInput) (*InboundResult, error) {
	if err := validateInbound(in); err != nil {
		return nil, err
	}
	var result *InboundResult
	err := s.repository.Transaction(ctx, func(tx *Repository) error {
		if existing, err := tx.FindRecordByReference(ctx, ReferenceHarvest, in.IdempotencyKey); err == nil {
			if existing.WarehouseID != in.WarehouseID || existing.MaterialID != in.MaterialID || existing.PlotID == nil || *existing.PlotID != in.PlotID || !existing.Quantity.Equal(in.Quantity) {
				return ErrConflict
			}
			stock, lockErr := tx.LockStock(ctx, in.WarehouseID, in.MaterialID)
			if lockErr != nil {
				return lockErr
			}
			result = &InboundResult{RecordID: existing.ID, StockQuantity: stock.Quantity}
			return nil
		} else if !errors.Is(err, ErrNotFound) {
			return err
		}
		material, err := tx.GetMaterial(ctx, in.MaterialID)
		if err != nil {
			return err
		}
		if material.Status != StatusActive {
			return ErrConflict
		}
		warehouse, err := tx.GetWarehouse(ctx, in.WarehouseID)
		if err != nil {
			return err
		}
		if warehouse.Status != StatusActive {
			return ErrConflict
		}
		exists, err := tx.PlotExists(ctx, in.PlotID)
		if err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		stock, err := tx.LockStock(ctx, in.WarehouseID, in.MaterialID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		record := &StockRecord{WarehouseID: in.WarehouseID, MaterialID: in.MaterialID, Type: RecordTypeIn, Quantity: in.Quantity, RefType: ReferenceHarvest, RefID: in.IdempotencyKey, PlotID: &in.PlotID, OperatorID: in.OperatorID, Remark: trimPointer(in.Remark)}
		record.CreatedAt, record.UpdatedAt = now, now
		if err := tx.AddStock(ctx, stock.ID, in.Quantity); err != nil {
			return err
		}
		if err := tx.CreateRecord(ctx, record); err != nil {
			return err
		}
		if err := tx.CreateInboundEvent(ctx, record, material); err != nil {
			return err
		}
		result = &InboundResult{RecordID: record.ID, StockQuantity: stock.Quantity.Add(in.Quantity)}
		return nil
	})
	// If another transaction committed the same idempotency key after our
	// initial lookup, our unique-key failure rolls back all local changes. In
	// that case return the original committed result.
	if errors.Is(err, ErrConflict) {
		if existing, findErr := s.repository.FindRecordByReference(ctx, ReferenceHarvest, in.IdempotencyKey); findErr == nil &&
			existing.WarehouseID == in.WarehouseID && existing.MaterialID == in.MaterialID && existing.PlotID != nil && *existing.PlotID == in.PlotID && existing.Quantity.Equal(in.Quantity) {
			if stock, stockErr := s.repository.GetStock(ctx, in.WarehouseID, in.MaterialID); stockErr == nil {
				return &InboundResult{RecordID: existing.ID, StockQuantity: stock.Quantity}, nil
			}
		}
	}
	return result, err
}

// DeductForOrder is transaction-owning for standalone callers. A trade service
// that already owns a transaction can call DeductForOrderWithRepository.
func (s *WarehouseService) DeductForOrder(ctx context.Context, orderNo string, operatorID uint64, items []OutboundItem) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		err = s.repository.Transaction(ctx, func(tx *Repository) error {
			return s.DeductForOrderWithRepository(ctx, tx, orderNo, operatorID, items)
		})
		if err == nil || !retryableTransactionError(err) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 10 * time.Millisecond):
		}
	}
	return err
}

func (s *WarehouseService) DeductForOrderWithRepository(ctx context.Context, tx *Repository, orderNo string, operatorID uint64, items []OutboundItem) error {
	orderNo = strings.TrimSpace(orderNo)
	if orderNo == "" || len(orderNo) > 128 || operatorID == 0 || len(items) == 0 {
		return ErrInvalidInput
	}
	for _, item := range items {
		if item.WarehouseID == 0 || item.MaterialID == 0 || !validQuantity(item.Quantity) {
			return ErrInvalidInput
		}
	}
	stocks, err := tx.LockStocksOrdered(ctx, items)
	if err != nil {
		return err
	}
	for i, stock := range stocks {
		item := orderedItems(items)[i]
		if stock.Quantity.LessThan(item.Quantity) {
			return ErrInsufficientStock
		}
		if err := tx.DeductStock(ctx, stock.ID, item.Quantity); err != nil {
			return err
		}
		record := &StockRecord{WarehouseID: item.WarehouseID, MaterialID: item.MaterialID, Type: RecordTypeOut, Quantity: item.Quantity, RefType: ReferenceOrder, RefID: orderNo, OperatorID: operatorID}
		if err := tx.CreateRecord(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func orderedItems(items []OutboundItem) []OutboundItem {
	result := append([]OutboundItem(nil), items...)
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].WarehouseID < result[i].WarehouseID || result[j].WarehouseID == result[i].WarehouseID && result[j].MaterialID < result[i].MaterialID {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

func validateMaterial(in *MaterialInput) error {
	if in.Status == "" {
		in.Status = StatusActive
	}
	if clean(in.Name) == "" || len([]rune(clean(in.Name))) > 128 || clean(in.Category) == "" || len([]rune(clean(in.Category))) > 64 || clean(in.Unit) == "" || len([]rune(clean(in.Unit))) > 32 || !validMasterStatus(in.Status) {
		return ErrInvalidInput
	}
	if in.Spec != nil && len([]rune(strings.TrimSpace(*in.Spec))) > 255 {
		return ErrInvalidInput
	}
	return nil
}
func validateWarehouse(in *WarehouseInput) error {
	if in.Status == "" {
		in.Status = StatusActive
	}
	if clean(in.Name) == "" || len([]rune(clean(in.Name))) > 128 || len([]rune(strings.TrimSpace(in.Location))) > 255 || !validMasterStatus(in.Status) {
		return ErrInvalidInput
	}
	return nil
}
func validateInbound(in InboundInput) error {
	if in.WarehouseID == 0 || in.MaterialID == 0 || in.PlotID == 0 || in.OperatorID == 0 || strings.TrimSpace(in.IdempotencyKey) == "" || len(in.IdempotencyKey) > 128 || !validQuantity(in.Quantity) {
		return ErrInvalidInput
	}
	return nil
}
func validQuantity(v decimal.Decimal) bool {
	return v.GreaterThan(decimal.Zero) && v.Exponent() >= -maxQuantityScale && v.LessThan(decimal.RequireFromString("1000000000000000"))
}
func trimPointer(v *string) *string {
	if v == nil {
		return nil
	}
	x := strings.TrimSpace(*v)
	if x == "" {
		return nil
	}
	return &x
}
func trimStringPointer(v string) *string { return trimPointer(&v) }
