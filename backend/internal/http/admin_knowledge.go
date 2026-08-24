package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/chenluuo/smart-project/backend/internal/knowledge"
	"github.com/gin-gonic/gin"
)

type adminKnowledgeService interface {
	ListAll(context.Context, knowledge.AdminListFilter) ([]knowledge.AdminDocumentView, int64, error)
	Delete(context.Context, uint64, uint64, string) (*knowledge.Document, error)
}

type adminKnowledgeHandler struct {
	service adminKnowledgeService
}

func registerAdminKnowledgeRoutes(router *gin.Engine, auth authService, service adminKnowledgeService) {
	handler := adminKnowledgeHandler{service: service}
	admin := router.Group("/api/v1/admin", jwtAuthentication(auth), requireSystemAdmin())
	admin.GET("/knowledge/docs", handler.list)
	admin.DELETE("/knowledge/docs/:docId", handler.delete)
}

func (h adminKnowledgeHandler) list(c *gin.Context) {
	filter, ok := parseAdminKnowledgeFilter(c)
	if !ok {
		return
	}
	items, total, err := h.service.ListAll(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{
		"items": items, "page": filter.Page, "pageSize": filter.PageSize, "total": total,
	})
}

func (h adminKnowledgeHandler) delete(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, 40101, "未登录或访问令牌无效")
		return
	}
	documentID, err := strconv.ParseUint(c.Param("docId"), 10, 64)
	if err != nil || documentID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：docId 必须为正整数")
		return
	}
	document, err := h.service.Delete(c.Request.Context(), claims.UserID, documentID, c.GetHeader("X-Trace-ID"))
	if err != nil {
		respondKnowledgeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{
		"id": document.ID, "deleted": true, "indexCleanup": "queued",
	})
}

func parseAdminKnowledgeFilter(c *gin.Context) (knowledge.AdminListFilter, bool) {
	filter := knowledge.AdminListFilter{
		Category: strings.TrimSpace(c.Query("category")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		status := knowledge.Status(strings.ToUpper(raw))
		switch status {
		case knowledge.StatusDraft, knowledge.StatusApproved, knowledge.StatusActive, knowledge.StatusArchived:
		default:
			respondError(c, http.StatusBadRequest, 40001, "参数错误：status 必须为 DRAFT、APPROVED、ACTIVE 或 ARCHIVED")
			return filter, false
		}
		filter.Status = &status
	}
	page, pageSize, ok := pagination(c)
	if !ok {
		return filter, false
	}
	filter.Page, filter.PageSize = page, pageSize
	return filter, true
}
