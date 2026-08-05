package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authcontext "storemesh-user-service/internal/auth"
	"storemesh-user-service/internal/domain"
)

type stubTokenValidator struct {
	claimsByToken map[string]*domain.TokenClaims
}

func (
	v *stubTokenValidator,
) ValidateToken(
	_ context.Context,
	token string,
) (*domain.TokenClaims, error) {
	claims, exists := v.claimsByToken[token]
	if !exists {
		return nil, domain.ErrInvalidToken
	}

	clonedClaims := *claims

	clonedClaims.Roles = append(
		[]string(nil),
		claims.Roles...,
	)

	return &clonedClaims, nil
}

func TestAuthentication_RejectsMissingBearerToken(
	t *testing.T,
) {
	gin.SetMode(
		gin.TestMode,
	)

	validator := &stubTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{},
	}

	router := gin.New()

	router.GET(
		"/protected",
		Authentication(validator),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	response := httptest.NewRecorder()

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	router.ServeHTTP(
		response,
		request,
	)

	assert.Equal(
		t,
		http.StatusUnauthorized,
		response.Code,
	)
}

func TestAuthentication_RejectsInvalidToken(
	t *testing.T,
) {
	gin.SetMode(
		gin.TestMode,
	)

	validator := &stubTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{},
	}

	router := gin.New()

	router.GET(
		"/protected",
		Authentication(validator),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	response := httptest.NewRecorder()

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer invalid-token",
	)

	router.ServeHTTP(
		response,
		request,
	)

	assert.Equal(
		t,
		http.StatusUnauthorized,
		response.Code,
	)
}

func TestAuthentication_RejectsRefreshToken(
	t *testing.T,
) {
	gin.SetMode(
		gin.TestMode,
	)

	validator := &stubTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{
			"refresh-token": {
				UserID:    "user-1",
				TokenType: domain.TokenTypeRefresh,
			},
		},
	}

	router := gin.New()

	router.GET(
		"/protected",
		Authentication(validator),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	response := httptest.NewRecorder()

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer refresh-token",
	)

	router.ServeHTTP(
		response,
		request,
	)

	assert.Equal(
		t,
		http.StatusUnauthorized,
		response.Code,
	)
}

func TestAuthentication_AttachesAccessTokenClaims(
	t *testing.T,
) {
	gin.SetMode(
		gin.TestMode,
	)

	validator := &stubTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{
			"access-token": {
				UserID:    "user-1",
				Email:     "user@example.com",
				TokenType: domain.TokenTypeAccess,
			},
		},
	}

	router := gin.New()

	router.GET(
		"/protected",
		Authentication(validator),
		func(c *gin.Context) {
			claims, err := authcontext.RequireClaims(
				c.Request.Context(),
			)

			require.NoError(t, err)

			c.String(
				http.StatusOK,
				claims.UserID,
			)
		},
	)

	response := httptest.NewRecorder()

	request := httptest.NewRequest(
		http.MethodGet,
		"/protected",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer access-token",
	)

	router.ServeHTTP(
		response,
		request,
	)

	assert.Equal(
		t,
		http.StatusOK,
		response.Code,
	)

	assert.Equal(
		t,
		"user-1",
		response.Body.String(),
	)
}

func TestRequireSelfOrRole_AllowsMatchingUser(
	t *testing.T,
) {
	gin.SetMode(
		gin.TestMode,
	)

	validator := &stubTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{
			"access-token": {
				UserID:    "user-1",
				TokenType: domain.TokenTypeAccess,
			},
		},
	}

	router := gin.New()

	router.GET(
		"/users/:id",
		Authentication(validator),
		RequireSelfOrRole(
			"id",
			"admin",
		),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	response := httptest.NewRecorder()

	request := httptest.NewRequest(
		http.MethodGet,
		"/users/user-1",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer access-token",
	)

	router.ServeHTTP(
		response,
		request,
	)

	assert.Equal(
		t,
		http.StatusOK,
		response.Code,
	)
}

func TestRequireSelfOrRole_RejectsDifferentUser(
	t *testing.T,
) {
	gin.SetMode(
		gin.TestMode,
	)

	validator := &stubTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{
			"access-token": {
				UserID:    "user-1",
				TokenType: domain.TokenTypeAccess,
			},
		},
	}

	router := gin.New()

	router.GET(
		"/users/:id",
		Authentication(validator),
		RequireSelfOrRole(
			"id",
			"admin",
		),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	response := httptest.NewRecorder()

	request := httptest.NewRequest(
		http.MethodGet,
		"/users/user-2",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer access-token",
	)

	router.ServeHTTP(
		response,
		request,
	)

	assert.Equal(
		t,
		http.StatusForbidden,
		response.Code,
	)
}

func TestRequireSelfOrRole_AllowsAdministrativeRole(
	t *testing.T,
) {
	gin.SetMode(
		gin.TestMode,
	)

	validator := &stubTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{
			"admin-token": {
				UserID:    "admin-1",
				Roles:     []string{"admin"},
				TokenType: domain.TokenTypeAccess,
			},
		},
	}

	router := gin.New()

	router.GET(
		"/users/:id",
		Authentication(validator),
		RequireSelfOrRole(
			"id",
			"admin",
		),
		func(c *gin.Context) {
			c.Status(http.StatusOK)
		},
	)

	response := httptest.NewRecorder()

	request := httptest.NewRequest(
		http.MethodGet,
		"/users/user-2",
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer admin-token",
	)

	router.ServeHTTP(
		response,
		request,
	)

	assert.Equal(
		t,
		http.StatusOK,
		response.Code,
	)
}

func TestRequireRole_EnforcesAdministrativeRole(t *testing.T) {
	gin.SetMode(gin.TestMode)

	validator := &stubTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{
			"admin-token": {
				UserID:    "admin-1",
				Roles:     []string{domain.RoleAdmin},
				TokenType: domain.TokenTypeAccess,
			},
			"customer-token": {
				UserID:    "customer-1",
				Roles:     []string{domain.RoleCustomer},
				TokenType: domain.TokenTypeAccess,
			},
		},
	}

	for _, test := range []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "admin allowed", token: "admin-token", wantStatus: http.StatusOK},
		{name: "customer forbidden", token: "customer-token", wantStatus: http.StatusForbidden},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.GET(
				"/admin",
				Authentication(validator),
				RequireRole(domain.RoleAdmin),
				func(c *gin.Context) { c.Status(http.StatusOK) },
			)

			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, "/admin", nil)
			request.Header.Set("Authorization", "Bearer "+test.token)

			router.ServeHTTP(response, request)
			assert.Equal(t, test.wantStatus, response.Code)
		})
	}
}
