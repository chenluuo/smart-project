package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/chenluuo/smart-project/backend/internal/alert"
	"github.com/gin-gonic/gin"
)

// adminAlertService 管理后台告警服务：查询全部告警记录（不做 owner 归属过滤）。
type adminAlertService interface {
	AdminList(context.Context, alert.ListFilter) (alert.ListResult, error)
}

type adminAlertHandler struct {
	service adminAlertService
}

func registerAdminAlertRoutes(router *gin.Engine, auth authService, service adminAlertService) {
	handler := adminAlertHandler{service: service}
	admin := router.Group("/api/v1/admin", jwtAuthentication(auth), requireAdminOrTechnician())
	admin.GET("/alerts", handler.list)
}

func (h adminAlertHandler) list(c *gin.Context) {
	// 复用用户侧告警列表的筛选解析（plotId/status/startTime/endTime/page/pageSize）。
	filter, err := parseAlertListFilter(c, true)
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, err.Error())
		return
	}
	result, err := h.service.AdminList(c.Request.Context(), filter)
	if errors.Is(err, alert.ErrInvalidInput) {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：告警筛选条件不合法")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	respondSuccess(c, http.StatusOK, result)
}
