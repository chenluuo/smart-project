package farm

import (
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
