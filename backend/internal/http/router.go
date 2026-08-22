package http

import (
	"context"
	"net/http"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/control"
	"github.com/chenluuo/smart-project/backend/internal/device"
	"github.com/chenluuo/smart-project/backend/internal/identity"
	"github.com/chenluuo/smart-project/backend/internal/plot"
	"github.com/gin-gonic/gin"
)

type databasePinger interface {
	PingContext(context.Context) error
}

func NewRouter(mode string, db databasePinger, authServices ...authService) *gin.Engine {
	gin.SetMode(mode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	health := healthHandler{db: db}
	actuator := router.Group("/actuator/health")
	actuator.GET("", health.readiness)
	actuator.GET("/liveness", health.liveness)
	actuator.GET("/readiness", health.readiness)
	if len(authServices) > 0 && authServices[0] != nil {
		registerAuthRoutes(router, authServices[0])
	}

	return router
}

func NewRouterWithPlotService(mode string, db databasePinger, auth authService, plots plotService) *gin.Engine {
	router := NewRouter(mode, db, auth)
	registerPlotRoutes(router, auth, plots)
	return router
}

func NewRouterWithServices(mode string, db databasePinger, auth authService, plots plotService, devices deviceService, controls ...controlService) *gin.Engine {
	router := NewRouter(mode, db, auth)
	if plots != nil {
		registerPlotRoutes(router, auth, plots)
	}
	if devices != nil {
		registerDeviceRoutes(router, auth, devices)
	}
	if len(controls) > 0 && controls[0] != nil {
		registerControlRoutes(router, auth, controls[0])
	}
	return router
}

type authService interface {
	Register(context.Context, identity.RegisterInput) (*identity.User, error)
	Login(context.Context, string, string) (*identity.LoginResult, error)
	Authenticate(string) (identity.Claims, error)
}

type plotService interface {
	List(context.Context, uint64) ([]plot.Plot, error)
	Get(context.Context, uint64, uint64) (*plot.Plot, error)
}

type deviceService interface {
	List(context.Context, uint64, device.ListFilter) (device.ListResult, error)
	Bind(context.Context, uint64, device.BindInput) (*device.Device, error)
	Unbind(context.Context, uint64, uint64) error
	Status(context.Context, uint64, uint64) (*device.Device, error)
}

type controlService interface {
	Issue(context.Context, uint64, uint64, control.IssueInput) (*control.IssueResult, error)
	IrrigationStatus(context.Context, uint64, uint64) (*control.IrrigationStatus, error)
	Command(context.Context, uint64, string) (*control.CommandResult, error)
	List(context.Context, uint64, control.ListFilter) (control.ListResult, error)
}

type healthHandler struct {
	db databasePinger
}

func (h healthHandler) liveness(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "UP"})
}

func (h healthHandler) readiness(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
	defer cancel()
	if h.db == nil || h.db.PingContext(ctx) != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "DOWN"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "UP"})
}
