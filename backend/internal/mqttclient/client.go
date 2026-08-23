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
	topic          string
	prefix         string
	handler        *Handler
	messageTimeout time.Duration
}

func New(cfg config.MQTTConfig, handler *Handler) *Client {
	topic := cfg.TopicPrefix + "/+/+/telemetry"
	result := &Client{topic: topic, prefix: strings.Trim(cfg.TopicPrefix, "/"), handler: handler, messageTimeout: cfg.MessageTimeout}
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
		slog.Info("MQTT broker connected", "topic", topic)
		token := client.Subscribe(topic, 1, result.onMessage)
		if !token.WaitTimeout(cfg.ConnectTimeout) {
			slog.Error("MQTT subscription timed out", "topic", topic)
			return
		}
		if err := token.Error(); err != nil {
			slog.Error("subscribe to MQTT telemetry", "topic", topic, "error", err)
		}
	})
	result.client = paho.NewClient(options)
	return result
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

// PublishCommand 向设备下发命令: {prefix}/{ownerID}/{deviceSN}/command (QoS 1)。
// 非阻塞: 仅检查消息是否成功进入发送队列, broker 不可用时由 paho 自动重连补发。
func (c *Client) PublishCommand(ownerID uint64, deviceSN string, payload []byte) error {
	if ownerID == 0 || strings.TrimSpace(deviceSN) == "" {
		return errors.New("invalid command route")
	}
	topic := fmt.Sprintf("%s/%d/%s/command", c.prefix, ownerID, deviceSN)
	token := c.client.Publish(topic, 1, false, payload)
	select {
	case <-token.Done():
		if err := token.Error(); err != nil {
			return fmt.Errorf("publish command %s: %w", topic, err)
		}
		return nil
	case <-time.After(c.messageTimeout):
		return fmt.Errorf("publish command %s: timed out", topic)
	}
}

func (c *Client) onMessage(_ paho.Client, message paho.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), c.messageTimeout)
	defer cancel()
	if err := c.handler.Handle(ctx, message.Topic(), message.Payload()); err != nil {
		slog.Warn("reject MQTT telemetry", "topic", message.Topic(), "messageId", message.MessageID(), "error", err)
	}
}
