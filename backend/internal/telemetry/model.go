package telemetry

import "time"

// MetricValue 表示某个指标的最新数值与单位。
type MetricValue struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// Latest 是某个地块的最新遥测快照；指标尚未接入时为 nil。
type Latest struct {
	PlotID       uint64
	SampleTime   time.Time
	SoilMoisture *MetricValue
	Temperature  *MetricValue
}
