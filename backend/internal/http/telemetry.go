package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/device"
	"github.com/chenluuo/smart-project/backend/internal/plot"
	"github.com/chenluuo/smart-project/backend/internal/telemetry"
	"github.com/gin-gonic/gin"
)

type telemetryHandler struct {
	plots     plotService
	devices   deviceService
	telemetry telemetryService
}

type telemetryMetric struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type plotMetrics struct {
	SoilMoisture *telemetryMetric `json:"soilMoisture,omitempty"`
	Temperature  *telemetryMetric `json:"temperature,omitempty"`
}

type sourceDevice struct {
	ID      uint64        `json:"id"`
	Name    string        `json:"name"`
	Status  device.Status `json:"status"`
	Battery *int          `json:"battery"`
}

type plotTelemetry struct {
	PlotID        uint64         `json:"plotId"`
	SampleTime    *time.Time     `json:"sampleTime"`
	Metrics       plotMetrics    `json:"metrics"`
	SourceDevices []sourceDevice `json:"sourceDevices"`
}

func registerTelemetryRoutes(router *gin.Engine, auth authService, plots plotService, devices deviceService, svc telemetryService) {
	handler := telemetryHandler{plots: plots, devices: devices, telemetry: svc}
	router.GET("/api/v1/plots/:plotId/telemetry/latest", jwtAuthentication(auth), handler.latest)
}

func (h telemetryHandler) latest(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, 40101, "未登录或访问令牌无效")
		return
	}
	plotID, err := strconv.ParseUint(c.Param("plotId"), 10, 64)
	if err != nil || plotID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：plotId 必须为正整数")
		return
	}
	ctx := c.Request.Context()

	if _, err := h.plots.Get(ctx, claims.UserID, plotID); err != nil {
		if errors.Is(err, plot.ErrNotFound) {
			respondError(c, http.StatusNotFound, 40401, "地块不存在")
		} else {
			respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		}
		return
	}

	result := plotTelemetry{PlotID: plotID, Metrics: plotMetrics{}, SourceDevices: []sourceDevice{}}

	latest, err := h.telemetry.LatestByPlot(ctx, plotID)
	switch {
	case err == nil && latest != nil:
		if !latest.SampleTime.IsZero() {
			t := latest.SampleTime
			result.SampleTime = &t
		}
		if latest.SoilMoisture != nil {
			result.Metrics.SoilMoisture = &telemetryMetric{Value: latest.SoilMoisture.Value, Unit: latest.SoilMoisture.Unit}
		}
		if latest.Temperature != nil {
			result.Metrics.Temperature = &telemetryMetric{Value: latest.Temperature.Value, Unit: latest.Temperature.Unit}
		}
	case errors.Is(err, telemetry.ErrNotFound):
		// 遥测尚未接入，指标保持为空
	default:
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}

	plotIDFilter := plotID
	boundDevices, err := h.devices.List(ctx, claims.UserID, device.ListFilter{PlotID: &plotIDFilter})
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	for _, item := range boundDevices.Items {
		result.SourceDevices = append(result.SourceDevices, sourceDevice{
			ID: item.Device.ID, Name: item.Device.Name, Status: item.Device.Status, Battery: item.Device.Battery,
		})
	}

	respondSuccess(c, http.StatusOK, result)
}
