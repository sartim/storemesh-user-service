package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"storemesh-user-service/internal/domain"
)

type UserHandler struct {
	service domain.UserService
	log     *zap.Logger
}

func NewUserHandler(
	service domain.UserService,
	log *zap.Logger,
) *UserHandler {
	return &UserHandler{
		service: service,
		log:     log,
	}
}

func (h *UserHandler) RegisterRoutes(
	router *gin.RouterGroup,
) {
	users := router.Group("/users")

	users.POST("", h.CreateUser)
	users.GET("/:id", h.GetUser)
	users.PUT("/:id", h.UpdateUser)
	users.PATCH("/:id", h.UpdateUser)
	users.DELETE("/:id", h.DeleteUser)

	auth := router.Group("/auth")

	auth.POST("/login", h.Authenticate)
	auth.POST("/refresh", h.RefreshToken)
	auth.POST("/logout", h.Logout)
	auth.POST("/logout-all", h.LogoutAll)
}
