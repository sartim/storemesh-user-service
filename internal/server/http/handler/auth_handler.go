package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"storemesh-user-service/internal/domain"
)

func (h *UserHandler) Authenticate(
	c *gin.Context,
) {
	var request authenticateRequest

	if err := c.ShouldBindJSON(
		&request,
	); err != nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid request body",
			},
		)

		return
	}

	user, pair, err := h.service.Authenticate(
		c.Request.Context(),
		request.toDomain(),
	)
	if err != nil {
		h.writeAuthenticationError(
			c,
			err,
		)

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
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "invalid request body",
			},
		)

		return
	}

	if strings.TrimSpace(
		request.RefreshToken,
	) == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": "refresh_token is required",
			},
		)

		return
	}

	pair, err := h.service.RefreshToken(
		c.Request.Context(),
		request.RefreshToken,
	)
	if err != nil {
		h.writeAuthenticationError(
			c,
			err,
		)

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
	accessToken, err := bearerToken(
		c.GetHeader("Authorization"),
	)
	if err != nil {
		h.writeAuthenticationError(
			c,
			err,
		)

		return
	}

	if err := h.service.Logout(
		c.Request.Context(),
		accessToken,
	); err != nil {
		h.writeAuthenticationError(
			c,
			err,
		)

		return
	}

	c.Status(
		http.StatusNoContent,
	)
}

func (h *UserHandler) LogoutAll(
	c *gin.Context,
) {
	accessToken, err := bearerToken(
		c.GetHeader("Authorization"),
	)
	if err != nil {
		h.writeAuthenticationError(
			c,
			err,
		)

		return
	}

	if err := h.service.LogoutAll(
		c.Request.Context(),
		accessToken,
	); err != nil {
		h.writeAuthenticationError(
			c,
			err,
		)

		return
	}

	c.Status(
		http.StatusNoContent,
	)
}

func (h *UserHandler) writeAuthenticationError(
	c *gin.Context,
	err error,
) {
	switch {
	case errors.Is(
		err,
		domain.ErrInvalidInput,
	):
		c.JSON(
			http.StatusBadRequest,
			gin.H{
				"error": err.Error(),
			},
		)

	case errors.Is(
		err,
		domain.ErrInvalidPassword,
	):
		c.JSON(
			http.StatusUnauthorized,
			gin.H{
				"error": "invalid credentials",
			},
		)

	case errors.Is(
		err,
		domain.ErrInvalidToken,
	),
		errors.Is(
			err,
			domain.ErrUnauthorized,
		),
		errors.Is(
			err,
			domain.ErrNotFound,
		):
		c.JSON(
			http.StatusUnauthorized,
			gin.H{
				"error": "invalid or expired token",
			},
		)

	case errors.Is(
		err,
		domain.ErrForbidden,
	):
		c.JSON(
			http.StatusForbidden,
			gin.H{
				"error": "account is not active",
			},
		)

	default:
		h.log.Error(
			"authentication request failed",
			zap.Error(err),
		)

		c.JSON(
			http.StatusInternalServerError,
			gin.H{
				"error": "internal server error",
			},
		)
	}
}

func bearerToken(
	authorizationHeader string,
) (string, error) {
	parts := strings.Fields(
		strings.TrimSpace(
			authorizationHeader,
		),
	)

	if len(parts) != 2 {
		return "", domain.ErrUnauthorized
	}

	if !strings.EqualFold(
		parts[0],
		"Bearer",
	) {
		return "", domain.ErrUnauthorized
	}

	token := strings.TrimSpace(
		parts[1],
	)

	if token == "" {
		return "", domain.ErrUnauthorized
	}

	return token, nil
}
