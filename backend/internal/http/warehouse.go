package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/trade"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type warehouseService interface {
	ListMaterials(context.Context, trade.PageFilter) (trade.ListResult[trade.Material], error)
	GetMaterial(context.Context, uint64) (*trade.Material, error)
	CreateMaterial(context.Context, trade.MaterialInput) (*trade.Material, error)
	UpdateMaterial(context.Context, uint64, trade.MaterialInput) (*trade.Material, error)
	DeleteMaterial(context.Context, uint64) error
	ListWarehouses(context.Context, trade.PageFilter) (trade.ListResult[trade.Warehouse], error)
	GetWarehouse(context.Context, uint64) (*trade.Warehouse, error)
	CreateWarehouse(context.Context, trade.WarehouseInput) (*trade.Warehouse, error)
	UpdateWarehouse(context.Context, uint64, trade.WarehouseInput) (*trade.Warehouse, error)
	DeleteWarehouse(context.Context, uint64) error
	ListStocks(context.Context, trade.StockFilter) (trade.ListResult[trade.StockView], error)
	Inbound(context.Context, trade.InboundInput) (*trade.InboundResult, error)
	ListRecords(context.Context, trade.RecordFilter) (trade.ListResult[trade.RecordView], error)
}

type warehouseHandler struct{ service warehouseService }

func registerWarehouseRoutes(router *gin.Engine, auth authService, service warehouseService) {
	h := warehouseHandler{service: service}
	api := router.Group("/api/v1", jwtAuthentication(auth))
	warehouseRead := requireAnyRole("WAREHOUSE_MANAGER", "SYSTEM_ADMIN")
	warehouseWrite := requireAnyRole("WAREHOUSE_MANAGER", "SYSTEM_ADMIN")
	api.GET("/materials", warehouseRead, h.listMaterials)
	api.GET("/materials/:id", warehouseRead, h.getMaterial)
	api.POST("/materials", warehouseWrite, h.createMaterial)
	api.PUT("/materials/:id", warehouseWrite, h.updateMaterial)
	api.DELETE("/materials/:id", warehouseWrite, h.deleteMaterial)
	api.GET("/warehouses", warehouseRead, h.listWarehouses)
	api.GET("/warehouses/:id", warehouseRead, h.getWarehouse)
	api.POST("/warehouses", warehouseWrite, h.createWarehouse)
	api.PUT("/warehouses/:id", warehouseWrite, h.updateWarehouse)
	api.DELETE("/warehouses/:id", warehouseWrite, h.deleteWarehouse)
	api.GET("/stocks", warehouseRead, h.listStocks)
	api.POST("/stocks/inbound", warehouseWrite, h.inbound)
	api.GET("/stock-records", warehouseRead, h.listRecords)
}

type materialRequest struct {
	Name     string             `json:"name"`
	Category string             `json:"category"`
	Unit     string             `json:"unit"`
	Spec     *string            `json:"spec"`
	Status   trade.MasterStatus `json:"status"`
}
type warehouseRequest struct {
	Name     string             `json:"name"`
	Location string             `json:"location"`
	Status   trade.MasterStatus `json:"status"`
}
type inboundRequest struct {
	WarehouseID uint64       `json:"warehouseId"`
	MaterialID  uint64       `json:"materialId"`
	Quantity    decimalInput `json:"quantity"`
	PlotID      uint64       `json:"plotId"`
	Remark      *string      `json:"remark"`
}
type decimalInput struct{ decimal.Decimal }

func (v *decimalInput) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	if len(text) >= 2 && text[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return err
		}
		text = s
	}
	parsed, err := decimal.NewFromString(text)
	if err != nil {
		return err
	}
	v.Decimal = parsed
	return nil
}

func (h warehouseHandler) listMaterials(c *gin.Context) {
	f, ok := pageFilter(c)
	if !ok {
		return
	}
	result, err := h.service.ListMaterials(c.Request.Context(), f)
	respondWarehouseResult(c, result, err)
}
func (h warehouseHandler) getMaterial(c *gin.Context) {
	id, ok := warehousePathID(c)
	if !ok {
		return
	}
	result, err := h.service.GetMaterial(c.Request.Context(), id)
	respondWarehouseResult(c, result, err)
}
func (h warehouseHandler) createMaterial(c *gin.Context) {
	var r materialRequest
	if !bindWarehouseJSON(c, &r) {
		return
	}
	result, err := h.service.CreateMaterial(c.Request.Context(), trade.MaterialInput{Name: r.Name, Category: r.Category, Unit: r.Unit, Spec: r.Spec, Status: r.Status})
	respondWarehouseCreated(c, result, err)
}
func (h warehouseHandler) updateMaterial(c *gin.Context) {
	id, ok := warehousePathID(c)
	if !ok {
		return
	}
	var r materialRequest
	if !bindWarehouseJSON(c, &r) {
		return
	}
	result, err := h.service.UpdateMaterial(c.Request.Context(), id, trade.MaterialInput{Name: r.Name, Category: r.Category, Unit: r.Unit, Spec: r.Spec, Status: r.Status})
	respondWarehouseResult(c, result, err)
}
func (h warehouseHandler) deleteMaterial(c *gin.Context) {
	id, ok := warehousePathID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteMaterial(c.Request.Context(), id); err != nil {
		respondWarehouseError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{"id": id})
}

func (h warehouseHandler) listWarehouses(c *gin.Context) {
	f, ok := pageFilter(c)
	if !ok {
		return
	}
	result, err := h.service.ListWarehouses(c.Request.Context(), f)
	respondWarehouseResult(c, result, err)
}
func (h warehouseHandler) getWarehouse(c *gin.Context) {
	id, ok := warehousePathID(c)
	if !ok {
		return
	}
	result, err := h.service.GetWarehouse(c.Request.Context(), id)
	respondWarehouseResult(c, result, err)
}
func (h warehouseHandler) createWarehouse(c *gin.Context) {
	var r warehouseRequest
	if !bindWarehouseJSON(c, &r) {
		return
	}
	result, err := h.service.CreateWarehouse(c.Request.Context(), trade.WarehouseInput{Name: r.Name, Location: r.Location, Status: r.Status})
	respondWarehouseCreated(c, result, err)
}
func (h warehouseHandler) updateWarehouse(c *gin.Context) {
	id, ok := warehousePathID(c)
	if !ok {
		return
	}
	var r warehouseRequest
	if !bindWarehouseJSON(c, &r) {
		return
	}
	result, err := h.service.UpdateWarehouse(c.Request.Context(), id, trade.WarehouseInput{Name: r.Name, Location: r.Location, Status: r.Status})
	respondWarehouseResult(c, result, err)
}
func (h warehouseHandler) deleteWarehouse(c *gin.Context) {
	id, ok := warehousePathID(c)
	if !ok {
		return
	}
	if err := h.service.DeleteWarehouse(c.Request.Context(), id); err != nil {
		respondWarehouseError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{"id": id})
}

func (h warehouseHandler) listStocks(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	f := trade.StockFilter{Page: page, PageSize: size}
	if !optionalUint(c, "warehouseId", &f.WarehouseID) || !optionalUint(c, "materialId", &f.MaterialID) {
		return
	}
	result, err := h.service.ListStocks(c.Request.Context(), f)
	respondWarehouseResult(c, result, err)
}

func (h warehouseHandler) inbound(c *gin.Context) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if key == "" || len(key) > 128 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：Idempotency-Key 必填且不能超过 128 字符")
		return
	}
	claims, ok := authenticatedClaims(c)
	if !ok {
		return
	}
	var r inboundRequest
	if !bindWarehouseJSON(c, &r) {
		return
	}
	result, err := h.service.Inbound(c.Request.Context(), trade.InboundInput{WarehouseID: r.WarehouseID, MaterialID: r.MaterialID, Quantity: r.Quantity.Decimal, PlotID: r.PlotID, Remark: r.Remark, IdempotencyKey: key, OperatorID: claims.UserID})
	respondWarehouseCreated(c, result, err)
}

func (h warehouseHandler) listRecords(c *gin.Context) {
	page, size, ok := pagination(c)
	if !ok {
		return
	}
	f := trade.RecordFilter{Page: page, PageSize: size}
	if !optionalUint(c, "warehouseId", &f.WarehouseID) || !optionalUint(c, "materialId", &f.MaterialID) || !optionalUint(c, "plotId", &f.PlotID) {
		return
	}
	if raw := strings.ToUpper(strings.TrimSpace(c.Query("type"))); raw != "" {
		value := trade.RecordType(raw)
		if value != trade.RecordTypeIn && value != trade.RecordTypeOut {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：type 必须为 IN 或 OUT")
			return
		}
		f.Type = &value
	}
	if !optionalTime(c, "startAt", &f.StartAt) || !optionalTime(c, "endAt", &f.EndAt) {
		return
	}
	if f.StartAt != nil && f.EndAt != nil && f.StartAt.After(*f.EndAt) {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：startAt 不能晚于 endAt")
		return
	}
	result, err := h.service.ListRecords(c.Request.Context(), f)
	respondWarehouseResult(c, result, err)
}

func pageFilter(c *gin.Context) (trade.PageFilter, bool) {
	page, size, ok := pagination(c)
	if !ok {
		return trade.PageFilter{}, false
	}
	f := trade.PageFilter{Keyword: strings.TrimSpace(c.Query("keyword")), Page: page, PageSize: size}
	if raw := strings.ToUpper(strings.TrimSpace(c.Query("status"))); raw != "" {
		status := trade.MasterStatus(raw)
		if status != trade.StatusActive && status != trade.StatusDisabled {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：status 必须为 ACTIVE 或 DISABLED")
			return f, false
		}
		f.Status = &status
	}
	return f, true
}
func warehousePathID(c *gin.Context) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：id 必须为正整数")
		return 0, false
	}
	return id, true
}
func optionalUint(c *gin.Context, name string, target **uint64) bool {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return true
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		respondError(c, http.StatusBadRequest, 40001, "参数错误："+name+" 必须为正整数")
		return false
	}
	*target = &v
	return true
}
func optionalTime(c *gin.Context, name string, target **time.Time) bool {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return true
	}
	v, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误："+name+" 必须为 RFC3339 时间")
		return false
	}
	*target = &v
	return true
}
func bindWarehouseJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：请求体格式不正确")
		return false
	}
	return true
}
func respondWarehouseCreated(c *gin.Context, result any, err error) {
	if err != nil {
		respondWarehouseError(c, err)
		return
	}
	respondSuccess(c, http.StatusCreated, result)
}
func respondWarehouseResult(c *gin.Context, result any, err error) {
	if err != nil {
		respondWarehouseError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, result)
}
func respondWarehouseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, trade.ErrInvalidInput):
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
	case errors.Is(err, trade.ErrNotFound):
		respondError(c, http.StatusNotFound, 40401, "仓储资源不存在")
	case errors.Is(err, trade.ErrConflict), errors.Is(err, trade.ErrInsufficientStock):
		respondError(c, http.StatusConflict, 40901, "仓储资源冲突或库存不足")
	default:
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
	}
}
