package httpserver

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"storemesh-user-service/internal/health"
	"storemesh-user-service/internal/observability"
)

func RegisterOperationalRoutes(
	router *gin.Engine,
	readiness *health.Checker,
	metrics *observability.HTTPMetrics,
) {
	router.GET(
		"/healthz",
		func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok"})
		},
	)

	router.GET(
		"/readyz",
		func(c *gin.Context) {
			report := readiness.Check(c.Request.Context())
			status := http.StatusOK
			if !report.Ready() {
				status = http.StatusServiceUnavailable
			}

			c.JSON(status, report)
		},
	)

	router.GET("/metrics", gin.WrapH(metrics.Handler()))
}
