package mqttclient

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/platform/database"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestResolveTelemetrySourceFollowsBinding 验证：遥测归属按设备当前有效绑定解析，
// topic 里的 ownerId 不参与匹配（地块转移归属后硬件无需改动，数据自动跟随绑定）。
func TestResolveTelemetrySourceFollowsBinding(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL resolver integration test")
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

	now := time.Now().UTC()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	serialNo := "SN-RES-" + suffix[len(suffix)-8:]

	// 两个用户：oldOwner（初始归属）与 newOwner（转移后归属）
	insertUser := func(name, mobilePrefix string) uint64 {
		result := db.Exec("INSERT INTO users(name, mobile, password_hash, status, created_at, updated_at) VALUES (?, ?, ?, 'ACTIVE', ?, ?)",
			name, mobilePrefix+suffix[len(suffix)-8:], "not-used", now, now)
		if result.Error != nil {
			t.Fatalf("insert user: %v", result.Error)
		}
		var id struct{ ID uint64 }
		if err := db.Raw("SELECT LAST_INSERT_ID() AS id").Scan(&id).Error; err != nil {
			t.Fatalf("last insert id: %v", err)
		}
		return id.ID
	}
	oldOwner := insertUser("res-old-"+suffix, "137")
	newOwner := insertUser("res-new-"+suffix, "136")

	// 地块初始归属 oldOwner
	plotResult := db.Exec("INSERT INTO plots(owner_id, code, name, status, created_at, updated_at) VALUES (?, ?, ?, 'ACTIVE', ?, ?)",
		oldOwner, "RP-"+suffix[len(suffix)-6:], "resolver plot", now, now)
	if plotResult.Error != nil {
		t.Fatalf("insert plot: %v", plotResult.Error)
	}
	var plotID uint64
	if err := db.Raw("SELECT LAST_INSERT_ID() AS id").Scan(&plotID).Error; err != nil {
		t.Fatalf("plot id: %v", err)
	}
	var deviceID uint64
	if err := db.Exec("INSERT INTO devices(device_code, serial_no, name, device_type, status, credential_status, created_at, updated_at) VALUES (?, ?, ?, 'SOIL_SENSOR', 'OFFLINE', 'PENDING', ?, ?)",
		"dev-"+suffix, serialNo, "resolver sensor", now, now).Error; err != nil {
		t.Fatalf("insert device: %v", err)
	}
	if err := db.Raw("SELECT LAST_INSERT_ID() AS id").Scan(&deviceID).Error; err != nil {
		t.Fatalf("device id: %v", err)
	}
	if err := db.Exec("INSERT INTO device_bindings(device_id, plot_id, bound_by, bound_at) VALUES (?, ?, ?, ?)",
		deviceID, plotID, oldOwner, now).Error; err != nil {
		t.Fatalf("insert binding: %v", err)
	}

	resolver := NewGormSourceResolver(db)
	ctx := context.Background()

	// 场景1：topic ownerId = 当前归属（oldOwner）→ 解析成功
	source, err := resolver.ResolveTelemetrySource(ctx, oldOwner, serialNo)
	if err != nil {
		t.Fatalf("resolve with matching owner: %v", err)
	}
	if source.PlotID != plotID || source.OwnerID != oldOwner || source.DeviceID != deviceID {
		t.Fatalf("source = %+v, want plot %d owner %d device %d", source, plotID, oldOwner, deviceID)
	}

	// 场景2：地块转移给 newOwner（模拟管理后台改归属，硬件 topic ownerId 不变）
	if err := db.Model(&struct{}{}).Exec("UPDATE plots SET owner_id = ?, updated_at = ? WHERE id = ?", newOwner, now, plotID).Error; err != nil {
		t.Fatalf("transfer plot: %v", err)
	}
	source, err = resolver.ResolveTelemetrySource(ctx, oldOwner, serialNo) // topic 仍带旧 ownerId
	if err != nil {
		t.Fatalf("resolve after transfer with stale topic owner: %v", err)
	}
	if source.OwnerID != newOwner {
		t.Fatalf("source.OwnerID = %d, want %d (数据应跟随绑定归属)", source.OwnerID, newOwner)
	}
	if source.PlotID != plotID {
		t.Fatalf("source.PlotID = %d, want %d", source.PlotID, plotID)
	}
}
