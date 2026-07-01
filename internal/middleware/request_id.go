package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const RequestIDHeader = "X-Request-ID"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {

		id := c.Request.Header.Get(RequestIDHeader)

		if id == "" {
			id = uuid.NewString()
		}

		c.Writer.Header().Set(RequestIDHeader, id)

		c.Set("requestID", id)

		c.Next()
	}
}
