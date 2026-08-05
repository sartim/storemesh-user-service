package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storemesh-user-service/internal/domain"
)

type handlerTestService struct {
	domain.UserService

	claims      *domain.TokenClaims
	listResult  *domain.ListUsersResponse
	listErr     error
	listRequest domain.ListUsersRequest
}

func (s *handlerTestService) ValidateToken(
	_ context.Context,
	_ string,
) (*domain.TokenClaims, error) {
	return s.claims, nil
}

func (s *handlerTestService) ListUsers(
	_ context.Context,
	req domain.ListUsersRequest,
) (*domain.ListUsersResponse, error) {
	s.listRequest = req
	return s.listResult, s.listErr
}

func TestListUsers_ReturnsPaginatedUsersForAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &handlerTestService{
		claims: &domain.TokenClaims{
			UserID:    "admin-1",
			Roles:     []string{domain.RoleAdmin},
			TokenType: domain.TokenTypeAccess,
		},
		listResult: &domain.ListUsersResponse{
			Users: []*domain.User{
				{
					ID:     "user-1",
					Email:  "customer@example.com",
					Status: domain.StatusActive,
					Roles:  []domain.Role{{Name: domain.RoleCustomer}},
				},
			},
			TotalItems: 1,
			TotalPages: 1,
			Page:       2,
			PerPage:    10,
		},
	}

	router := gin.New()
	handler := NewUserHandler(service, zap.NewNop())
	handler.RegisterRoutes(router.Group("/api/v1"))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/users?page=2&per_page=10&status=active",
		nil,
	)
	request.Header.Set("Authorization", "Bearer access-token")

	router.ServeHTTP(response, request)

	require.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, 2, service.listRequest.Page)
	assert.Equal(t, 10, service.listRequest.PerPage)
	assert.Equal(t, domain.StatusActive, service.listRequest.Status)

	var payload listUsersResponse
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	assert.Len(t, payload.Users, 1)
	assert.Equal(t, int64(1), payload.TotalItems)
}

func TestListUsers_RejectsNonAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)

	service := &handlerTestService{
		claims: &domain.TokenClaims{
			UserID:    "customer-1",
			Roles:     []string{domain.RoleCustomer},
			TokenType: domain.TokenTypeAccess,
		},
	}

	router := gin.New()
	NewUserHandler(service, zap.NewNop()).RegisterRoutes(router.Group("/api/v1"))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	request.Header.Set("Authorization", "Bearer access-token")

	router.ServeHTTP(response, request)

	assert.Equal(t, http.StatusForbidden, response.Code)
}

func TestWriteError_MapsDomainErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid input", err: domain.ErrInvalidInput, wantStatus: http.StatusBadRequest},
		{name: "duplicate", err: domain.ErrAlreadyExists, wantStatus: http.StatusConflict},
		{name: "missing", err: domain.ErrNotFound, wantStatus: http.StatusNotFound},
		{name: "invalid credentials", err: domain.ErrInvalidPassword, wantStatus: http.StatusUnauthorized},
		{name: "invalid token", err: domain.ErrInvalidToken, wantStatus: http.StatusUnauthorized},
		{name: "forbidden", err: domain.ErrForbidden, wantStatus: http.StatusForbidden},
	}

	handler := NewUserHandler(nil, zap.NewNop())

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(response)

			handler.writeError(context, test.err)

			assert.Equal(t, test.wantStatus, response.Code)
		})
	}
}
