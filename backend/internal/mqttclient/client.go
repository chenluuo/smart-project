package mqttclient

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/config"
	paho "github.com/eclipse/paho.mqtt.golang"
)

type Client struct {
	client         paho.Client
	telemetryTopic string
	ackTopic       string
	handler        *Handler
	ackHandler     MessageHandler
	messageTimeout time.Duration
}

type MessageHandler interface {
	Handle(context.Context, string, []byte) error
}

func New(cfg config.MQTTConfig, handler *Handler, ackHandlers ...MessageHandler) *Client {
	telemetryTopic := cfg.TopicPrefix + "/+/+/telemetry"
	result := &Client{telemetryTopic: telemetryTopic, handler: handler, messageTimeout: cfg.MessageTimeout}
	if len(ackHandlers) > 0 {
		result.ackHandler = ackHandlers[0]
		result.ackTopic = cfg.TopicPrefix + "/+/+/config/thresholds/ack"
	}
	options := paho.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.ClientID).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetCleanSession(false).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(cfg.ReconnectBackoff).
		SetConnectTimeout(cfg.ConnectTimeout).
		SetKeepAlive(30 * time.Second).
		SetPingTimeout(10 * time.Second)
	options.SetConnectionLostHandler(func(_ paho.Client, err error) {
		slog.Warn("MQTT connection lost; reconnecting", "error", err)
	})
	options.SetOnConnectHandler(func(client paho.Client) {
		slog.Info("MQTT broker connected", "telemetryTopic", telemetryTopic, "thresholdAckTopic", result.ackTopic)
		result.subscribe(client, telemetryTopic, result.onTelemetry, cfg.ConnectTimeout)
		if result.ackHandler != nil {
			result.subscribe(client, result.ackTopic, result.onThresholdAck, cfg.ConnectTimeout)
		}
	})
	result.client = paho.NewClient(options)
	return result
}

func (c *Client) subscribe(client paho.Client, topic string, callback paho.MessageHandler, timeout time.Duration) {
	token := client.Subscribe(topic, 1, callback)
	if !token.WaitTimeout(timeout) {
		slog.Error("MQTT subscription timed out", "topic", topic)
		return
	}
	if err := token.Error(); err != nil {
		slog.Error("subscribe to MQTT topic", "topic", topic, "error", err)
	}
}

// Start begins connecting in the background. With ConnectRetry enabled, an
// unavailable broker or an offline device never prevents the HTTP API starting.
func (c *Client) Start() {
	token := c.client.Connect()
	go func() {
		token.Wait()
		if err := token.Error(); err != nil {
			slog.Warn("MQTT initial connection failed; retrying", "error", err)
		}
	}()
}

func (c *Client) Close() {
	c.client.Disconnect(250)
}

func (c *Client) onTelemetry(_ paho.Client, message paho.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), c.messageTimeout)
	defer cancel()
	if err := c.handler.Handle(ctx, message.Topic(), message.Payload()); err != nil {
		slog.Warn("reject MQTT telemetry", "topic", message.Topic(), "messageId", message.MessageID(), "error", err)
	}
}

func (c *Client) onThresholdAck(_ paho.Client, message paho.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), c.messageTimeout)
	defer cancel()
	if err := c.ackHandler.Handle(ctx, message.Topic(), message.Payload()); err != nil {
		slog.Warn("reject MQTT threshold acknowledgement", "topic", message.Topic(), "messageId", message.MessageID(), "error", err)
	}
}

func (c *Client) Publish(ctx context.Context, topic string, payload []byte, qos byte, retained bool) error {
	if c == nil || c.client == nil {
		return errors.New("MQTT client is not configured")
	}
	timeout := c.messageTimeout
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return context.DeadlineExceeded
		}
		if timeout <= 0 || remaining < timeout {
			timeout = remaining
		}
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	token := c.client.Publish(topic, qos, retained, payload)
	if !token.WaitTimeout(timeout) {
		return fmt.Errorf("publish MQTT message to %s timed out", topic)
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("publish MQTT message to %s: %w", topic, err)
	}
	return nil
}
