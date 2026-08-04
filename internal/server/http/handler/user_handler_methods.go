package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *UserHandler) GetUser(c *gin.Context) {
	user, err := h.service.GetUser(
		c.Request.Context(),
		c.Param("id"),
	)
	if err != nil {
		c.JSON(
			http.StatusNotFound,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		toUserResponse(user),
	)
}

func (h *UserHandler) CreateUser(c *gin.Context) {
	var request createUserRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	user, err := h.service.CreateUser(
		c.Request.Context(),
		request.toDomain(),
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	c.JSON(
		http.StatusCreated,
		toUserResponse(user),
	)
}

func (h *UserHandler) UpdateUser(c *gin.Context) {
	var request updateUserRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	user, err := h.service.UpdateUser(
		c.Request.Context(),
		request.toDomain(c.Param("id")),
	)
	if err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	c.JSON(
		http.StatusOK,
		toUserResponse(user),
	)
}

func (h *UserHandler) DeleteUser(c *gin.Context) {
	if err := h.service.DeleteUser(
		c.Request.Context(),
		c.Param("id"),
	); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)
		return
	}

	c.Status(http.StatusNoContent)
}
