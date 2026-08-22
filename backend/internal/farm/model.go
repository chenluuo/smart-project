package farm

import (
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"github.com/shopspring/decimal"
)

type Status string
type Role string

const (
	StatusActive   Status = "ACTIVE"
	StatusDisabled Status = "DISABLED"
	RoleFarmer     Role   = "FARMER"
	RoleAdmin      Role   = "FARM_ADMIN"
)

type Farm struct {
	ID      uint64  `json:"id" gorm:"primaryKey;autoIncrement"`
	OwnerID uint64  `json:"ownerId" gorm:"column:owner_id;not null;index:idx_farms_owner"`
	Name    string  `json:"name" gorm:"size:128;not null"`
	Address *string `json:"address,omitempty" gorm:"size:255"`
	Status  Status  `json:"status" gorm:"size:32;not null"`
	persistence.Auditable
}

func (Farm) TableName() string { return "farms" }

type Member struct {
	ID       uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	FarmID   uint64 `json:"farmId" gorm:"column:farm_id;not null;uniqueIndex:uk_farm_user"`
	UserID   uint64 `json:"userId" gorm:"column:user_id;not null;uniqueIndex:uk_farm_user;index:idx_farm_users_user"`
	FarmRole Role   `json:"farmRole" gorm:"column:farm_role;size:32;not null"`
	persistence.Auditable
}

func (Member) TableName() string { return "farm_users" }

type Plot struct {
	ID          uint64           `json:"id" gorm:"primaryKey;autoIncrement"`
	FarmID      uint64           `json:"farmId" gorm:"column:farm_id;not null;index:idx_plots_farm"`
	Name        string           `json:"name" gorm:"size:128;not null"`
	CropType    *string          `json:"cropType,omitempty" gorm:"column:crop_type;size:64"`
	GrowthStage *string          `json:"growthStage,omitempty" gorm:"column:growth_stage;size:64"`
	Area        *decimal.Decimal `json:"area,omitempty" gorm:"type:decimal(12,2)"`
	Location    *string          `json:"location,omitempty" gorm:"size:255"`
	Status      Status           `json:"status" gorm:"size:32;not null"`
	persistence.Auditable
}

func (Plot) TableName() string { return "plots" }

// 指标数值对象
type MetricValue struct {
	Value         float64 `json:"value"`
	Unit          string  `json:"unit"`
	ThresholdDiff float64 `json:"thresholdDiff,omitempty"`
	DayDiff       float64 `json:"dayDiff,omitempty"`
	Status        string  `json:"status,omitempty"`
}

type DeviceOnlineStat struct {
	Online  int `json:"online"`
	Total   int `json:"total"`
	Offline int `json:"offline"`
}

type AlertStat struct {
	Active         int `json:"active"`
	PendingConfirm int `json:"pendingConfirm"`
}

type DashboardPlotItem struct {
	Id           string  `json:"id"`
	Code         string  `json:"code"`
	SoilMoisture float64 `json:"soilMoisture"`
	Temperature  float64 `json:"temperature"`
	Status       string  `json:"status"`
}

type DashboardOverview struct {
	FarmId          string              `json:"farmId"`
	FarmName        string              `json:"farmName"`
	SampleTime      string              `json:"sampleTime"`
	AvgSoilMoisture MetricValue         `json:"avgSoilMoisture"`
	AvgTemperature  MetricValue         `json:"avgTemperature"`
	DeviceOnline    DeviceOnlineStat    `json:"deviceOnline"`
	Alerts          AlertStat           `json:"alerts"`
	Plots           []DashboardPlotItem `json:"plots"`
}
type SourceDevice struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Battery int    `json:"battery"`
}

type PlotLatestTelemetry struct {
	PlotId        string                 `json:"plotId"`
	SampleTime    time.Time              `json:"sampleTime"`
	Metrics       map[string]MetricValue `json:"metrics"`
	SourceDevices []SourceDevice         `json:"sourceDevices"`
}
