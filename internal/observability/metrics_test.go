package observability

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPMetrics_RecordsRouteAndStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metrics := NewHTTPMetrics()
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/users/:id", func(c *gin.Context) {
		c.Status(http.StatusAccepted)
	})

	response := httptest.NewRecorder()
	router.ServeHTTP(
		response,
		httptest.NewRequest(http.MethodGet, "/users/user-1", nil),
	)

	metricsResponse := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(
		metricsResponse,
		httptest.NewRequest(http.MethodGet, "/metrics", nil),
	)

	require.Equal(t, http.StatusOK, metricsResponse.Code)
	body := metricsResponse.Body.String()
	assert.True(t, strings.Contains(
		body,
		`storemesh_user_service_http_requests_total{method="GET",route="/users/:id",status="202"} 1`,
	))
}
