package http

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type databasePinger interface {
	PingContext(context.Context) error
}

func NewRouter(mode string, db databasePinger) *gin.Engine {
	gin.SetMode(mode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	health := healthHandler{db: db}
	actuator := router.Group("/actuator/health")
	actuator.GET("", health.readiness)
	actuator.GET("/liveness", health.liveness)
	actuator.GET("/readiness", health.readiness)

	return router
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
