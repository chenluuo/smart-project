package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/device"
	"github.com/chenluuo/smart-project/backend/internal/plot"
	"github.com/chenluuo/smart-project/backend/internal/telemetry"
)

type telemetryServiceStub struct {
	latest *telemetry.Latest
	err    error
	plotID uint64
}

func (s *telemetryServiceStub) LatestByPlot(_ context.Context, plotID uint64) (*telemetry.Latest, error) {
	s.plotID = plotID
	return s.latest, s.err
}

func newTelemetryTestRouter(plots plotService, devices deviceService, telemetry telemetryService) http.Handler {
	return NewRouterWithBackendServices("test", pingerStub{}, authServiceStub{}, plots, devices, nil, nil, nil, nil, telemetry, "service-key")
}

func TestPlotTelemetryRequiresAuthentication(t *testing.T) {
	router := newTelemetryTestRouter(&plotServiceStub{}, &deviceServiceStub{}, &telemetryServiceStub{})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/plots/11/telemetry/latest", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized || !strings.Contains(response.Body.String(), `"code":40101`) {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestPlotTelemetryValidatesIDAndOwnership(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		plots      plotService
		wantStatus int
		wantCode   string
	}{
		{name: "invalid plotId", path: "/api/v1/plots/not-a-number/telemetry/latest", plots: &plotServiceStub{}, wantStatus: http.StatusBadRequest, wantCode: `"code":40001`},
		{name: "foreign or missing plot", path: "/api/v1/plots/99/telemetry/latest", plots: &plotServiceStub{getErr: plot.ErrNotFound}, wantStatus: http.StatusNotFound, wantCode: `"code":40401`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := newTelemetryTestRouter(tt.plots, &deviceServiceStub{}, &telemetryServiceStub{})
			request := httptest.NewRequest(http.MethodGet, tt.path, nil)
			request.Header.Set("Authorization", "Bearer signed-token")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tt.wantStatus || !strings.Contains(response.Body.String(), tt.wantCode) {
				t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestPlotTelemetryReturnsLatestAndSourceDevices(t *testing.T) {
	soil := telemetry.MetricValue{Value: 27.8, Unit: "%"}
	temp := telemetry.MetricValue{Value: 26.8, Unit: "°C"}
	sampleTime := time.Date(2026, 8, 22, 8, 21, 0, 0, time.FixedZone("CST", 8*60*60))
	telemetryStub := &telemetryServiceStub{latest: &telemetry.Latest{
		PlotID: 11, SampleTime: sampleTime, SoilMoisture: &soil, Temperature: &temp,
	}}
	plotStub := &plotServiceStub{plot: &plot.Plot{ID: 11, OwnerID: 7, Code: "A3", Name: "东侧棚", Status: plot.StatusActive}}
	battery := 87
	deviceStub := &deviceServiceStub{listResult: device.ListResult{Items: []device.ListItem{{
		Device: device.Device{ID: 3, Name: "土壤传感器 03", Status: device.StatusOnline, Battery: &battery},
		PlotID: 11,
	}}}}

	router := newTelemetryTestRouter(plotStub, deviceStub, telemetryStub)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/plots/11/telemetry/latest", nil)
	request.Header.Set("Authorization", "Bearer signed-token")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if telemetryStub.plotID != 11 {
		t.Fatalf("LatestByPlot plotID = %d, want 11", telemetryStub.plotID)
	}
	for _, want := range []string{
		`"plotId":11`,
		`"soilMoisture":{"value":27.8,"unit":"%"}`,
		`"temperature":{"value":26.8,"unit":"°C"}`,
		`"name":"土壤传感器 03"`,
		`"status":"ONLINE"`,
		`"battery":87`,
	} {
		if !strings.Contains(response.Body.String(), want) {
			t.Fatalf("body = %s, want it to contain %s", response.Body.String(), want)
		}
	}
}
