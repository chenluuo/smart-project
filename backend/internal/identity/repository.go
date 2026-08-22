package identity

import (
	"context"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	"gorm.io/gorm"
)

type Repositories struct {
	Users *persistence.Repository[User]
	Roles *persistence.Repository[Role]
	db    *gorm.DB
}

func NewRepositories(db *gorm.DB) Repositories {
	return Repositories{Users: persistence.NewRepository[User](db), Roles: persistence.NewRepository[Role](db), db: db}
}

func (r Repositories) FindUserByMobile(ctx context.Context, mobile string) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).Where("mobile = ?", mobile).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r Repositories) FindRoleByCode(ctx context.Context, code string) (*Role, error) {
	var role Role
	if err := r.db.WithContext(ctx).Where("role_code = ?", code).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}
