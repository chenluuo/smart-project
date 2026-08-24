package alert

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/outbox"
	"gorm.io/datatypes"
)

type thresholdDeliveryStoreStub struct {
	sentMessageID  string
	publishFailure string
	ackOwnerID     uint64
	ackDeviceSN    string
	ackInput       ThresholdAckInput
	ackAt          time.Time
	expireAt       time.Time
	expired        int64
	err            error
}

func (s *thresholdDeliveryStoreStub) MarkThresholdSent(_ context.Context, messageID string, _ time.Time) error {
	s.sentMessageID = messageID
	return s.err
}

func (s *thresholdDeliveryStoreStub) RecordThresholdPublishFailure(_ context.Context, _ string, message string, _ time.Time) error {
	s.publishFailure = message
	return s.err
}

func (s *thresholdDeliveryStoreStub) ApplyThresholdAck(_ context.Context, ownerID uint64, deviceSN string, input ThresholdAckInput, now time.Time) error {
	s.ackOwnerID, s.ackDeviceSN, s.ackInput, s.ackAt = ownerID, deviceSN, input, now
	return s.err
}

func (s *thresholdDeliveryStoreStub) ExpireThresholdDeliveries(_ context.Context, now time.Time) (int64, error) {
	s.expireAt = now
	return s.expired, s.err
}

type thresholdPublisherStub struct {
	topic    string
	payload  []byte
	qos      byte
	retained bool
	err      error
}

func (s *thresholdPublisherStub) Publish(_ context.Context, topic string, payload []byte, qos byte, retained bool) error {
	s.topic, s.payload, s.qos, s.retained = topic, append([]byte(nil), payload...), qos, retained
	return s.err
}

func TestThresholdDispatcherPublishesVersionedRetainedSnapshot(t *testing.T) {
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	config := ThresholdConfigMessage{
		MessageID: "thr_message_1", PlotID: 11, ConfigVersion: 7,
		Rules: []ThresholdConfigRule{{
			ID: 2, Metric: "soilMoisture", Operator: OperatorLT, Value: 28,
			Hysteresis: 2, DurationSeconds: 300, Level: LevelMedium, Enabled: true,
		}},
		IssuedAt: now, ExpiresAt: now.Add(2 * time.Minute),
	}
	raw, _ := json.Marshal(thresholdOutboxPayload{OwnerID: 7, DeviceSN: "BEARPI-001", Config: config})
	events := &alertEventStoreStub{events: []outbox.Event{{ID: 9, Payload: datatypes.JSON(raw)}}}
	deliveries := &thresholdDeliveryStoreStub{}
	publisher := &thresholdPublisherStub{}
	dispatcher, err := NewThresholdDispatcher(events, deliveries, publisher, "agri")
	if err != nil {
		t.Fatalf("NewThresholdDispatcher() error = %v", err)
	}
	dispatcher.now = func() time.Time { return now }

	published, err := dispatcher.DispatchOnce(context.Background(), 10)
	if err != nil || published != 1 {
		t.Fatalf("DispatchOnce() = (%d, %v)", published, err)
	}
	if events.lastPrefix != "THRESHOLD_CONFIG_" || len(events.publishedIDs) != 1 || deliveries.sentMessageID != config.MessageID {
		t.Fatalf("delivery state events=%+v deliveries=%+v", events, deliveries)
	}
	if publisher.topic != "agri/7/BEARPI-001/config/thresholds/v/7" || publisher.qos != 1 || !publisher.retained {
		t.Fatalf("publish route topic=%q qos=%d retained=%v", publisher.topic, publisher.qos, publisher.retained)
	}
	var delivered ThresholdConfigMessage
	if err := json.Unmarshal(publisher.payload, &delivered); err != nil || delivered.ConfigVersion != 7 || len(delivered.Rules) != 1 || delivered.Rules[0].Value != 28 {
		t.Fatalf("published payload = %s, error=%v", publisher.payload, err)
	}
}

func TestSimulatedMachineThresholdAcknowledgement(t *testing.T) {
	store := &thresholdDeliveryStoreStub{}
	handler, err := NewThresholdAckHandler("agri", store)
	if err != nil {
		t.Fatalf("NewThresholdAckHandler() error = %v", err)
	}
	now := time.Date(2026, 8, 24, 9, 0, 5, 0, time.UTC)
	handler.now = func() time.Time { return now }

	// 模拟机器持久化第 7 版阈值配置后，通过 MQTT 上报 ACK。
	raw := []byte(`{"messageId":"thr_message_1","configVersion":7,"status":"APPLIED"}`)
	err = handler.Handle(context.Background(), "agri/7/BEARPI-001/config/thresholds/ack", raw)
	if err != nil {
		t.Fatalf("Handle(machine ACK) error = %v", err)
	}
	if store.ackOwnerID != 7 || store.ackDeviceSN != "BEARPI-001" || store.ackInput.MessageID != "thr_message_1" ||
		store.ackInput.ConfigVersion != 7 || store.ackInput.Status != ThresholdSyncApplied || !store.ackAt.Equal(now) {
		t.Fatalf("routed ACK = owner:%d device:%s input:%+v at:%v", store.ackOwnerID, store.ackDeviceSN, store.ackInput, store.ackAt)
	}
}

func TestSimulatedMachineFailedAckRequiresReasonAndRejectsWrongTopic(t *testing.T) {
	store := &thresholdDeliveryStoreStub{}
	handler, _ := NewThresholdAckHandler("agri", store)
	tests := []struct {
		name  string
		topic string
		raw   string
	}{
		{name: "failed without reason", topic: "agri/7/BEARPI-001/config/thresholds/ack", raw: `{"messageId":"thr_1","configVersion":7,"status":"FAILED"}`},
		{name: "wrong device topic", topic: "agri/7/BEARPI-001/telemetry", raw: `{"messageId":"thr_1","configVersion":7,"status":"APPLIED"}`},
		{name: "unknown field", topic: "agri/7/BEARPI-001/config/thresholds/ack", raw: `{"messageId":"thr_1","configVersion":7,"status":"APPLIED","ownerId":8}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handler.Handle(context.Background(), tt.topic, []byte(tt.raw)); err == nil {
				t.Fatal("Handle() error = nil")
			}
		})
	}
	if store.ackInput.MessageID != "" {
		t.Fatalf("invalid ACK reached store: %+v", store.ackInput)
	}
}

func TestThresholdDispatcherRetriesPublishFailure(t *testing.T) {
	now := time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC)
	config := ThresholdConfigMessage{
		MessageID: "thr_message_2", PlotID: 11, ConfigVersion: 8,
		Rules:    []ThresholdConfigRule{{ID: 2, Metric: "soilMoisture", Operator: OperatorLT, Value: 26, Level: LevelHigh, Enabled: true}},
		IssuedAt: now, ExpiresAt: now.Add(time.Minute),
	}
	raw, _ := json.Marshal(thresholdOutboxPayload{OwnerID: 7, DeviceSN: "BEARPI-001", Config: config})
	events := &alertEventStoreStub{events: []outbox.Event{{ID: 10, RetryCount: 1, Payload: datatypes.JSON(raw)}}}
	deliveries := &thresholdDeliveryStoreStub{}
	publisher := &thresholdPublisherStub{err: errors.New("broker unavailable")}
	dispatcher, _ := NewThresholdDispatcher(events, deliveries, publisher, "agri")
	dispatcher.now = func() time.Time { return now }

	published, err := dispatcher.DispatchOnce(context.Background(), 10)
	if published != 0 || err == nil || len(events.failedIDs) != 1 || deliveries.publishFailure != "broker unavailable" {
		t.Fatalf("DispatchOnce() = (%d, %v), events=%+v deliveries=%+v", published, err, events, deliveries)
	}
}
