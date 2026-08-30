package trade

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/chenluuo/smart-project/backend/internal/outbox"
	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound          = errors.New("warehouse resource not found")
	ErrConflict          = errors.New("warehouse resource conflict")
	ErrInvalidInput      = errors.New("invalid warehouse input")
	ErrInsufficientStock = errors.New("insufficient stock")
)

type Repository struct{ db *gorm.DB }

func NewWarehouseRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) WithTx(tx *gorm.DB) *Repository { return &Repository{db: tx} }

func (r *Repository) Transaction(ctx context.Context, fn func(*Repository) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(r.WithTx(tx)) })
}

func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}

func (r *Repository) ListMaterials(ctx context.Context, f PageFilter) (ListResult[Material], error) {
	f.Page, f.PageSize = normalizePage(f.Page, f.PageSize)
	q := r.db.WithContext(ctx).Model(&Material{}).Where("status <> ?", StatusDeleted)
	if f.Keyword != "" {
		q = q.Where("name LIKE ? OR category LIKE ?", "%"+f.Keyword+"%", "%"+f.Keyword+"%")
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult[Material]{}, err
	}
	items := make([]Material, 0)
	if err := q.Order("id ASC").Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).Find(&items).Error; err != nil {
		return ListResult[Material]{}, err
	}
	return ListResult[Material]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

func (r *Repository) GetMaterial(ctx context.Context, id uint64) (*Material, error) {
	var v Material
	if err := r.db.WithContext(ctx).Where("id = ? AND status <> ?", id, StatusDeleted).Take(&v).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &v, nil
}

func (r *Repository) CreateMaterial(ctx context.Context, v *Material) error {
	return mapWriteError(r.db.WithContext(ctx).Create(v).Error)
}
func (r *Repository) UpdateMaterial(ctx context.Context, v *Material) error {
	return mapWriteError(r.db.WithContext(ctx).Save(v).Error)
}

func (r *Repository) DeleteMaterial(ctx context.Context, id uint64) error {
	return r.Transaction(ctx, func(tx *Repository) error {
		if _, err := tx.GetMaterial(ctx, id); err != nil {
			return err
		}
		var count int64
		if err := tx.db.WithContext(ctx).Model(&Stock{}).Where("material_id = ? AND status <> ? AND quantity > 0", id, StatusDeleted).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrConflict
		}
		result := tx.db.WithContext(ctx).Model(&Material{}).Where("id = ? AND status <> ?", id, StatusDeleted).Update("status", StatusDeleted)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *Repository) ListWarehouses(ctx context.Context, f PageFilter) (ListResult[Warehouse], error) {
	f.Page, f.PageSize = normalizePage(f.Page, f.PageSize)
	q := r.db.WithContext(ctx).Model(&Warehouse{}).Where("status <> ?", StatusDeleted)
	if f.Keyword != "" {
		q = q.Where("name LIKE ? OR location LIKE ?", "%"+f.Keyword+"%", "%"+f.Keyword+"%")
	}
	if f.Status != nil {
		q = q.Where("status = ?", *f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult[Warehouse]{}, err
	}
	items := make([]Warehouse, 0)
	if err := q.Order("id ASC").Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).Find(&items).Error; err != nil {
		return ListResult[Warehouse]{}, err
	}
	return ListResult[Warehouse]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, nil
}

func (r *Repository) GetWarehouse(ctx context.Context, id uint64) (*Warehouse, error) {
	var v Warehouse
	if err := r.db.WithContext(ctx).Where("id = ? AND status <> ?", id, StatusDeleted).Take(&v).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &v, nil
}

func (r *Repository) CreateWarehouse(ctx context.Context, v *Warehouse) error {
	return mapWriteError(r.db.WithContext(ctx).Create(v).Error)
}
func (r *Repository) UpdateWarehouse(ctx context.Context, v *Warehouse) error {
	return mapWriteError(r.db.WithContext(ctx).Save(v).Error)
}

func (r *Repository) DeleteWarehouse(ctx context.Context, id uint64) error {
	return r.Transaction(ctx, func(tx *Repository) error {
		if _, err := tx.GetWarehouse(ctx, id); err != nil {
			return err
		}
		var count int64
		if err := tx.db.WithContext(ctx).Model(&Stock{}).Where("warehouse_id = ? AND status <> ? AND quantity > 0", id, StatusDeleted).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return ErrConflict
		}
		result := tx.db.WithContext(ctx).Model(&Warehouse{}).Where("id = ? AND status <> ?", id, StatusDeleted).Update("status", StatusDeleted)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

func (r *Repository) ListStocks(ctx context.Context, f StockFilter) (ListResult[StockView], error) {
	f.Page, f.PageSize = normalizePage(f.Page, f.PageSize)
	q := r.db.WithContext(ctx).Table("stocks s").
		Joins("JOIN warehouses w ON w.id=s.warehouse_id AND w.status <> ?", StatusDeleted).
		Joins("JOIN materials m ON m.id=s.material_id AND m.status <> ?", StatusDeleted).
		Where("s.status = ?", StatusActive)
	if f.WarehouseID != nil {
		q = q.Where("s.warehouse_id = ?", *f.WarehouseID)
	}
	if f.MaterialID != nil {
		q = q.Where("s.material_id = ?", *f.MaterialID)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult[StockView]{}, err
	}
	items := make([]StockView, 0)
	err := q.Select("s.id stock_id, s.warehouse_id, w.name warehouse_name, s.material_id, m.name material_name, m.unit, s.quantity total_quantity").
		Order("s.warehouse_id ASC, s.material_id ASC").Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).Scan(&items).Error
	return ListResult[StockView]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, err
}

func (r *Repository) ListRecords(ctx context.Context, f RecordFilter) (ListResult[RecordView], error) {
	f.Page, f.PageSize = normalizePage(f.Page, f.PageSize)
	q := r.db.WithContext(ctx).Table("stock_records sr")
	if f.WarehouseID != nil {
		q = q.Where("sr.warehouse_id = ?", *f.WarehouseID)
	}
	if f.MaterialID != nil {
		q = q.Where("sr.material_id = ?", *f.MaterialID)
	}
	if f.Type != nil {
		q = q.Where("sr.type = ?", *f.Type)
	}
	if f.PlotID != nil {
		q = q.Where("sr.plot_id = ?", *f.PlotID)
	}
	if f.StartAt != nil {
		q = q.Where("sr.created_at >= ?", *f.StartAt)
	}
	if f.EndAt != nil {
		q = q.Where("sr.created_at <= ?", *f.EndAt)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return ListResult[RecordView]{}, err
	}
	items := make([]RecordView, 0)
	err := q.Select("sr.*, w.name warehouse_name, m.name material_name, m.unit").Joins("JOIN warehouses w ON w.id=sr.warehouse_id").Joins("JOIN materials m ON m.id=sr.material_id").Order("sr.created_at DESC, sr.id DESC").Limit(f.PageSize).Offset((f.Page - 1) * f.PageSize).Scan(&items).Error
	return ListResult[RecordView]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, err
}

func (r *Repository) FindRecordByReference(ctx context.Context, refType ReferenceType, refID string) (*StockRecord, error) {
	var v StockRecord
	if err := r.db.WithContext(ctx).Where("ref_type = ? AND ref_id = ?", refType, refID).Order("id ASC").Take(&v).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &v, nil
}

func (r *Repository) LockStock(ctx context.Context, warehouseID, materialID uint64) (*Stock, error) {
	var stock Stock
	q := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("warehouse_id = ? AND material_id = ?", warehouseID, materialID).Take(&stock)
	if q.Error == nil {
		return &stock, nil
	}
	if !errors.Is(q.Error, gorm.ErrRecordNotFound) {
		return nil, q.Error
	}
	created := Stock{WarehouseID: warehouseID, MaterialID: materialID, Quantity: decimal.Zero, Status: StatusActive}
	if err := r.db.WithContext(ctx).Create(&created).Error; err != nil && !duplicateKey(err) {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where("warehouse_id = ? AND material_id = ?", warehouseID, materialID).Take(&stock).Error; err != nil {
		return nil, err
	}
	return &stock, nil
}

func (r *Repository) GetStock(ctx context.Context, warehouseID, materialID uint64) (*Stock, error) {
	var stock Stock
	if err := r.db.WithContext(ctx).Where("warehouse_id = ? AND material_id = ?", warehouseID, materialID).Take(&stock).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return &stock, nil
}

func (r *Repository) AddStock(ctx context.Context, stockID uint64, quantity decimal.Decimal) error {
	if !quantity.GreaterThan(decimal.Zero) {
		return ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Model(&Stock{}).Where("id = ?", stockID).UpdateColumn("quantity", gorm.Expr("quantity + ?", quantity))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeductStock(ctx context.Context, stockID uint64, quantity decimal.Decimal) error {
	if !quantity.GreaterThan(decimal.Zero) {
		return ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Model(&Stock{}).Where("id = ? AND quantity >= ?", stockID, quantity).UpdateColumn("quantity", gorm.Expr("quantity - ?", quantity))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrInsufficientStock
	}
	return nil
}

func (r *Repository) CreateRecord(ctx context.Context, record *StockRecord) error {
	return mapWriteError(r.db.WithContext(ctx).Create(record).Error)
}

func (r *Repository) PlotExists(ctx context.Context, plotID uint64) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("plots").Where("id = ? AND status = ?", plotID, "ACTIVE").Count(&count).Error
	return count > 0, err
}

func (r *Repository) CreateInboundEvent(ctx context.Context, record *StockRecord, material *Material) error {
	payload, err := json.Marshal(map[string]any{
		"recordId": record.ID, "warehouseId": record.WarehouseID, "materialId": record.MaterialID,
		"materialName": material.Name, "quantity": record.Quantity.StringFixed(3), "unit": material.Unit,
		"plotId": record.PlotID, "operatorId": record.OperatorID, "occurredAt": record.CreatedAt,
	})
	if err != nil {
		return err
	}
	return r.db.WithContext(ctx).Create(&outbox.Event{
		AggregateType: "STOCK", AggregateID: strconv.FormatUint(record.ID, 10), EventType: "STOCK_INBOUND_CREATED",
		Payload: datatypes.JSON(payload), Status: outbox.StatusPending, AvailableAt: record.CreatedAt,
	}).Error
}

func (r *Repository) LockStocksOrdered(ctx context.Context, items []OutboundItem) ([]Stock, error) {
	sorted := append([]OutboundItem(nil), items...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].WarehouseID == sorted[j].WarehouseID {
			return sorted[i].MaterialID < sorted[j].MaterialID
		}
		return sorted[i].WarehouseID < sorted[j].WarehouseID
	})
	stocks := make([]Stock, 0, len(sorted))
	for _, item := range sorted {
		stock, err := r.LockStock(ctx, item.WarehouseID, item.MaterialID)
		if err != nil {
			return nil, err
		}
		stocks = append(stocks, *stock)
	}
	return stocks, nil
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
func duplicateKey(err error) bool {
	var e *drivermysql.MySQLError
	return errors.As(err, &e) && e.Number == 1062
}
func retryableTransactionError(err error) bool {
	var e *drivermysql.MySQLError
	return errors.As(err, &e) && (e.Number == 1205 || e.Number == 1213)
}
func mapWriteError(err error) error {
	if err == nil {
		return nil
	}
	if duplicateKey(err) {
		return fmt.Errorf("%w: duplicate", ErrConflict)
	}
	return err
}
func validMasterStatus(v MasterStatus) bool { return v == StatusActive || v == StatusDisabled }
func clean(value string) string             { return strings.TrimSpace(value) }
