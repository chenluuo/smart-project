package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/plot"
	"github.com/gin-gonic/gin"
	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type adminPlotService interface {
	AdminList(context.Context, plot.AdminListFilter) ([]plot.AdminPlotItem, int64, error)
	CreatePlot(context.Context, plot.CreateInput) (*plot.Plot, error)
	AssignOwner(context.Context, uint64, uint64) (*plot.Plot, error)
}

type adminPlotHandler struct {
	service adminPlotService
}

type adminPlotItemView struct {
	ID          uint64       `json:"id"`
	Code        string       `json:"code"`
	Name        string       `json:"name"`
	Area        *float64     `json:"area"`
	Location    *string      `json:"location"`
	Status      plot.Status  `json:"status"`
	OwnerID     uint64       `json:"ownerId"`
	OwnerName   *string      `json:"ownerName"`
	DeviceCount int64        `json:"deviceCount"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

type createPlotRequest struct {
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	Area     *float64 `json:"area"`
	Location *string  `json:"location"`
	OwnerID  *uint64  `json:"ownerId"`
}

type assignOwnerRequest struct {
	OwnerID *uint64 `json:"ownerId"`
}

func registerAdminPlotRoutes(router *gin.Engine, auth authService, service adminPlotService) {
	handler := adminPlotHandler{service: service}
	admin := router.Group("/api/v1/admin", jwtAuthentication(auth), requireSystemAdmin())
	admin.GET("/plots", handler.list)
	admin.POST("/plots", handler.create)
	admin.PUT("/plots/:plotId/owner", handler.assignOwner)
}

func (h adminPlotHandler) list(c *gin.Context) {
	filter, ok := parseAdminPlotFilter(c)
	if !ok {
		return
	}
	items, total, err := h.service.AdminList(c.Request.Context(), filter)
	if err != nil {
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
		return
	}
	views := make([]adminPlotItemView, 0, len(items))
	for _, item := range items {
		var area *float64
		if item.Plot.Area != nil {
			value, _ := item.Plot.Area.Float64()
			area = &value
		}
		views = append(views, adminPlotItemView{
			ID: item.Plot.ID, Code: item.Plot.Code, Name: item.Plot.Name, Area: area,
			Location: item.Plot.Location, Status: item.Plot.Status, OwnerID: item.Plot.OwnerID,
			OwnerName: item.OwnerName, DeviceCount: item.DeviceCount,
			CreatedAt: item.Plot.CreatedAt, UpdatedAt: item.Plot.UpdatedAt,
		})
	}
	respondSuccess(c, http.StatusOK, gin.H{
		"items": views, "page": filter.Page, "pageSize": filter.PageSize, "total": total,
	})
}

func (h adminPlotHandler) create(c *gin.Context) {
	var request createPlotRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：请求体格式不正确")
		return
	}
	input := plot.CreateInput{
		Code: strings.TrimSpace(request.Code), Name: strings.TrimSpace(request.Name),
		Location: trimOptional(request.Location),
	}
	if request.Area != nil {
		area := decimal.NewFromFloat(*request.Area)
		if area.IsNegative() {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：area 不能为负数")
			return
		}
		input.Area = &area
	}
	if request.OwnerID == nil || *request.OwnerID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：ownerId 必填且必须为有效用户")
		return
	}
	input.OwnerID = *request.OwnerID
	if input.Code == "" || len(input.Code) > 32 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：code 必填且长度不能超过 32 个字符")
		return
	}
	if input.Name == "" || len([]rune(input.Name)) > 128 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：name 必填且长度不能超过 128 个字符")
		return
	}
	result, err := h.service.CreatePlot(c.Request.Context(), input)
	if err != nil {
		respondAdminPlotError(c, err, "创建地块失败")
		return
	}
	respondSuccess(c, http.StatusCreated, adminPlotView(result))
}

func (h adminPlotHandler) assignOwner(c *gin.Context) {
	plotID, err := strconv.ParseUint(c.Param("plotId"), 10, 64)
	if err != nil || plotID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：plotId 必须为正整数")
		return
	}
	var request assignOwnerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：请求体格式不正确")
		return
	}
	if request.OwnerID == nil || *request.OwnerID == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：ownerId 必填且必须为有效用户")
		return
	}
	result, err := h.service.AssignOwner(c.Request.Context(), plotID, *request.OwnerID)
	if err != nil {
		respondAdminPlotError(c, err, "分配地块失败")
		return
	}
	respondSuccess(c, http.StatusOK, adminPlotView(result))
}

func respondAdminPlotError(c *gin.Context, err error, fallback string) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound), errors.Is(err, plot.ErrNotFound):
		respondError(c, http.StatusNotFound, 40401, "地块不存在")
	default:
		var mysqlErr *drivermysql.MySQLError
		if errors.As(err, &mysqlErr) {
			switch mysqlErr.Number {
			case 1062: // 唯一索引冲突（owner_id, code）
				respondError(c, http.StatusConflict, 40904, "地块编码与归属用户组合已存在")
				return
			case 1452: // 外键失败：归属用户不存在
				respondError(c, http.StatusBadRequest, 40001, "归属用户不存在")
				return
			}
		}
		respondError(c, http.StatusInternalServerError, 50000, fallback)
	}
}

func adminPlotView(plot *plot.Plot) gin.H {
	var area *float64
	if plot.Area != nil {
		value, _ := plot.Area.Float64()
		area = &value
	}
	return gin.H{
		"id": plot.ID, "code": plot.Code, "name": plot.Name, "area": area,
		"location": plot.Location, "status": plot.Status, "ownerId": plot.OwnerID,
		"createdAt": plot.CreatedAt,
	}
}

func parseAdminPlotFilter(c *gin.Context) (plot.AdminListFilter, bool) {
	filter := plot.AdminListFilter{Keyword: strings.TrimSpace(c.Query("keyword"))}
	if raw := strings.TrimSpace(c.Query("ownerId")); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：ownerId 必须为非负整数")
			return filter, false
		}
		filter.OwnerID = &parsed
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		status := plot.Status(strings.ToUpper(raw))
		if status != plot.StatusActive && status != plot.StatusDisabled {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：status 必须为 ACTIVE 或 DISABLED")
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

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
