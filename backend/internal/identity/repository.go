package identity

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/shared/persistence"
	drivermysql "github.com/go-sql-driver/mysql"
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
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r Repositories) FindUserByAccountName(ctx context.Context, accountName string) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).Where("name = ?", accountName).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r Repositories) FindUserByID(ctx context.Context, userID uint64) (*User, error) {
	var user User
	if err := r.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &user, nil
}

func (r Repositories) CreateUser(ctx context.Context, user *User) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		var role Role
		if err := tx.Where("role_code = ?", "FARMER").First(&role).Error; err != nil {
			return err
		}
		return tx.Create(&UserRole{UserID: user.ID, RoleID: role.ID}).Error
	})
	if err != nil {
		var mysqlErr *drivermysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return ErrUserConflict
		}
		return err
	}
	return nil
}

func (r Repositories) FindRoleCodesByUserID(ctx context.Context, userID uint64) ([]string, error) {
	var codes []string
	err := r.db.WithContext(ctx).
		Table("roles").
		Select("roles.role_code").
		Joins("JOIN user_roles ON user_roles.role_id = roles.id").
		Where("user_roles.user_id = ?", userID).
		Order("roles.id").
		Scan(&codes).Error
	return codes, err
}

func (r Repositories) FindRoleByCode(ctx context.Context, code string) (*Role, error) {
	var role Role
	if err := r.db.WithContext(ctx).Where("role_code = ?", code).First(&role).Error; err != nil {
		return nil, err
	}
	return &role, nil
}

// AdminUserFilter 管理后台用户列表过滤条件。
type AdminUserFilter struct {
	Keyword  string
	Role     string
	Status   UserStatus
	Page     int
	PageSize int
}

// AdminUserView 管理后台用户列表项。
type AdminUserView struct {
	ID        uint64     `json:"id"`
	Username  string     `json:"username"`
	Mobile    string     `json:"mobile"`
	Role      string     `json:"role"`
	Status    UserStatus `json:"status"`
	PlotCount int64      `json:"plotCount"`
	CreatedAt time.Time  `json:"createdAt"`
}

// ListUsers 管理后台全量用户列表（含主角色与名下地块数）。
// 与现有按 JWT 视角的 /users/me 不同，这里不做归属过滤，仅由 admin 路由使用。
func (r Repositories) ListUsers(ctx context.Context, filter AdminUserFilter) ([]AdminUserView, int64, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	keyword := strings.TrimSpace(filter.Keyword)
	role := strings.TrimSpace(filter.Role)

	countQuery := r.db.WithContext(ctx).Model(&User{})
	if keyword != "" {
		like := "%" + keyword + "%"
		countQuery = countQuery.Where("(name LIKE ? OR mobile LIKE ?)", like, like)
	}
	if role != "" {
		countQuery = countQuery.Where("EXISTS (SELECT 1 FROM user_roles ur JOIN roles r2 ON r2.id = ur.role_id WHERE ur.user_id = users.id AND r2.role_code = ?)", role)
	}
	if filter.Status != "" {
		countQuery = countQuery.Where("status = ?", filter.Status)
	}
	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	selectSQL := "u.id AS id, u.name AS username, u.mobile AS mobile, u.status AS status, u.created_at AS created_at, " +
		"(SELECT r2.role_code FROM user_roles ur JOIN roles r2 ON r2.id = ur.role_id WHERE ur.user_id = u.id ORDER BY CASE WHEN r2.role_code = 'SYSTEM_ADMIN' THEN 0 ELSE 1 END, r2.id LIMIT 1) AS role, " +
		"(SELECT COUNT(*) FROM plots p WHERE p.owner_id = u.id) AS plot_count"
	listQuery := r.db.WithContext(ctx).Table("users u").Select(selectSQL)
	if keyword != "" {
		like := "%" + keyword + "%"
		listQuery = listQuery.Where("(u.name LIKE ? OR u.mobile LIKE ?)", like, like)
	}
	if role != "" {
		listQuery = listQuery.Where("EXISTS (SELECT 1 FROM user_roles ur JOIN roles r2 ON r2.id = ur.role_id WHERE ur.user_id = u.id AND r2.role_code = ?)", role)
	}
	if filter.Status != "" {
		listQuery = listQuery.Where("u.status = ?", filter.Status)
	}
	items := make([]AdminUserView, 0, filter.PageSize)
	if err := listQuery.Order("u.id ASC").Limit(filter.PageSize).Offset((filter.Page - 1) * filter.PageSize).Scan(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}
