package http

import (
	"context"
	"net/http"
	"time"

	"github.com/chenluuo/smart-project/backend/internal/identity"
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

type authService interface {
	Register(context.Context, identity.RegisterInput) (*identity.User, error)
	Login(context.Context, string, string) (*identity.LoginResult, error)
	Authenticate(string) (identity.Claims, error)
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
