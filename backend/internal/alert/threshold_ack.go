package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type thresholdAckRequest struct {
	MessageID     string              `json:"messageId"`
	ConfigVersion uint64              `json:"configVersion"`
	Status        ThresholdSyncStatus `json:"status"`
	Reason        string              `json:"reason,omitempty"`
}

type ThresholdAckHandler struct {
	prefix string
	store  thresholdDeliveryStore
	now    func() time.Time
}

func NewThresholdAckHandler(prefix string, store thresholdDeliveryStore) (*ThresholdAckHandler, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" || strings.ContainsAny(prefix, "+#") || store == nil {
		return nil, errors.New("threshold ack handler dependencies are required")
	}
	return &ThresholdAckHandler{prefix: prefix, store: store, now: time.Now}, nil
}

func (h *ThresholdAckHandler) Handle(ctx context.Context, topic string, raw []byte) error {
	ownerID, deviceSN, err := parseThresholdAckTopic(h.prefix, topic)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request thresholdAckRequest
	if err := decoder.Decode(&request); err != nil {
		return ErrInvalidInput
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrInvalidInput
	}
	request.MessageID = strings.TrimSpace(request.MessageID)
	request.Reason = strings.TrimSpace(request.Reason)
	if request.MessageID == "" || len(request.MessageID) > 64 || request.ConfigVersion == 0 ||
		(request.Status != ThresholdSyncApplied && request.Status != ThresholdSyncFailed) ||
		utf8.RuneCountInString(request.Reason) > 500 || request.Status == ThresholdSyncFailed && request.Reason == "" {
		return ErrInvalidInput
	}
	err = h.store.ApplyThresholdAck(ctx, ownerID, deviceSN, ThresholdAckInput{
		MessageID: request.MessageID, ConfigVersion: request.ConfigVersion,
		Status: request.Status, Reason: request.Reason,
	}, h.now().UTC())
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if err != nil {
		return err
	}
	return nil
}

func parseThresholdAckTopic(prefix, topic string) (uint64, string, error) {
	parts := strings.Split(topic, "/")
	if len(parts) != 6 || parts[0] != prefix || !validMQTTTopicSegment(parts[2]) ||
		parts[3] != "config" || parts[4] != "thresholds" || parts[5] != "ack" {
		return 0, "", ErrInvalidInput
	}
	ownerID, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || ownerID == 0 {
		return 0, "", ErrInvalidInput
	}
	return ownerID, parts[2], nil
}
