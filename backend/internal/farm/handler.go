package farm

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func GetDashboardOverview(c *gin.Context) {
	farmId := c.Query("farmId")
	if farmId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "farmId is required",
		})
		return
	}

	respData := DashboardOverview{
		FarmId:     farmId,
		FarmName:   "张家湾温室群",
		SampleTime: "2026-08-22T08:21:00Z",
		AvgSoilMoisture: MetricValue{
			Value:         28.9,
			Unit:          "%",
			ThresholdDiff: -1.1,
			DayDiff:       -1.2,
		},
		AvgTemperature: MetricValue{
			Value:   26.3,
			Unit:    "°C",
			Status:  "STABLE",
			DayDiff: 0.4,
		},
		DeviceOnline: DeviceOnlineStat{
			Online:  12,
			Total:   13,
			Offline: 1,
		},
		Alerts: AlertStat{
			Active:         2,
			PendingConfirm: 1,
		},
		Plots: []DashboardPlotItem{
			{
				Id:           "plot_a3",
				Code:         "A3",
				SoilMoisture: 27.8,
				Temperature:  26.8,
				Status:       "ALERT",
			},
		},
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "OK",
		"data":    respData,
	})
}

func GetPlotLatestTelemetry(c *gin.Context) {
	plotId := c.Param("plotId")

	respData := PlotLatestTelemetry{
		PlotId:     plotId,
		SampleTime: parseTime("2026-08-22T08:21:00Z"),
		Metrics: map[string]MetricValue{
			"soilMoisture": {Value: 27.8, Unit: "%"},
			"temperature":  {Value: 26.8, Unit: "℃"},
		},
		SourceDevices: []SourceDevice{
			{
				ID:      "dev_soil_03",
				Name:    "土壤传感器 03",
				Status:  "ONLINE",
				Battery: 87,
			},
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "OK",
		"data":    respData,
	})
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}
