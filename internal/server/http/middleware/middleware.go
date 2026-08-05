package middleware

import (
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	requestIDHeader = "X-Request-ID"
	requestIDKey    = "request_id"

	maximumRequestIDLength = 128
)

// RequestID ensures every HTTP request has a request identifier.
//
// A valid client-supplied X-Request-ID is preserved. Otherwise, the service
// generates a UUID and returns it through the response headers.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(
			c.GetHeader(requestIDHeader),
		)

		if requestID == "" ||
			len(requestID) > maximumRequestIDLength {
			requestID = uuid.NewString()
		}

		c.Set(
			requestIDKey,
			requestID,
		)

		c.Header(
			requestIDHeader,
			requestID,
		)

		c.Next()
	}
}

// Logger records one structured log entry after each HTTP request.
func Logger(
	log *zap.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()

		c.Next()

		requestID := c.GetString(
			requestIDKey,
		)

		fields := []zap.Field{
			zap.String(
				"http.request_id",
				requestID,
			),
			zap.String(
				"http.method",
				c.Request.Method,
			),
			zap.String(
				"http.path",
				c.Request.URL.Path,
			),
			zap.Int(
				"http.status",
				c.Writer.Status(),
			),
			zap.Int(
				"http.response_size_bytes",
				c.Writer.Size(),
			),
			zap.Duration(
				"http.duration",
				time.Since(startedAt),
			),
			zap.String(
				"http.client_ip",
				c.ClientIP(),
			),
		}

		if len(c.Errors) > 0 {
			fields = append(
				fields,
				zap.String(
					"http.errors",
					c.Errors.String(),
				),
			)
		}

		switch {
		case c.Writer.Status() >= http.StatusInternalServerError:
			log.Error(
				"HTTP request completed",
				fields...,
			)

		case c.Writer.Status() >= http.StatusBadRequest:
			log.Warn(
				"HTTP request completed",
				fields...,
			)

		default:
			log.Info(
				"HTTP request completed",
				fields...,
			)
		}
	}
}

// Recovery converts panics into a generic HTTP 500 response.
//
// Internal panic details and the stack trace are written only to structured
// logs and are not returned to clients.
func Recovery(
	log *zap.Logger,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			log.Error(
				"panic recovered from HTTP request",
				zap.String(
					"http.request_id",
					c.GetString(requestIDKey),
				),
				zap.String(
					"http.method",
					c.Request.Method,
				),
				zap.String(
					"http.path",
					c.Request.URL.Path,
				),
				zap.Any(
					"panic",
					recovered,
				),
				zap.ByteString(
					"stack",
					debug.Stack(),
				),
			)

			c.AbortWithStatusJSON(
				http.StatusInternalServerError,
				gin.H{
					"error": "internal server error",
				},
			)
		}()

		c.Next()
	}
}

// CORS provides conservative browser support for local development.
//
// In the intended StoreMesh architecture, browsers communicate with the
// GraphQL BFF rather than directly with the user service. Consequently, only
// local Next.js development origins are allowed here.
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := strings.TrimSpace(
			c.GetHeader("Origin"),
		)

		originAllowed := isAllowedOrigin(
			origin,
		)

		if originAllowed {
			c.Header(
				"Access-Control-Allow-Origin",
				origin,
			)

			c.Header(
				"Vary",
				"Origin",
			)

			c.Header(
				"Access-Control-Allow-Methods",
				"GET, POST, PUT, PATCH, DELETE, OPTIONS",
			)

			c.Header(
				"Access-Control-Allow-Headers",
				"Authorization, Content-Type, X-Request-ID",
			)

			c.Header(
				"Access-Control-Expose-Headers",
				"X-Request-ID",
			)

			c.Header(
				"Access-Control-Max-Age",
				"43200",
			)
		}

		if c.Request.Method == http.MethodOptions {
			if origin != "" && !originAllowed {
				c.AbortWithStatus(
					http.StatusForbidden,
				)

				return
			}

			c.AbortWithStatus(
				http.StatusNoContent,
			)

			return
		}

		c.Next()
	}
}

func isAllowedOrigin(
	origin string,
) bool {
	switch origin {
	case "",
		"http://localhost:3000",
		"http://127.0.0.1:3000":
		return true

	default:
		return false
	}
}
