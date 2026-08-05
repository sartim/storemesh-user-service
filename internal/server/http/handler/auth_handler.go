package handler

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	authcontext "storemesh-user-service/internal/auth"
	"storemesh-user-service/internal/domain"
)

func (h *UserHandler) Authenticate(
	c *gin.Context,
) {
	var request authenticateRequest

	if err := c.ShouldBindJSON(
		&request,
	); err != nil {
		writeInvalidRequest(c)

		return
	}

	user, pair, err := h.service.Authenticate(
		c.Request.Context(),
		request.toDomain(),
	)
	if err != nil {
		h.writeError(c, err)

		return
	}

	c.JSON(
		http.StatusOK,
		authenticationResponse{
			User:   toUserResponse(user),
			Tokens: toTokenPairResponse(pair),
		},
	)
}

func (h *UserHandler) RefreshToken(
	c *gin.Context,
) {
	var request refreshTokenRequest

	if err := c.ShouldBindJSON(
		&request,
	); err != nil {
		writeInvalidRequest(c)

		return
	}

	if strings.TrimSpace(
		request.RefreshToken,
	) == "" {
		h.writeError(
			c,
			fmt.Errorf(
				"%w: refresh_token is required",
				domain.ErrInvalidInput,
			),
		)

		return
	}

	pair, err := h.service.RefreshToken(
		c.Request.Context(),
		request.RefreshToken,
	)
	if err != nil {
		h.writeError(c, err)

		return
	}

	c.JSON(
		http.StatusOK,
		toTokenPairResponse(pair),
	)
}

func (h *UserHandler) Logout(
	c *gin.Context,
) {
	accessToken, err := authcontext.ParseBearerToken(
		c.GetHeader("Authorization"),
	)
	if err != nil {
		h.writeError(c, err)

		return
	}

	if err := h.service.Logout(
		c.Request.Context(),
		accessToken,
	); err != nil {
		h.writeError(c, err)

		return
	}

	c.Status(
		http.StatusNoContent,
	)
}

func (h *UserHandler) LogoutAll(
	c *gin.Context,
) {
	accessToken, err := authcontext.ParseBearerToken(
		c.GetHeader("Authorization"),
	)
	if err != nil {
		h.writeError(c, err)

		return
	}

	if err := h.service.LogoutAll(
		c.Request.Context(),
		accessToken,
	); err != nil {
		h.writeError(c, err)

		return
	}

	c.Status(
		http.StatusNoContent,
	)
}
