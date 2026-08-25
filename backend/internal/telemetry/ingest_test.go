package telemetry

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/alert"
)

type latestStoreStub struct{ written Latest }

func (s *latestStoreStub) LatestByPlot(context.Context, uint64) (*Latest, error) {
	return nil, ErrNotFound
}
func (s *latestStoreStub) LatestByPlots(context.Context, []uint64) ([]Latest, error) { return nil, nil }
func (s *latestStoreStub) PutLatest(_ context.Context, value Latest) error {
	s.written = value
	return nil
}

type activityWriterStub struct {
	ownerID, deviceID uint64
	at                time.Time
}

func (s *activityWriterStub) MarkActive(_ context.Context, ownerID, deviceID uint64, at time.Time) error {
	s.ownerID, s.deviceID, s.at = ownerID, deviceID, at
	return nil
}

type warningSynchronizerStub struct{ input alert.DeviceWarningInput }

func (s *warningSynchronizerStub) SyncDeviceWarnings(_ context.Context, input alert.DeviceWarningInput) ([]alert.WarningTransition, error) {
	s.input = input
	return nil, nil
}

func TestIngestAcceptsOnlyMeasurementsAndServerSuppliesRouting(t *testing.T) {
	latest, activity, warnings := &latestStoreStub{}, &activityWriterStub{}, &warningSynchronizerStub{}
	service := NewIngestService(latest, activity, warnings, nil)
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	payload := Payload{
		Temperature: float64Ptr(26.5), SoilMoisture: float64Ptr(31.2), Light: float64Ptr(880),
		TemperatureWarning: boolPtr(true),
	}
	result, err := service.Ingest(context.Background(), TrustedSource{OwnerID: 7, PlotID: 11, PlotCode: "A3", DeviceID: 31}, payload)
	if err != nil || result.Light == nil || result.Light.Value != 880 || !result.Warnings.Temperature {
		t.Fatalf("Ingest() = (%+v, %v)", result, err)
	}
	if warnings.input.OwnerID != 7 || warnings.input.DeviceID != 31 || !warnings.input.OccurredAt.Equal(now) || activity.deviceID != 31 || !latest.written.SampleTime.Equal(now) {
		t.Fatalf("routing warning=%+v activity=%+v latest=%+v", warnings.input, activity, latest.written)
	}
	raw, _ := json.Marshal(payload)
	for _, forbidden := range []string{"deviceId", "ownerId", "plotId", "receivedAt", "status", "battery", "signal"} {
		if json.Valid(raw) && containsJSONField(raw, forbidden) {
			t.Fatalf("device payload contains forbidden field %q: %s", forbidden, raw)
		}
	}
}

func TestIngestSkipsNilMetrics(t *testing.T) {
	latest, activity, warnings := &latestStoreStub{}, &activityWriterStub{}, &warningSynchronizerStub{}
	service := NewIngestService(latest, activity, warnings, nil)
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	// 土壤传感器只上报 soilMoisture（单参数设备）
	payload := Payload{SoilMoisture: float64Ptr(45.0), SoilMoistureWarning: boolPtr(false)}
	result, err := service.Ingest(context.Background(), TrustedSource{OwnerID: 7, PlotID: 11, PlotCode: "A3", DeviceID: 31}, payload)
	if err != nil {
		t.Fatalf("Ingest(single metric) = %v", err)
	}
	if result.SoilMoisture == nil || result.SoilMoisture.Value != 45.0 {
		t.Fatalf("soil moisture = %+v, want 45", result.SoilMoisture)
	}
	if result.Temperature != nil || result.Light != nil {
		t.Fatalf("unreported metrics should be nil, got %+v", result)
	}
	if warnings.input.SoilMoisture == nil || warnings.input.Temperature != nil || warnings.input.Light != nil {
		t.Fatalf("warnings input should only carry reported metric: %+v", warnings.input)
	}
}

func TestIngestEmptyPayloadIsHeartbeat(t *testing.T) {
	latest, activity, warnings := &latestStoreStub{}, &activityWriterStub{}, &warningSynchronizerStub{}
	service := NewIngestService(latest, activity, warnings, nil)
	now := time.Date(2026, 8, 23, 2, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	// 执行器心跳：全空指标 payload（如水泵），只保活不写数据
	result, err := service.Ingest(context.Background(), TrustedSource{OwnerID: 7, PlotID: 11, PlotCode: "A3", DeviceID: 31}, Payload{})
	if err != nil {
		t.Fatalf("Ingest(heartbeat) = %v", err)
	}
	if activity.deviceID != 31 || !activity.at.Equal(now) {
		t.Fatalf("heartbeat should mark device active: %+v", activity)
	}
	if latest.written.PlotID != 0 {
		t.Fatalf("heartbeat must not write latest, got %+v", latest.written)
	}
	if warnings.input.DeviceID != 0 {
		t.Fatalf("heartbeat must not sync warnings, got %+v", warnings.input)
	}
	if result.SoilMoisture != nil || result.Temperature != nil || result.Light != nil {
		t.Fatalf("heartbeat result should have no metrics: %+v", result)
	}
}

func containsJSONField(raw []byte, field string) bool {
	var value map[string]any
	_ = json.Unmarshal(raw, &value)
	_, ok := value[field]
	return ok
}

func TestDecodePayloadAcceptsPartialAndRejectsUnknown(t *testing.T) {
	valid := []byte(`{"temperature":26,"soilMoisture":30,"light":900,"temperatureWarning":false,"soilMoistureWarning":false,"lightWarning":true}`)
	payload, err := DecodePayload(valid)
	if err != nil || *payload.Light != 900 || !*payload.LightWarning {
		t.Fatalf("DecodePayload(valid) = (%+v, %v)", payload, err)
	}
	// 单参数设备：只带一个指标 + 对应 warning
	single, err := DecodePayload([]byte(`{"soilMoisture":45,"soilMoistureWarning":false}`))
	if err != nil || single.SoilMoisture == nil || *single.SoilMoisture != 45 || single.Temperature != nil {
		t.Fatalf("DecodePayload(single) = (%+v, %v)", single, err)
	}
	// 执行器心跳：全空 payload 合法（Ingest 按心跳处理）
	empty, err := DecodePayload([]byte(`{}`))
	if err != nil || empty.Temperature != nil || empty.SoilMoisture != nil || empty.Light != nil {
		t.Fatalf("DecodePayload(empty heartbeat) = (%+v, %v)", empty, err)
	}
	// 未知字段拒绝
	if _, err := DecodePayload([]byte(`{"temperature":26,"soilMoisture":30,"light":900,"temperatureWarning":false,"soilMoistureWarning":false,"lightWarning":false,"deviceId":31}`)); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("unknown field error = %v, want ErrInvalidInput", err)
	}
}

func float64Ptr(value float64) *float64 { return &value }
func boolPtr(value bool) *bool          { return &value }
