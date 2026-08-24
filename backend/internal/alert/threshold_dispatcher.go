package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/outbox"
)

type ThresholdPublisher interface {
	Publish(context.Context, string, []byte, byte, bool) error
}

type ThresholdDispatcher struct {
	events     EventStore
	deliveries thresholdDeliveryStore
	publisher  ThresholdPublisher
	prefix     string
	now        func() time.Time
}

func NewThresholdDispatcher(events EventStore, deliveries thresholdDeliveryStore, publisher ThresholdPublisher, prefix string) (*ThresholdDispatcher, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if events == nil || deliveries == nil || publisher == nil || prefix == "" || strings.ContainsAny(prefix, "+#") {
		return nil, errors.New("threshold dispatcher dependencies are required")
	}
	return &ThresholdDispatcher{events: events, deliveries: deliveries, publisher: publisher, prefix: prefix, now: time.Now}, nil
}

func (d *ThresholdDispatcher) DispatchOnce(ctx context.Context, batchSize int) (int, error) {
	if batchSize < 1 || batchSize > 500 {
		return 0, errors.New("threshold dispatcher batch size must be between 1 and 500")
	}
	now := d.now().UTC()
	events, err := d.events.ClaimAvailableByEventPrefix(ctx, now, "THRESHOLD_CONFIG_", batchSize, 30*time.Second)
	if err != nil {
		return 0, fmt.Errorf("claim threshold outbox events: %w", err)
	}
	published := 0
	var dispatchErrors []error
	for index := range events {
		event := &events[index]
		var envelope thresholdOutboxPayload
		if err := json.Unmarshal(event.Payload, &envelope); err != nil {
			dispatchErrors = append(dispatchErrors, d.markFailed(ctx, event, fmt.Errorf("decode payload: %w", err)))
			continue
		}
		if !validThresholdEnvelope(envelope) {
			dispatchErrors = append(dispatchErrors, d.markFailed(ctx, event, errors.New("invalid threshold delivery payload")))
			continue
		}
		if !now.Before(envelope.Config.ExpiresAt) {
			_, _ = d.deliveries.ExpireThresholdDeliveries(ctx, now)
			if err := d.events.MarkPublished(ctx, event.ID, now); err != nil {
				dispatchErrors = append(dispatchErrors, fmt.Errorf("close expired threshold event %d: %w", event.ID, err))
			}
			continue
		}
		payload, err := json.Marshal(envelope.Config)
		if err == nil {
			topic := fmt.Sprintf("%s/%d/%s/config/thresholds/v/%d", d.prefix, envelope.OwnerID, envelope.DeviceSN, envelope.Config.ConfigVersion)
			err = d.publisher.Publish(ctx, topic, payload, 1, true)
		}
		if err != nil {
			_ = d.deliveries.RecordThresholdPublishFailure(ctx, envelope.Config.MessageID, err.Error(), now)
			dispatchErrors = append(dispatchErrors, d.markFailed(ctx, event, err))
			continue
		}
		if err := d.deliveries.MarkThresholdSent(ctx, envelope.Config.MessageID, now); err != nil {
			dispatchErrors = append(dispatchErrors, fmt.Errorf("mark threshold delivery %s sent: %w", envelope.Config.MessageID, err))
			continue
		}
		if err := d.events.MarkPublished(ctx, event.ID, now); err != nil {
			dispatchErrors = append(dispatchErrors, fmt.Errorf("mark threshold event %d published: %w", event.ID, err))
			continue
		}
		published++
	}
	return published, errors.Join(dispatchErrors...)
}

func (d *ThresholdDispatcher) Run(ctx context.Context, interval time.Duration, batchSize int) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := d.DispatchOnce(ctx, batchSize); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("dispatch threshold outbox", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (d *ThresholdDispatcher) markFailed(ctx context.Context, event *outbox.Event, deliveryErr error) error {
	message := deliveryErr.Error()
	if len(message) > 1000 {
		message = message[:1000]
	}
	now := d.now().UTC()
	availableAt := now.Add(thresholdRetryDelay(event.RetryCount + 1))
	if err := d.events.MarkFailed(ctx, event.ID, message, availableAt, now); err != nil {
		return fmt.Errorf("deliver threshold event %d: %v; mark failed: %w", event.ID, deliveryErr, err)
	}
	return fmt.Errorf("deliver threshold event %d: %w", event.ID, deliveryErr)
}

func validThresholdEnvelope(value thresholdOutboxPayload) bool {
	return value.OwnerID > 0 && validMQTTTopicSegment(value.DeviceSN) && value.Config.MessageID != "" &&
		value.Config.PlotID > 0 && value.Config.ConfigVersion > 0 && !value.Config.IssuedAt.IsZero() &&
		!value.Config.ExpiresAt.IsZero() && len(value.Config.Rules) > 0
}

func validMQTTTopicSegment(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && !strings.ContainsAny(value, "/+#")
}

func thresholdRetryDelay(retryCount int) time.Duration {
	if retryCount < 1 {
		retryCount = 1
	}
	if retryCount > 8 {
		retryCount = 8
	}
	return time.Duration(1<<(retryCount-1)) * time.Second
}

type ThresholdExpiryWorker struct {
	store thresholdDeliveryStore
	now   func() time.Time
}

func NewThresholdExpiryWorker(store thresholdDeliveryStore) *ThresholdExpiryWorker {
	return &ThresholdExpiryWorker{store: store, now: time.Now}
}

func (w *ThresholdExpiryWorker) Run(ctx context.Context, interval time.Duration) {
	if w == nil || w.store == nil {
		return
	}
	if interval <= 0 {
		interval = 2 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if count, err := w.store.ExpireThresholdDeliveries(ctx, w.now().UTC()); err != nil && !errors.Is(err, context.Canceled) {
			slog.Warn("expire threshold deliveries", "error", err)
		} else if count > 0 {
			slog.Info("threshold deliveries timed out", "count", strconv.FormatInt(count, 10))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
