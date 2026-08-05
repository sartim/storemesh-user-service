package grpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	userv1 "storemesh-user-service/gen/user/v1"
	"storemesh-user-service/internal/domain"
)

type grpcAuthTestService struct {
	domain.UserService

	refreshToken   string
	logoutToken    string
	logoutAllToken string
}

func (s *grpcAuthTestService) RefreshToken(
	_ context.Context,
	refreshToken string,
) (*domain.TokenPair, error) {
	s.refreshToken = refreshToken
	return &domain.TokenPair{
		AccessToken:  "next-access-token",
		RefreshToken: "next-refresh-token",
	}, nil
}

func (s *grpcAuthTestService) Logout(
	_ context.Context,
	accessToken string,
) error {
	s.logoutToken = accessToken
	return nil
}

func (s *grpcAuthTestService) LogoutAll(
	_ context.Context,
	accessToken string,
) error {
	s.logoutAllToken = accessToken
	return nil
}

func TestRefreshToken_ReturnsRotatedTokenPair(t *testing.T) {
	service := &grpcAuthTestService{}
	server := NewUserGRPCServer(service)

	response, err := server.RefreshToken(
		context.Background(),
		&userv1.RefreshTokenRequest{RefreshToken: "refresh-token"},
	)

	require.NoError(t, err)
	assert.Equal(t, "refresh-token", service.refreshToken)
	assert.Equal(t, "next-access-token", response.AccessToken)
	assert.Equal(t, "next-refresh-token", response.RefreshToken)
	assert.Equal(t, "Bearer", response.TokenType)
}

func TestLogout_UsesAuthorizationMetadata(t *testing.T) {
	service := &grpcAuthTestService{}
	server := NewUserGRPCServer(service)
	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer access-token"),
	)

	response, err := server.Logout(ctx, &userv1.LogoutRequest{})

	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "access-token", service.logoutToken)
}

func TestLogoutAll_UsesAuthorizationMetadata(t *testing.T) {
	service := &grpcAuthTestService{}
	server := NewUserGRPCServer(service)
	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs("authorization", "Bearer access-token"),
	)

	response, err := server.LogoutAll(ctx, &userv1.LogoutAllRequest{})

	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, "access-token", service.logoutAllToken)
}
