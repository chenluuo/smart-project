package identity

import "github.com/chenluuo/smart-project/backend/internal/shared/persistence"

type UserStatus string

const (
	UserStatusActive   UserStatus = "ACTIVE"
	UserStatusDisabled UserStatus = "DISABLED"
)

type User struct {
	ID           uint64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string     `json:"name" gorm:"size:64;not null"`
	Mobile       string     `json:"mobile" gorm:"size:32;not null;uniqueIndex:uk_users_mobile"`
	PasswordHash string     `json:"-" gorm:"column:password_hash;size:255;not null"`
	Status       UserStatus `json:"status" gorm:"size:32;not null"`
	persistence.Auditable
}

func (User) TableName() string { return "users" }

type Role struct {
	ID       uint64 `json:"id" gorm:"primaryKey;autoIncrement"`
	RoleCode string `json:"roleCode" gorm:"column:role_code;size:64;not null;uniqueIndex:uk_roles_code"`
	RoleName string `json:"roleName" gorm:"column:role_name;size:64;not null"`
	persistence.Auditable
}

func (Role) TableName() string { return "roles" }

type UserRole struct {
	UserID uint64 `json:"userId" gorm:"column:user_id;primaryKey"`
	RoleID uint64 `json:"roleId" gorm:"column:role_id;primaryKey"`
}

func (UserRole) TableName() string { return "user_roles" }
