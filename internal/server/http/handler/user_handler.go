package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"storemesh-user-service/internal/domain"
	httpmiddleware "storemesh-user-service/internal/server/http/middleware"
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
	// Public user registration.
	router.POST(
		"/users",
		h.CreateUser,
	)

	// Protected self-service user routes.
	protectedUsers := router.Group(
		"/users",
	)

	protectedUsers.Use(
		httpmiddleware.Authentication(
			h.service,
		),
	)

	protectedUsers.GET(
		"/:id",
		httpmiddleware.RequireSelfOrRole(
			"id",
			"admin",
		),
		h.GetUser,
	)

	protectedUsers.PUT(
		"/:id",
		httpmiddleware.RequireSelfOrRole(
			"id",
			"admin",
		),
		h.UpdateUser,
	)

	protectedUsers.PATCH(
		"/:id",
		httpmiddleware.RequireSelfOrRole(
			"id",
			"admin",
		),
		h.UpdateUser,
	)

	protectedUsers.DELETE(
		"/:id",
		httpmiddleware.RequireSelfOrRole(
			"id",
			"admin",
		),
		h.DeleteUser,
	)

	authRoutes := router.Group(
		"/auth",
	)

	authRoutes.POST(
		"/login",
		h.Authenticate,
	)

	authRoutes.POST(
		"/refresh",
		h.RefreshToken,
	)

	// Logout operations validate the access token inside the service because
	// they also revoke the associated Redis-backed authentication session.
	authRoutes.POST(
		"/logout",
		h.Logout,
	)

	authRoutes.POST(
		"/logout-all",
		h.LogoutAll,
	)
}
