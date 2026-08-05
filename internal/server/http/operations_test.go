package httpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"storemesh-user-service/internal/health"
	"storemesh-user-service/internal/observability"
)

func TestReadiness_ReturnsServiceUnavailableWhenDependencyIsDown(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterOperationalRoutes(
		router,
		health.NewChecker(
			time.Second,
			health.Dependency{
				Name: "postgres",
				Check: func(context.Context) error {
					return errors.New("unavailable")
				},
			},
		),
		observability.NewHTTPMetrics(),
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/readyz", nil),
	)

	assert.Equal(t, http.StatusServiceUnavailable, response.Code)
	assert.JSONEq(
		t,
		`{"status":"not_ready","dependencies":{"postgres":"down"}}`,
		response.Body.String(),
	)
}

func TestLiveness_DoesNotCheckDependencies(t *testing.T) {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	RegisterOperationalRoutes(
		router,
		health.NewChecker(time.Second),
		observability.NewHTTPMetrics(),
	)

	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/healthz", nil),
	)

	assert.Equal(t, http.StatusOK, response.Code)
	assert.JSONEq(t, `{"status":"ok"}`, response.Body.String())
}
