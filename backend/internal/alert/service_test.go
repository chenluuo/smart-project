package alert

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type alertStoreStub struct {
	rules         []Rule
	rows          []AlertListRow
	total         int64
	confirmed     *Alert
	err           error
	ownerID       uint64
	plotID        uint64
	alertID       uint64
	rule          *Rule
	filter        ListFilter
	confirmRemark string
	confirmedAt   time.Time
}

func (s *alertStoreStub) ListRulesByOwner(_ context.Context, ownerID, plotID uint64) ([]Rule, error) {
	s.ownerID, s.plotID = ownerID, plotID
	return s.rules, s.err
}

func (s *alertStoreStub) UpsertRuleByOwner(_ context.Context, ownerID uint64, rule *Rule) error {
	s.ownerID, s.rule = ownerID, rule
	return s.err
}

func (s *alertStoreStub) ListAlertsByOwner(_ context.Context, ownerID uint64, filter ListFilter) ([]AlertListRow, int64, error) {
	s.ownerID, s.filter = ownerID, filter
	return s.rows, s.total, s.err
}

func (s *alertStoreStub) ConfirmAlertByOwner(_ context.Context, ownerID, alertID uint64, remark string, now time.Time) (*Alert, error) {
	s.ownerID, s.alertID, s.confirmRemark, s.confirmedAt = ownerID, alertID, remark, now
	return s.confirmed, s.err
}

func TestListRulesMapsContractFields(t *testing.T) {
	store := &alertStoreStub{rules: []Rule{{
		ID: 2, PlotID: 11, Metric: "soilMoisture", ComparisonOperator: OperatorLT,
		Threshold: decimal.NewFromInt(28), DurationSeconds: 300, Enabled: true, Level: LevelMedium,
	}}}
	result, err := NewService(store).ListRules(context.Background(), 7, 11)
	if err != nil || store.ownerID != 7 || store.plotID != 11 || len(result) != 1 {
		t.Fatalf("ListRules() = (%+v, %v), store=%+v", result, err, store)
	}
	if result[0].Operator != OperatorLT || result[0].Value != 28 || result[0].Unit != "%" {
		t.Fatalf("rule = %+v", result[0])
	}
}

func TestUpsertRuleNormalizesAndValidatesInput(t *testing.T) {
	store := &alertStoreStub{}
	service := NewService(store)
	now := time.Date(2026, 8, 22, 8, 20, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	result, err := service.UpsertRule(context.Background(), 7, 11, 2, RuleInput{
		Metric: " soilMoisture ", Operator: "lt", Value: 28,
		DurationSeconds: 300, Level: "medium", Enabled: true,
	})
	if err != nil || result.ID != 2 || !result.UpdatedAt.Equal(now) {
		t.Fatalf("UpsertRule() = (%+v, %v)", result, err)
	}
	if store.ownerID != 7 || store.rule.PlotID != 11 || store.rule.Metric != "soilMoisture" || store.rule.Level != LevelMedium {
		t.Fatalf("stored rule = %+v", store.rule)
	}
	_, err = service.UpsertRule(context.Background(), 7, 11, 2, RuleInput{Metric: "soilMoisture", Operator: OperatorLT, Level: "urgent"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("invalid UpsertRule() error = %v", err)
	}
}

func TestListDefaultsAndMapsLegacyConfirmedState(t *testing.T) {
	started := time.Date(2026, 8, 22, 8, 20, 0, 0, time.UTC)
	store := &alertStoreStub{rows: []AlertListRow{{
		ID: 3, PlotID: 11, PlotCode: "A3", Metric: "soilMoisture", Operator: OperatorLT,
		Threshold: decimal.NewFromInt(30), DurationSeconds: 300, Level: LevelMedium,
		Status: StatusAcknowledged, TriggerValue: decimal.RequireFromString("28.6"), TriggeredAt: started,
	}}, total: 1}
	result, err := NewService(store).List(context.Background(), 7, ListFilter{})
	if err != nil || store.filter.Page != 1 || store.filter.PageSize != 20 || result.Total != 1 {
		t.Fatalf("List() = (%+v, %v), filter=%+v", result, err, store.filter)
	}
	item := result.Items[0]
	if item.Status != StatusConfirmed || item.Title != "A3 地块湿度偏低" || item.CurrentValue != 28.6 || item.ThresholdValue != 30 {
		t.Fatalf("item = %+v", item)
	}
}

func TestConfirmTrimsRemarkAndMapsStoreErrors(t *testing.T) {
	store := &alertStoreStub{confirmed: &Alert{ID: 3}}
	service := NewService(store)
	now := time.Date(2026, 8, 22, 8, 23, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	result, err := service.Confirm(context.Background(), 7, 3, " 已处理 ")
	if err != nil || result.Status != StatusConfirmed || !result.ConfirmedAt.Equal(now) || store.confirmRemark != "已处理" {
		t.Fatalf("Confirm() = (%+v, %v), store=%+v", result, err, store)
	}
	store.err = gorm.ErrRecordNotFound
	if _, err := service.Confirm(context.Background(), 7, 4, "已处理"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Confirm() error = %v, want ErrNotFound", err)
	}
	store.err = ErrConflict
	if _, err := service.Confirm(context.Background(), 7, 3, "已处理"); !errors.Is(err, ErrConflict) {
		t.Fatalf("Confirm() error = %v, want ErrConflict", err)
	}
}
