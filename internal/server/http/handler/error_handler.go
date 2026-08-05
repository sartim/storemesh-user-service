package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"storemesh-user-service/internal/domain"
)

func (h *UserHandler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidPassword):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	case errors.Is(err, domain.ErrInvalidToken),
		errors.Is(err, domain.ErrUnauthorized):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
	case errors.Is(err, domain.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	default:
		h.log.Error("HTTP request failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

func writeInvalidRequest(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
}
