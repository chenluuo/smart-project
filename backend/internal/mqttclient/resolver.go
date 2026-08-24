package mqttclient

import (
	"context"
	"strings"

	"github.com/chenluuo/smart-project/backend/internal/telemetry"
	"gorm.io/gorm"
)

type GormSourceResolver struct {
	db *gorm.DB
}

func NewGormSourceResolver(db *gorm.DB) *GormSourceResolver {
	return &GormSourceResolver{db: db}
}

// ResolveTelemetrySource 按设备序列号解析遥测归属（当前有效绑定 → 地块 → 归属用户）。
//
// topic 里的 ownerID 不再参与匹配：硬件侧 ownerId 是固件写死的，管理后台
// 转移地块归属/重绑设备后不会跟随变化；数据归属以数据库绑定关系为唯一真理源
// （device_bindings → plots.owner_id），确保数据/告警/推送自动路由到当前归属用户。
func (r *GormSourceResolver) ResolveTelemetrySource(ctx context.Context, _ uint64, deviceSN string) (telemetry.TrustedSource, error) {
	var source telemetry.TrustedSource
	err := r.db.WithContext(ctx).Table("devices AS d").
		Select("p.owner_id AS owner_id, p.id AS plot_id, p.code AS plot_code, d.id AS device_id").
		Joins("JOIN device_bindings AS b ON b.device_id = d.id AND b.unbound_at IS NULL").
		Joins("JOIN plots AS p ON p.id = b.plot_id").
		Where("d.serial_no = ?", strings.TrimSpace(deviceSN)).
		Take(&source).Error
	return source, err
}
