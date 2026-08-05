package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *UserHandler) ListRoles(c *gin.Context) {
	roles, err := h.service.ListRoles(c.Request.Context())
	if err != nil {
		h.writeError(c, err)
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
		h.writeError(c, err)
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
		h.writeError(c, err)
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
		h.writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}
