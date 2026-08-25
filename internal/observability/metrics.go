package observability

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HTTPMetrics struct {
	registry *prometheus.Registry

	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inFlight prometheus.Gauge
}

func NewHTTPMetrics() *HTTPMetrics {
	metrics := &HTTPMetrics{
		registry: prometheus.NewRegistry(),
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "storemesh",
				Subsystem: "user_service",
				Name:      "http_requests_total",
				Help:      "Total number of completed HTTP requests.",
			},
			[]string{"method", "route", "status"},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: "storemesh",
				Subsystem: "user_service",
				Name:      "http_request_duration_seconds",
				Help:      "HTTP request duration in seconds.",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"method", "route", "status"},
		),
		inFlight: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Namespace: "storemesh",
				Subsystem: "user_service",
				Name:      "http_requests_in_flight",
				Help:      "Current number of in-flight HTTP requests.",
			},
		),
	}

	metrics.registry.MustRegister(
		metrics.requests,
		metrics.duration,
		metrics.inFlight,
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	return metrics
}

func (m *HTTPMetrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		startedAt := time.Now()
		m.inFlight.Inc()

		defer func() {
			m.inFlight.Dec()

			route := c.FullPath()
			if route == "" {
				route = "unmatched"
			}

			status := strconv.Itoa(c.Writer.Status())
			labels := []string{c.Request.Method, route, status}

			m.requests.WithLabelValues(labels...).Inc()
			m.duration.WithLabelValues(labels...).Observe(time.Since(startedAt).Seconds())
		}()

		c.Next()
	}
}

func (m *HTTPMetrics) Handler() http.Handler {
	return promhttp.HandlerFor(
		m.registry,
		promhttp.HandlerOpts{EnableOpenMetrics: true},
	)
}
