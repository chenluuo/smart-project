package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/identity"
	"github.com/chenluuo/smart-project/backend/internal/plot"
	"github.com/gin-gonic/gin"
)

type plotHandler struct {
	service plotService
}

type plotListItem struct {
	ID           uint64      `json:"id"`
	Code         string      `json:"code"`
	Name         string      `json:"name"`
	Status       plot.Status `json:"status"`
	SoilMoisture *float64    `json:"soilMoisture"`
	Temperature  *float64    `json:"temperature"`
	DeviceStatus *string     `json:"deviceStatus"`
	AlertCount   int64       `json:"alertCount"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

type plotDetail struct {
	ID           uint64      `json:"id"`
	Code         string      `json:"code"`
	Name         string      `json:"name"`
	CropName     *string     `json:"cropName"`
	PlantingTime *time.Time  `json:"plantingTime,omitempty"`
	Area         *float64    `json:"area"`
	Status       plot.Status `json:"status"`
	CreatedAt    time.Time   `json:"createdAt"`
}

type updateCropRequest struct {
	CropName string `json:"cropName"`
}

func registerPlotRoutes(router *gin.Engine, auth authService, service plotService) {
	handler := plotHandler{service: service}
	plots := router.Group("/api/v1/plots", jwtAuthentication(auth))
	plots.GET("", handler.list)
	plots.GET("/:plotId", handler.get)
	plots.POST("/:plotId/crop", handler.updateCrop)
}

func (h plotHandler) list(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, 40101, "未登录或访问令牌无效")
		return
	}

	results, err := h.service.List(c.Request.Context(), claims.UserID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	items := make([]plotListItem, 0, len(results))
	for _, result := range results {
		items = append(items, plotListItem{
			ID: result.ID, Code: result.Code, Name: result.Name, Status: result.Status,
			UpdatedAt: result.UpdatedAt,
		})
	}
	respondSuccess(c, http.StatusOK, items)
}

func (h plotHandler) get(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, 40101, "未登录或访问令牌无效")
		return
	}
	plotID, err := strconv.ParseUint(c.Param("plotId"), 10, 64)
	if err != nil || plotID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：plotId 必须为正整数")
		return
	}

	result, err := h.service.Get(c.Request.Context(), claims.UserID, plotID)
	if errors.Is(err, plot.ErrNotFound) {
		respondError(c, http.StatusNotFound, 40401, "地块不存在")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}

	var area *float64
	if result.Area != nil {
		value, _ := result.Area.Float64()
		area = &value
	}
	respondSuccess(c, http.StatusOK, plotDetail{
		ID: result.ID, Code: result.Code, Name: result.Name, CropName: result.CropType,
		PlantingTime: result.PlantingTime, Area: area, Status: result.Status, CreatedAt: result.CreatedAt,
	})
}

func (h plotHandler) updateCrop(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		respondError(c, http.StatusUnauthorized, 40101, "未登录或访问令牌无效")
		return
	}
	plotID, err := strconv.ParseUint(c.Param("plotId"), 10, 64)
	if err != nil || plotID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：plotId 必须为正整数")
		return
	}
	var request updateCropRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：请求体格式不正确")
		return
	}

	result, err := h.service.UpdateCrop(c.Request.Context(), claims.UserID, plotID, request.CropName)
	switch {
	case errors.Is(err, plot.ErrInvalidInput):
		respondError(c, http.StatusBadRequest, 40001, "参数错误：cropName 不能为空且长度不能超过 64 个字符")
	case errors.Is(err, plot.ErrNotFound):
		respondError(c, http.StatusNotFound, 40401, "地块不存在")
	case err != nil:
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
	default:
		respondSuccess(c, http.StatusOK, gin.H{
			"id":           result.ID,
			"cropName":     result.CropType,
			"plantingTime": result.PlantingTime,
		})
	}
}

func authenticatedClaims(c *gin.Context) (identity.Claims, bool) {
	value, exists := c.Get(claimsContextKey)
	claims, ok := value.(identity.Claims)
	return claims, exists && ok
}
