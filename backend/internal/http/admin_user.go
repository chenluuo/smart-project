package http

import (
	"context"
	"net/http"
	"strings"

	"github.com/chenluuo/smart-project/backend/internal/identity"
	"github.com/gin-gonic/gin"
)

type adminUserService interface {
	ListUsers(context.Context, identity.AdminUserFilter) ([]identity.AdminUserView, int64, error)
}

type adminUserHandler struct {
	service adminUserService
}

func registerAdminUserRoutes(router *gin.Engine, auth authService, service adminUserService) {
	handler := adminUserHandler{service: service}
	admin := router.Group("/api/v1/admin", jwtAuthentication(auth), requireSystemAdmin())
	admin.GET("/users", handler.list)
}

func (h adminUserHandler) list(c *gin.Context) {
	filter, ok := parseAdminUserFilter(c)
	if !ok {
		return
	}
	items, total, err := h.service.ListUsers(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{
		"items": items, "page": filter.Page, "pageSize": filter.PageSize, "total": total,
	})
}

func parseAdminUserFilter(c *gin.Context) (identity.AdminUserFilter, bool) {
	filter := identity.AdminUserFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
		Role:    strings.TrimSpace(c.Query("role")),
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		status := identity.UserStatus(strings.ToUpper(raw))
		if status != identity.UserStatusActive && status != identity.UserStatusDisabled {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：status 必须为 ACTIVE 或 DISABLED")
			return filter, false
		}
		filter.Status = status
	}
	page, pageSize, ok := pagination(c)
	if !ok {
		return filter, false
	}
	filter.Page, filter.PageSize = page, pageSize
	return filter, true
}
