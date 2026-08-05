package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"storemesh-user-service/internal/domain"
)

func (h *UserHandler) ListRoles(c *gin.Context) {
	roles, err := h.service.ListRoles(c.Request.Context())
	if err != nil {
		writeRoleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"roles": toRoleResponses(roles)})
}

func (h *UserHandler) GetUserRoles(c *gin.Context) {
	roles, err := h.service.GetUserRoles(
		c.Request.Context(),
		c.Param("id"),
	)
	if err != nil {
		writeRoleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"roles": toRoleResponses(roles)})
}

func (h *UserHandler) AssignRole(c *gin.Context) {
	user, err := h.service.AssignRole(
		c.Request.Context(),
		c.Param("id"),
		c.Param("role"),
	)
	if err != nil {
		writeRoleError(c, err)
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *UserHandler) RevokeRole(c *gin.Context) {
	user, err := h.service.RevokeRole(
		c.Request.Context(),
		c.Param("id"),
		c.Param("role"),
	)
	if err != nil {
		writeRoleError(c, err)
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

func writeRoleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrInvalidInput):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrAlreadyExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
	case errors.Is(err, domain.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
