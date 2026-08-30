package trade

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/platform/database"
	"github.com/shopspring/decimal"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestWarehouseInboundIsAtomicAndIdempotentMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL warehouse integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL DB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := database.Migrate(context.Background(), sqlDB); err != nil {
		t.Fatalf("migrate MySQL: %v", err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	now := time.Now().UTC().Truncate(time.Microsecond)
	userResult := db.Exec("INSERT INTO users(name,mobile,password_hash,status,created_at,updated_at) VALUES (?,?,?,'ACTIVE',?,?)", "warehouse-"+suffix, "135"+suffix[len(suffix)-8:], "unused", now, now)
	if userResult.Error != nil {
		t.Fatalf("insert user: %v", userResult.Error)
	}
	var user struct{ ID uint64 }
	if err := db.Raw("SELECT LAST_INSERT_ID() id").Scan(&user).Error; err != nil {
		t.Fatal(err)
	}
	plotResult := db.Exec("INSERT INTO plots(owner_id,code,name,status,created_at,updated_at) VALUES (?,?,?,'ACTIVE',?,?)", user.ID, "WH-"+suffix, "仓储测试地块", now, now)
	if plotResult.Error != nil {
		t.Fatalf("insert plot: %v", plotResult.Error)
	}
	var plot struct{ ID uint64 }
	if err := db.Raw("SELECT LAST_INSERT_ID() id").Scan(&plot).Error; err != nil {
		t.Fatal(err)
	}

	service := NewWarehouseService(NewWarehouseRepository(db), NoReservations{})
	material, err := service.CreateMaterial(context.Background(), MaterialInput{Name: "番茄-" + suffix, Category: "作物", Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	warehouse, err := service.CreateWarehouse(context.Background(), WarehouseInput{Name: "成品仓-" + suffix})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM outbox_events WHERE aggregate_type='STOCK' AND aggregate_id IN (SELECT CAST(id AS CHAR) FROM stock_records WHERE material_id=?)", material.ID)
		db.Exec("DELETE FROM stock_records WHERE material_id=?", material.ID)
		db.Exec("DELETE FROM stocks WHERE material_id=?", material.ID)
		db.Exec("DELETE FROM materials WHERE id=?", material.ID)
		db.Exec("DELETE FROM warehouses WHERE id=?", warehouse.ID)
		db.Exec("DELETE FROM plots WHERE id=?", plot.ID)
		db.Exec("DELETE FROM users WHERE id=?", user.ID)
	})

	input := InboundInput{WarehouseID: warehouse.ID, MaterialID: material.ID, Quantity: decimal.RequireFromString("500.125"), PlotID: plot.ID, OperatorID: user.ID, IdempotencyKey: "harvest-" + suffix}
	first, err := service.Inbound(context.Background(), input)
	if err != nil {
		t.Fatalf("first inbound: %v", err)
	}
	second, err := service.Inbound(context.Background(), input)
	if err != nil {
		t.Fatalf("idempotent inbound: %v", err)
	}
	if first.RecordID != second.RecordID || !second.StockQuantity.Equal(input.Quantity) {
		t.Fatalf("results first=%+v second=%+v", first, second)
	}
	var stocks, records, events int64
	db.Model(&Stock{}).Where("material_id=?", material.ID).Count(&stocks)
	db.Model(&StockRecord{}).Where("material_id=?", material.ID).Count(&records)
	db.Table("outbox_events").Where("aggregate_type='STOCK' AND aggregate_id=?", strconv.FormatUint(first.RecordID, 10)).Count(&events)
	if stocks != 1 || records != 1 || events != 1 {
		t.Fatalf("counts stocks=%d records=%d events=%d", stocks, records, events)
	}

	start := make(chan struct{})
	errorsByWorker := make(chan error, 2)
	var workers sync.WaitGroup
	for index := 0; index < 2; index++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			<-start
			concurrent := input
			concurrent.Quantity = decimal.NewFromInt(10)
			concurrent.IdempotencyKey = "concurrent-" + strconv.Itoa(index) + "-" + suffix
			_, err := service.Inbound(context.Background(), concurrent)
			errorsByWorker <- err
		}(index)
	}
	close(start)
	workers.Wait()
	close(errorsByWorker)
	for workerErr := range errorsByWorker {
		if workerErr != nil {
			t.Fatalf("concurrent inbound: %v", workerErr)
		}
	}
	stock, err := NewWarehouseRepository(db).GetStock(context.Background(), warehouse.ID, material.ID)
	if err != nil || !stock.Quantity.Equal(decimal.RequireFromString("520.125")) {
		t.Fatalf("stock after concurrent inbound = %+v, err=%v", stock, err)
	}

	err = service.DeductForOrder(context.Background(), "ORDER-SHORT-"+suffix, user.ID, []OutboundItem{{WarehouseID: warehouse.ID, MaterialID: material.ID, Quantity: decimal.NewFromInt(1000)}})
	if err != ErrInsufficientStock {
		t.Fatalf("shortage error=%v", err)
	}
	stock, _ = NewWarehouseRepository(db).GetStock(context.Background(), warehouse.ID, material.ID)
	if !stock.Quantity.Equal(decimal.RequireFromString("520.125")) {
		t.Fatalf("shortage changed stock to %s", stock.Quantity)
	}
	var outboundRecords int64
	db.Model(&StockRecord{}).Where("material_id=? AND type='OUT'", material.ID).Count(&outboundRecords)
	if outboundRecords != 0 {
		t.Fatalf("shortage wrote %d OUT records", outboundRecords)
	}

	err = service.DeductForOrder(context.Background(), "ORDER-OK-"+suffix, user.ID, []OutboundItem{{WarehouseID: warehouse.ID, MaterialID: material.ID, Quantity: decimal.RequireFromString("20.125")}})
	if err != nil {
		t.Fatalf("successful outbound: %v", err)
	}
	stock, _ = NewWarehouseRepository(db).GetStock(context.Background(), warehouse.ID, material.ID)
	if !stock.Quantity.Equal(decimal.NewFromInt(500)) {
		t.Fatalf("stock after outbound=%s", stock.Quantity)
	}
}
