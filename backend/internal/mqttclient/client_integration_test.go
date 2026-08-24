package mqttclient

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/config"
	"github.com/chenluuo/smart-project/backend/internal/telemetry"
	paho "github.com/eclipse/paho.mqtt.golang"
)

type capturedMQTTMessage struct {
	topic   string
	payload string
}

type ackCaptureHandler struct{ messages chan capturedMQTTMessage }

func (h *ackCaptureHandler) Handle(_ context.Context, topic string, raw []byte) error {
	h.messages <- capturedMQTTMessage{topic: topic, payload: string(raw)}
	return nil
}

type telemetryCaptureIngestor struct{ payloads chan telemetry.Payload }

func (i *telemetryCaptureIngestor) Ingest(_ context.Context, _ telemetry.TrustedSource, payload telemetry.Payload) (*telemetry.Latest, error) {
	i.payloads <- payload
	return &telemetry.Latest{}, nil
}

func TestMQTTClientReceivesSimulatedMachineTelemetryAndThresholdAck(t *testing.T) {
	brokerURL := os.Getenv("TEST_MQTT_BROKER_URL")
	if brokerURL == "" {
		t.Skip("set TEST_MQTT_BROKER_URL to run MQTT machine simulation")
	}
	suffix := time.Now().UnixNano()
	ackHandler := &ackCaptureHandler{messages: make(chan capturedMQTTMessage, 1)}
	telemetryIngestor := &telemetryCaptureIngestor{payloads: make(chan telemetry.Payload, 1)}
	serverClient := New(config.MQTTConfig{
		BrokerURL: brokerURL, ClientID: fmt.Sprintf("threshold-server-test-%d", suffix), TopicPrefix: "agri",
		ConnectTimeout: 2 * time.Second, ReconnectBackoff: 100 * time.Millisecond, MessageTimeout: 2 * time.Second,
	}, NewHandler("agri", &resolverStub{source: telemetry.TrustedSource{OwnerID: 7, PlotID: 11, PlotCode: "P-11", DeviceID: 3}}, telemetryIngestor), ackHandler)
	serverClient.Start()
	t.Cleanup(serverClient.Close)
	deadline := time.Now().Add(5 * time.Second)
	for !serverClient.client.IsConnected() && time.Now().Before(deadline) {
		time.Sleep(25 * time.Millisecond)
	}
	if !serverClient.client.IsConnected() {
		t.Fatal("server MQTT client did not connect")
	}
	// OnConnect subscribes synchronously; a short grace period avoids racing the broker subscription acknowledgement.
	time.Sleep(100 * time.Millisecond)

	machineOptions := paho.NewClientOptions().AddBroker(brokerURL).
		SetClientID(fmt.Sprintf("threshold-machine-test-%d", suffix)).SetConnectTimeout(2 * time.Second)
	machine := paho.NewClient(machineOptions)
	connectToken := machine.Connect()
	if !connectToken.WaitTimeout(2*time.Second) || connectToken.Error() != nil {
		t.Fatalf("connect simulated machine: %v", connectToken.Error())
	}
	t.Cleanup(func() { machine.Disconnect(100) })

	telemetryTopic := "agri/7/BEARPI-SIM-001/telemetry"
	telemetryPayload := `{"temperature":31.5,"soilMoisture":24,"light":880,"temperatureWarning":false,"soilMoistureWarning":true,"lightWarning":false}`
	telemetryToken := machine.Publish(telemetryTopic, 1, false, telemetryPayload)
	if !telemetryToken.WaitTimeout(2*time.Second) || telemetryToken.Error() != nil {
		t.Fatalf("publish simulated machine telemetry: %v", telemetryToken.Error())
	}
	select {
	case payload := <-telemetryIngestor.payloads:
		if payload.SoilMoisture != 24 || !payload.SoilMoistureWarning || payload.TemperatureWarning {
			t.Fatalf("received simulated telemetry = %+v", payload)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not receive simulated machine telemetry")
	}

	topic := "agri/7/BEARPI-SIM-001/config/thresholds/ack"
	payload := `{"messageId":"thr_simulated","configVersion":9,"status":"APPLIED"}`
	publishToken := machine.Publish(topic, 1, false, payload)
	if !publishToken.WaitTimeout(2*time.Second) || publishToken.Error() != nil {
		t.Fatalf("publish simulated machine ACK: %v", publishToken.Error())
	}
	select {
	case message := <-ackHandler.messages:
		if message.topic != topic || message.payload != payload {
			t.Fatalf("received MQTT message = %+v", message)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not receive simulated machine ACK")
	}
}
