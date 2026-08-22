package alert

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/platform/database"
	"github.com/shopspring/decimal"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestCreateTriggeredAlertPersistsOwnerNotificationAndOriginalAgentRequest(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL alert integration test")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL DB: %v", err)
	}
	defer sqlDB.Close()
	if err := database.Migrate(context.Background(), sqlDB); err != nil {
		t.Fatalf("migrate MySQL: %v", err)
	}

	now := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	mobile := "139" + suffix[len(suffix)-8:]
	plotCode := "A3-" + suffix
	userResult := db.Exec("INSERT INTO users(name, mobile, password_hash, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"alert-owner-"+suffix, mobile, "not-used", "ACTIVE", now, now)
	if userResult.Error != nil {
		t.Fatalf("insert owner: %v", userResult.Error)
	}
	var owner struct{ ID uint64 }
	if err := db.Table("users").Select("id").Where("mobile = ?", mobile).Take(&owner).Error; err != nil {
		t.Fatalf("read owner: %v", err)
	}
	plotResult := db.Exec("INSERT INTO plots(owner_id, code, name, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		owner.ID, plotCode, "集成测试地块", "ACTIVE", now, now)
	if plotResult.Error != nil {
		t.Fatalf("insert plot: %v", plotResult.Error)
	}
	var plot struct{ ID uint64 }
	if err := db.Table("plots").Select("id").Where("owner_id = ? AND code = ?", owner.ID, plotCode).Take(&plot).Error; err != nil {
		t.Fatalf("read plot: %v", err)
	}
	rule := Rule{
		PlotID: plot.ID, Name: "soil moisture low", Metric: "soilMoisture",
		ComparisonOperator: OperatorLT, Threshold: decimal.NewFromInt(30), DurationSeconds: 300,
		Hysteresis: decimal.NewFromInt(2), Level: LevelHigh, Enabled: true,
	}
	if err := db.Create(&rule).Error; err != nil {
		t.Fatalf("insert rule: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM outbox_events WHERE aggregate_type = ? AND aggregate_id IN (SELECT CAST(id AS CHAR) FROM alerts WHERE rule_id = ?)", "ALERT", rule.ID)
		db.Exec("DELETE FROM notifications WHERE alert_id IN (SELECT id FROM alerts WHERE rule_id = ?)", rule.ID)
		db.Exec("DELETE FROM alerts WHERE rule_id = ?", rule.ID)
		db.Exec("DELETE FROM alert_rules WHERE id = ?", rule.ID)
		db.Exec("DELETE FROM plots WHERE id = ?", plot.ID)
		db.Exec("DELETE FROM users WHERE id = ?", owner.ID)
	})

	original := json.RawMessage(`{"ruleId":1,"triggerValue":28.6,"extra":{"source":"telemetry"}}`)
	input := TriggerInput{RuleID: rule.ID, TriggerValue: 28.6, TriggeredAt: &now, TraceID: "trace-1", ForwardBody: original}
	repository := NewRepositories(db)
	first, err := repository.CreateTriggeredAlert(context.Background(), input, now)
	if err != nil || !first.Created {
		t.Fatalf("first CreateTriggeredAlert() = (%+v, %v)", first, err)
	}
	if first.OwnerID != owner.ID || first.PlotID != plot.ID || first.PlotCode != plotCode || first.Metric != rule.Metric {
		t.Fatalf("trigger routing metadata = %+v", first)
	}
	second, err := repository.CreateTriggeredAlert(context.Background(), input, now.Add(time.Second))
	if err != nil || second.Created || second.Alert.ID != first.Alert.ID {
		t.Fatalf("duplicate CreateTriggeredAlert() = (%+v, %v), first=%+v", second, err, first)
	}

	var alertCount, notificationCount, outboxCount int64
	db.Table("alerts").Where("id = ?", first.Alert.ID).Count(&alertCount)
	db.Table("notifications").Where("alert_id = ? AND user_id = ? AND channel = ? AND status = ?", first.Alert.ID, owner.ID, "IN_APP", "SENT").Count(&notificationCount)
	db.Table("outbox_events").Where("aggregate_type = ? AND aggregate_id = ? AND event_type = ?", "ALERT", first.Alert.ID, "ALERT_TRIGGERED").Count(&outboxCount)
	if alertCount != 1 || notificationCount != 1 || outboxCount != 1 {
		t.Fatalf("counts alert=%d notification=%d outbox=%d", alertCount, notificationCount, outboxCount)
	}
	var outboxRow struct{ Payload []byte }
	if err := db.Table("outbox_events").Select("payload").Where("aggregate_type = ? AND aggregate_id = ?", "ALERT", first.Alert.ID).Take(&outboxRow).Error; err != nil {
		t.Fatalf("read outbox payload: %v", err)
	}
	var want, got any
	if json.Unmarshal(original, &want) != nil || json.Unmarshal(outboxRow.Payload, &got) != nil || !equalJSON(want, got) {
		t.Fatalf("forwarded payload = %s, want semantic JSON %s", outboxRow.Payload, original)
	}
}

func equalJSON(left, right any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}
