package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/alert"
	"github.com/chenluuo/smart-project/backend/internal/plot"
	"github.com/gin-gonic/gin"
)

// adminPlotListService 管理后台全量地块查询（复用 adminPlots 的 AdminList）。
type adminPlotListService interface {
	AdminList(context.Context, plot.AdminListFilter) ([]plot.AdminPlotItem, int64, error)
}

// adminTelemetryAlerts 管理后台活动告警查询（全部用户，复用 alert.Service.AdminList）。
type adminTelemetryAlerts interface {
	AdminList(context.Context, alert.ListFilter) (alert.ListResult, error)
}

type adminTelemetryHandler struct {
	plots     adminPlotListService
	telemetry telemetryService
	alerts    adminTelemetryAlerts
}

type adminPlotLatestItem struct {
	PlotID        uint64    `json:"plotId"`
	PlotCode      string    `json:"plotCode"`
	PlotName      string    `json:"plotName"`
	OwnerID       uint64    `json:"ownerId"`
	OwnerName     *string   `json:"ownerName"`
	Status        string    `json:"status"`
	SampleTime    *time.Time `json:"sampleTime,omitempty"`
	SoilMoisture  *float64  `json:"soilMoisture,omitempty"`
	Temperature   *float64  `json:"temperature,omitempty"`
	Light         *float64  `json:"light,omitempty"`
}

// registerAdminTelemetryRoutes 管理后台遥测（SYSTEM_ADMIN）：
// GET /api/v1/admin/telemetry/latest —— 全部地块最新温湿度/光照 + 告警状态。
func registerAdminTelemetryRoutes(
	router *gin.Engine,
	auth authService,
	plots adminPlotListService,
	telemetry telemetryService,
	alerts adminTelemetryAlerts,
) {
	handler := adminTelemetryHandler{plots: plots, telemetry: telemetry, alerts: alerts}
	admin := router.Group("/api/v1/admin", jwtAuthentication(auth), requireSystemAdmin())
	admin.GET("/telemetry/latest", handler.latest)
}

func (h adminTelemetryHandler) latest(c *gin.Context) {
	filter := plot.AdminListFilter{Page: 1, PageSize: 100}
	allPlots, _, err := h.plots.AdminList(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}

	// 可选 plotId 过滤（查询后过滤，不扩展 AdminListFilter 以免影响其他调用方）
	if raw := c.Query("plotId"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：plotId 必须为正整数")
			return
		}
		filtered := make([]plot.AdminPlotItem, 0, len(allPlots))
		for _, p := range allPlots {
			if p.Plot.ID == id {
				filtered = append(filtered, p)
			}
		}
		allPlots = filtered
	}

	plotIDs := make([]uint64, 0, len(allPlots))
	for _, p := range allPlots {
		plotIDs = append(plotIDs, p.Plot.ID)
	}
	latestList, err := h.telemetry.LatestByPlots(c.Request.Context(), plotIDs)
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	latestByPlot := make(map[uint64]map[string]*float64, len(latestList))
	sampleTimeByPlot := make(map[uint64]time.Time, len(latestList))
	for _, l := range latestList {
		entry := make(map[string]*float64, 3)
		if l.SoilMoisture != nil {
			value := l.SoilMoisture.Value
			entry["soilMoisture"] = &value
		}
		if l.Temperature != nil {
			value := l.Temperature.Value
			entry["temperature"] = &value
		}
		if l.Light != nil {
			value := l.Light.Value
			entry["light"] = &value
		}
		latestByPlot[l.PlotID] = entry
		if !l.SampleTime.IsZero() {
			sampleTimeByPlot[l.PlotID] = l.SampleTime
		}
	}

	// 活动告警地块集合（全部用户），派生 NORMAL/ALERT
	activeStatus := alert.StatusActive
	activeAlerts, err := h.alerts.AdminList(c.Request.Context(), alert.ListFilter{Status: &activeStatus, PageSize: 100})
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	alertPlots := make(map[uint64]bool)
	for _, item := range activeAlerts.Items {
		alertPlots[item.PlotID] = true
	}

	items := make([]adminPlotLatestItem, 0, len(allPlots))
	for _, p := range allPlots {
		item := adminPlotLatestItem{
			PlotID: p.Plot.ID, PlotCode: p.Plot.Code, PlotName: p.Plot.Name,
			OwnerID: p.Plot.OwnerID, OwnerName: p.OwnerName, Status: "NORMAL",
		}
		if alertPlots[p.Plot.ID] {
			item.Status = "ALERT"
		}
		if entry, ok := latestByPlot[p.Plot.ID]; ok {
			if t, hasTime := sampleTimeByPlot[p.Plot.ID]; hasTime {
				value := t
				item.SampleTime = &value
			}
			item.SoilMoisture = entry["soilMoisture"]
			item.Temperature = entry["temperature"]
			item.Light = entry["light"]
		}
		items = append(items, item)
	}

	respondSuccess(c, http.StatusOK, items)
}
