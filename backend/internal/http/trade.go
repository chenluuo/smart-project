package http

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/trade"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// tradeService 贸易模块服务接口（由 trade.Service 实现）。
type tradeService interface {
	CreateMaterial(context.Context, trade.MaterialInput) (*trade.Material, error)
	UpdateMaterial(context.Context, uint64, trade.MaterialInput) (*trade.Material, error)
	DeleteMaterial(context.Context, uint64) error
	ListMaterials(context.Context) ([]trade.Material, error)
	ListMarketMaterials(context.Context) ([]trade.MarketMaterialView, error)

	CreateWarehouse(context.Context, trade.WarehouseInput) (*trade.Warehouse, error)
	UpdateWarehouse(context.Context, uint64, trade.WarehouseInput) (*trade.Warehouse, error)
	DeleteWarehouse(context.Context, uint64) error
	ListWarehouses(context.Context) ([]trade.Warehouse, error)

	StockIn(context.Context, uint64, uint64, decimal.Decimal) (*trade.Stock, error)
	ListStocks(context.Context) ([]trade.StockView, error)
	ListStockRecords(context.Context, trade.StockRecordFilter) ([]trade.StockRecordView, int64, error)

	CreateOrder(context.Context, trade.CreateOrderInput) (*trade.OrderHeader, []trade.OrderItem, error)
	ListOrders(context.Context, uint64, string, trade.OrderListFilter) ([]trade.OrderView, int64, error)
	GetOrder(context.Context, uint64) (*trade.OrderDetail, error)
	Review(context.Context, uint64, bool) (*trade.OrderHeader, error)
	StartTrade(context.Context, uint64) (*trade.OrderHeader, error)
	Terminate(context.Context, uint64, bool) (*trade.OrderHeader, error)
	Confirm(context.Context, uint64, []trade.ConfirmItem) (*trade.OrderHeader, error)
}

type tradeHandler struct {
	service tradeService
}

func registerTradeRoutes(router *gin.Engine, auth authService, service tradeService) {
	handler := tradeHandler{service: service}

	market := router.Group("/api/v1/market", jwtAuthentication(auth))
	market.GET("/materials", handler.listMarketMaterials)

	orders := router.Group("/api/v1/orders", jwtAuthentication(auth))
	orders.POST("", requireAnyRole("CUSTOMER"), handler.createOrder)
	orders.GET("", handler.listOrders)
	orders.GET("/:orderId", handler.getOrder)
	orders.POST("/:orderId/review", requireAdminOrWarehouseManager(), handler.review)
	orders.POST("/:orderId/start-trade", requireAdminOrWarehouseManager(), handler.startTrade)
	orders.POST("/:orderId/terminate", handler.terminate)
	orders.POST("/:orderId/confirm", requireAdminOrWarehouseManager(), handler.confirm)

	stocks := router.Group("/api/v1/stocks", jwtAuthentication(auth))
	stocks.GET("", handler.listStocks)
	stocks.POST("", requireAdminOrWarehouseManager(), handler.stockIn)

	records := router.Group("/api/v1/stock-records", jwtAuthentication(auth))
	records.GET("", handler.listStockRecords)

	materials := router.Group("/api/v1/materials", jwtAuthentication(auth))
	materials.GET("", handler.listMaterials)
	materials.POST("", requireAdminOrWarehouseManager(), handler.createMaterial)
	materials.PUT("/:materialId", requireAdminOrWarehouseManager(), handler.updateMaterial)
	materials.DELETE("/:materialId", requireAdminOrWarehouseManager(), handler.deleteMaterial)

	warehouses := router.Group("/api/v1/warehouses", jwtAuthentication(auth))
	warehouses.GET("", handler.listWarehouses)
	warehouses.POST("", requireAdminOrWarehouseManager(), handler.createWarehouse)
	warehouses.PUT("/:warehouseId", requireAdminOrWarehouseManager(), handler.updateWarehouse)
	warehouses.DELETE("/:warehouseId", requireAdminOrWarehouseManager(), handler.deleteWarehouse)
}

// requireAdminOrWarehouseManager 允许系统管理员或仓库管理员。
func requireAdminOrWarehouseManager() gin.HandlerFunc {
	return requireAnyRole("SYSTEM_ADMIN", "WAREHOUSE_MANAGER")
}

// 市场物料

func (h tradeHandler) listMarketMaterials(c *gin.Context) {
	views, err := h.service.ListMarketMaterials(c.Request.Context())
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, views)
}

// 物料主数据

type materialRequest struct {
	Name     string  `json:"name"`
	Category string  `json:"category"`
	Unit     string  `json:"unit"`
	Spec     *string `json:"spec"`
}

func (h tradeHandler) listMaterials(c *gin.Context) {
	materials, err := h.service.ListMaterials(c.Request.Context())
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, materials)
}

func (h tradeHandler) createMaterial(c *gin.Context) {
	var request materialRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
		return
	}
	material, err := h.service.CreateMaterial(c.Request.Context(), trade.MaterialInput{
		Name: request.Name, Category: request.Category, Unit: request.Unit, Spec: request.Spec,
	})
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusCreated, material)
}

func (h tradeHandler) updateMaterial(c *gin.Context) {
	materialID, err := positivePathID(c, "materialId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：materialId 必须为正整数")
		return
	}
	var request materialRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
		return
	}
	material, err := h.service.UpdateMaterial(c.Request.Context(), materialID, trade.MaterialInput{
		Name: request.Name, Category: request.Category, Unit: request.Unit, Spec: request.Spec,
	})
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, material)
}

func (h tradeHandler) deleteMaterial(c *gin.Context) {
	materialID, err := positivePathID(c, "materialId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：materialId 必须为正整数")
		return
	}
	if err := h.service.DeleteMaterial(c.Request.Context(), materialID); err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{"id": materialID, "deleted": true})
}

// 仓库主数据

type warehouseRequest struct {
	Name   string  `json:"name"`
	Code   string  `json:"code"`
	Remark *string `json:"remark"`
}

func (h tradeHandler) listWarehouses(c *gin.Context) {
	warehouses, err := h.service.ListWarehouses(c.Request.Context())
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, warehouses)
}

func (h tradeHandler) createWarehouse(c *gin.Context) {
	var request warehouseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
		return
	}
	warehouse, err := h.service.CreateWarehouse(c.Request.Context(), trade.WarehouseInput{
		Name: request.Name, Code: request.Code, Remark: request.Remark,
	})
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusCreated, warehouse)
}

func (h tradeHandler) updateWarehouse(c *gin.Context) {
	warehouseID, err := positivePathID(c, "warehouseId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：warehouseId 必须为正整数")
		return
	}
	var request warehouseRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
		return
	}
	warehouse, err := h.service.UpdateWarehouse(c.Request.Context(), warehouseID, trade.WarehouseInput{
		Name: request.Name, Code: request.Code, Remark: request.Remark,
	})
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, warehouse)
}

func (h tradeHandler) deleteWarehouse(c *gin.Context) {
	warehouseID, err := positivePathID(c, "warehouseId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：warehouseId 必须为正整数")
		return
	}
	if err := h.service.DeleteWarehouse(c.Request.Context(), warehouseID); err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{"id": warehouseID, "deleted": true})
}

// 库存

type stockInRequest struct {
	WarehouseID uint64          `json:"warehouseId"`
	MaterialID  uint64          `json:"materialId"`
	Quantity    decimal.Decimal `json:"quantity"`
}

func (h tradeHandler) listStocks(c *gin.Context) {
	views, err := h.service.ListStocks(c.Request.Context())
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, views)
}

func (h tradeHandler) stockIn(c *gin.Context) {
	var request stockInRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
		return
	}
	stock, err := h.service.StockIn(c.Request.Context(), request.WarehouseID, request.MaterialID, request.Quantity)
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusCreated, stock)
}

func (h tradeHandler) listStockRecords(c *gin.Context) {
	filter, ok := parseStockRecordFilter(c)
	if !ok {
		return
	}
	items, total, err := h.service.ListStockRecords(c.Request.Context(), filter)
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{
		"items": items, "page": filter.Page, "pageSize": filter.PageSize, "total": total,
	})
}

func parseStockRecordFilter(c *gin.Context) (trade.StockRecordFilter, bool) {
	filter := trade.StockRecordFilter{
		Direction: strings.TrimSpace(c.Query("direction")),
	}
	if raw := strings.TrimSpace(c.Query("materialId")); raw != "" {
		id, err := parseQueryID(raw)
		if err != nil {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：materialId 必须为正整数")
			return filter, false
		}
		filter.MaterialID = id
	}
	if raw := strings.TrimSpace(c.Query("warehouseId")); raw != "" {
		id, err := parseQueryID(raw)
		if err != nil {
			respondError(c, http.StatusBadRequest, 40001, "参数错误：warehouseId 必须为正整数")
			return filter, false
		}
		filter.WarehouseID = id
	}
	page, pageSize, ok := pagination(c)
	if !ok {
		return filter, false
	}
	filter.Page, filter.PageSize = page, pageSize
	return filter, true
}

// 订单

type orderItemRequest struct {
	MaterialID uint64          `json:"materialId"`
	Quantity   decimal.Decimal `json:"quantity"`
}

type createOrderRequest struct {
	ExpectedTime *time.Time         `json:"expectedTime"`
	Remark       string             `json:"remark"`
	Items        []orderItemRequest `json:"items"`
}

func (h tradeHandler) createOrder(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		return
	}
	var request createOrderRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
		return
	}
	items := make([]trade.OrderItemInput, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, trade.OrderItemInput{MaterialID: item.MaterialID, Quantity: item.Quantity})
	}
	order, orderItems, err := h.service.CreateOrder(c.Request.Context(), trade.CreateOrderInput{
		CustomerID: claims.UserID, ExpectedTime: request.ExpectedTime, Remark: request.Remark, Items: items,
	})
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusCreated, gin.H{"order": order, "items": orderItems})
}

func (h tradeHandler) listOrders(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		return
	}
	filter, ok := parseOrderFilter(c)
	if !ok {
		return
	}
	items, total, err := h.service.ListOrders(c.Request.Context(), claims.UserID, claims.Role, filter)
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, gin.H{
		"items": items, "page": filter.Page, "pageSize": filter.PageSize, "total": total,
	})
}

func (h tradeHandler) getOrder(c *gin.Context) {
	orderID, err := positivePathID(c, "orderId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：orderId 必须为正整数")
		return
	}
	detail, err := h.service.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, detail)
}

func (h tradeHandler) review(c *gin.Context) {
	orderID, err := positivePathID(c, "orderId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：orderId 必须为正整数")
		return
	}
	var request struct {
		Action string `json:"action"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
		return
	}
	action := strings.ToLower(strings.TrimSpace(request.Action))
	if action != "approve" && action != "reject" {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：action 必须为 approve 或 reject")
		return
	}
	order, err := h.service.Review(c.Request.Context(), orderID, action == "approve")
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, order)
}

func (h tradeHandler) startTrade(c *gin.Context) {
	orderID, err := positivePathID(c, "orderId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：orderId 必须为正整数")
		return
	}
	order, err := h.service.StartTrade(c.Request.Context(), orderID)
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, order)
}

func (h tradeHandler) terminate(c *gin.Context) {
	claims, ok := authenticatedClaims(c)
	if !ok {
		return
	}
	orderID, err := positivePathID(c, "orderId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：orderId 必须为正整数")
		return
	}
	var cancel bool
	switch claims.Role {
	case "CUSTOMER":
		cancel = true
	case "SYSTEM_ADMIN", "WAREHOUSE_MANAGER":
		cancel = false
	default:
		respondError(c, http.StatusForbidden, 40301, "无权限执行此操作")
		return
	}
	order, err := h.service.Terminate(c.Request.Context(), orderID, cancel)
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, order)
}

type confirmItemRequest struct {
	MaterialID uint64          `json:"materialId"`
	Quantity   decimal.Decimal `json:"quantity"`
}

type confirmRequest struct {
	Items []confirmItemRequest `json:"items"`
}

func (h tradeHandler) confirm(c *gin.Context) {
	orderID, err := positivePathID(c, "orderId")
	if err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误：orderId 必须为正整数")
		return
	}
	var request confirmRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
		return
	}
	items := make([]trade.ConfirmItem, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, trade.ConfirmItem{MaterialID: item.MaterialID, Quantity: item.Quantity})
	}
	order, err := h.service.Confirm(c.Request.Context(), orderID, items)
	if err != nil {
		respondTradeError(c, err)
		return
	}
	respondSuccess(c, http.StatusOK, order)
}

func parseOrderFilter(c *gin.Context) (trade.OrderListFilter, bool) {
	filter := trade.OrderListFilter{
		Keyword: strings.TrimSpace(c.Query("keyword")),
	}
	if raw := strings.TrimSpace(c.Query("status")); raw != "" {
		status := trade.OrderStatus(strings.ToUpper(raw))
		switch status {
		case trade.OrderStatusPending, trade.OrderStatusApproved, trade.OrderStatusTrading,
			trade.OrderStatusConfirmed, trade.OrderStatusClosed, trade.OrderStatusRejected:
		default:
			respondError(c, http.StatusBadRequest, 40001, "参数错误：status 非法")
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

func parseQueryID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid positive id")
	}
	return id, nil
}

func respondTradeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, trade.ErrInvalidInput):
		respondError(c, http.StatusBadRequest, 40001, "参数错误")
	case errors.Is(err, trade.ErrNotFound):
		respondError(c, http.StatusNotFound, 40405, "资源不存在")
	case errors.Is(err, trade.ErrConflict):
		respondError(c, http.StatusConflict, 40904, "状态冲突或库存不足")
	default:
		respondError(c, http.StatusInternalServerError, 50000, "服务器内部错误")
	}
}
