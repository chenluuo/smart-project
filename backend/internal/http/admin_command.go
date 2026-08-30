package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/chenluuo/smart-project/backend/internal/control"
	"github.com/gin-gonic/gin"
)

// adminCommandService 管理后台命令记录查询（复用 device_commands 全量）。
type adminCommandService interface {
	AdminList(context.Context, control.ListFilter) ([]control.CommandListRow, int64, error)
}

type adminCommandHandler struct {
	service adminCommandService
}

type adminCommandItemView struct {
	ID           uint64          `json:"id"`
	CommandID    string          `json:"commandId"`
	PlotID       uint64          `json:"plotId"`
	PlotCode     string          `json:"plotCode"`
	PlotName     string          `json:"plotName"`
	DeviceID     uint64          `json:"deviceId"`
	IssuedBy     uint64          `json:"issuedBy"`
	OperatorName string          `json:"operatorName"`
	Action       control.Action  `json:"action"`
	Parameters   json.RawMessage `json:"parameters"`
	Status       control.Status  `json:"status"`
	ErrorCode    *string         `json:"errorCode,omitempty"`
	ErrorMessage *string         `json:"errorMessage,omitempty"`
	IssuedAt     string          `json:"issuedAt"`
	ExpiresAt    string          `json:"expiresAt"`
	ExecutedAt   *string         `json:"executedAt,omitempty"`
	CreatedAt    string          `json:"createdAt"`
}

// registerAdminCommandRoutes 命令记录：SYSTEM_ADMIN 专属（管理员面板"命令记录"页）。
func registerAdminCommandRoutes(router *gin.Engine, auth authService, service adminCommandService) {
	handler := adminCommandHandler{service: service}
	admin := router.Group("/api/v1/admin", jwtAuthentication(auth), requireSystemAdmin())
	admin.GET("/commands", handler.list)
}

func (h adminCommandHandler) list(c *gin.Context) {
	filter := control.ListFilter{Page: 1, PageSize: 20}
	if raw := c.Query("plotId"); raw != "" {
		id, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || id == 0 {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：plotId 必须为正整数")
			return
		}
		filter.PlotID = &id
	}
	if raw := c.Query("status"); raw != "" {
		status := control.Status(raw)
		if !control.ValidStatus(status) {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：status 不合法")
			return
		}
		filter.Status = &status
	}
	page, pageSize, ok := pagination(c)
	if !ok {
		return
	}
	filter.Page, filter.PageSize = page, pageSize

	rows, total, err := h.service.AdminList(c.Request.Context(), filter)
	if errors.Is(err, control.ErrInvalidInput) {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：分页、plotId 或 status 不合法")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}

	views := make([]adminCommandItemView, 0, len(rows))
	for _, row := range rows {
		view := adminCommandItemView{
			ID: row.ID, CommandID: row.CommandID, PlotID: row.PlotID,
			PlotCode: row.PlotCode, PlotName: row.PlotName, DeviceID: row.DeviceID,
			IssuedBy: row.IssuedBy, OperatorName: row.OperatorName,
			Action: row.Action, Parameters: json.RawMessage(row.ParametersJSON),
			Status: row.Status, ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage,
			IssuedAt: row.IssuedAt.Format("2006-01-02T15:04:05Z07:00"),
			ExpiresAt: row.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
			CreatedAt: row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if row.ExecutedAt != nil {
			value := row.ExecutedAt.Format("2006-01-02T15:04:05Z07:00")
			view.ExecutedAt = &value
		}
		views = append(views, view)
	}
	respondSuccess(c, http.StatusOK, gin.H{
		"items": views, "page": filter.Page, "pageSize": filter.PageSize, "total": total,
	})
}
