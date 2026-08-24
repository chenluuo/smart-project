package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/chenluuo/smart-project/backend/internal/device"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type adminDeviceService interface {
	AdminList(context.Context, device.ListFilter) ([]device.AdminDeviceItem, int64, error)
	AdminBind(context.Context, uint64, device.BindInput) (*device.Device, error)
	AdminUnbind(context.Context, uint64) error
	AdminDelete(context.Context, uint64) error
}

type adminDeviceHandler struct {
	service adminDeviceService
}

type adminDeviceItemView struct {
	ID              uint64        `json:"id"`
	DeviceSN        string        `json:"deviceSn"`
	Name            string        `json:"name"`
	Type            string        `json:"type"`
	Status          device.Status `json:"status"`
	PlotID          uint64        `json:"plotId"`
	PlotCode        *string       `json:"plotCode"`
	PlotName        *string       `json:"plotName"`
	OwnerName       *string       `json:"ownerName"`
	FirmwareVersion *string       `json:"firmwareVersion"`
	LastSeenAt      *string       `json:"lastSeenAt"`
}

func registerAdminDeviceRoutes(router *gin.Engine, auth authService, service adminDeviceService) {
	handler := adminDeviceHandler{service: service}
	admin := router.Group("/api/v1/admin", jwtAuthentication(auth), requireSystemAdmin())
	admin.GET("/devices", handler.list)
	admin.POST("/devices/bind", handler.bind)
	admin.DELETE("/devices/:deviceId/binding", handler.unbind)
	admin.DELETE("/devices/:deviceId", handler.delete)
}

func (h adminDeviceHandler) list(c *gin.Context) {
	filter, err := parseAdminDeviceListFilter(c)
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, err.Error())
		return
	}
	items, total, err := h.service.AdminList(c.Request.Context(), filter)
	if errors.Is(err, device.ErrInvalidInput) {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：分页、plotId 或 status 不合法")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	views := make([]adminDeviceItemView, 0, len(items))
	for _, item := range items {
		var lastSeen *string
		if item.Device.LastSeenAt != nil {
			value := item.Device.LastSeenAt.Format("2006-01-02T15:04:05Z07:00")
			lastSeen = &value
		}
		views = append(views, adminDeviceItemView{
			ID: item.Device.ID, DeviceSN: item.Device.SerialNo, Name: item.Device.Name,
			Type: item.Device.DeviceType, Status: item.Device.Status, PlotID: item.PlotID,
			PlotCode: item.PlotCode, PlotName: item.PlotName, OwnerName: item.OwnerName,
			FirmwareVersion: item.Device.FirmwareVersion, LastSeenAt: lastSeen,
		})
	}
	respondSuccess(c, http.StatusOK, gin.H{
		"items": views, "page": filter.Page, "pageSize": filter.PageSize, "total": total,
	})
}

func (h adminDeviceHandler) bind(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, 40101, "未登录或访问令牌无效")
		return
	}
	var request bindDeviceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：请求体格式不正确")
		return
	}
	result, err := h.service.AdminBind(c.Request.Context(), claims.UserID, device.BindInput{
		SerialNo: strings.TrimSpace(request.DeviceSN), PlotID: request.PlotID,
		Name: strings.TrimSpace(request.Name), DeviceType: strings.TrimSpace(request.Type),
	})
	switch {
	case errors.Is(err, device.ErrInvalidInput):
		respondError(c, http.StatusBadRequest, 40001, "参数错误：deviceSn、plotId、name 和 type 不能为空且长度必须合法")
	case errors.Is(err, device.ErrNotFound):
		respondError(c, http.StatusNotFound, 40401, "地块不存在")
	case errors.Is(err, device.ErrAlreadyBound):
		respondError(c, http.StatusConflict, 40901, "绑定失败：该设备已绑定到其他地块，请先解绑")
	case errors.Is(err, device.ErrDeviceTypeMismatch):
		respondError(c, http.StatusConflict, 40901, "设备类型与已登记信息不一致")
	case err != nil:
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
	default:
		respondSuccess(c, http.StatusOK, gin.H{"id": result.ID, "deviceSn": result.SerialNo, "status": result.Status})
	}
}

func (h adminDeviceHandler) unbind(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("deviceId"), 10, 64)
	if err != nil || deviceID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：deviceId 必须为正整数")
		return
	}
	if err := h.service.AdminUnbind(c.Request.Context(), deviceID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondError(c, http.StatusNotFound, 40402, "设备不存在或未绑定")
			return
		}
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	respondSuccess(c, http.StatusOK, true)
}

func (h adminDeviceHandler) delete(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("deviceId"), 10, 64)
	if err != nil || deviceID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：deviceId 必须为正整数")
		return
	}
	if err := h.service.AdminDelete(c.Request.Context(), deviceID); err != nil {
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			respondError(c, http.StatusNotFound, 40402, "设备不存在")
		case errors.Is(err, device.ErrDeviceHasCommands):
			respondError(c, http.StatusConflict, 40901, "设备存在命令记录，无法删除（保留操作历史）")
		default:
			respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		}
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{"id": deviceID, "deleted": true})
}

func parseAdminDeviceListFilter(c *gin.Context) (device.ListFilter, error) {
	filter := device.ListFilter{Page: 1, PageSize: 20, DeviceType: strings.TrimSpace(c.Query("type"))}
	if value := c.Query("plotId"); value != "" {
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil || parsed == 0 {
			return filter, errors.New("参数错误：plotId 必须为正整数")
		}
		filter.PlotID = &parsed
	}
	if value := strings.TrimSpace(c.Query("status")); value != "" {
		status := device.Status(strings.ToUpper(value))
		if !device.ValidStatus(status) {
			return filter, errors.New("参数错误：status 不合法")
		}
		filter.Status = &status
	}
	var err error
	if value := c.Query("page"); value != "" {
		filter.Page, err = strconv.Atoi(value)
		if err != nil || filter.Page < 1 {
			return filter, errors.New("参数错误：page 必须为正整数")
		}
	}
	if value := c.Query("pageSize"); value != "" {
		filter.PageSize, err = strconv.Atoi(value)
		if err != nil || filter.PageSize < 1 || filter.PageSize > 100 {
			return filter, errors.New("参数错误：pageSize 必须为 1 到 100 的整数")
		}
	}
	return filter, nil
}
