package middleware

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	authcontext "storemesh-user-service/internal/auth"
	"storemesh-user-service/internal/domain"
)

type interceptorTokenValidator struct {
	claimsByToken map[string]*domain.TokenClaims
}

func (
	v *interceptorTokenValidator,
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

func TestAuthentication_PublicMethodBypassesAuthentication(
	t *testing.T,
) {
	validator := &interceptorTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{},
	}

	interceptor := Authentication(
		validator,
		"/user.v1.UserService/CreateUser",
	)

	handlerCalled := false

	response, err := interceptor(
		context.Background(),
		"request",
		&grpc.UnaryServerInfo{
			FullMethod: "/user.v1.UserService/CreateUser",
		},
		func(
			_ context.Context,
			req any,
		) (any, error) {
			handlerCalled = true

			return req, nil
		},
	)

	require.NoError(t, err)

	assert.True(
		t,
		handlerCalled,
	)

	assert.Equal(
		t,
		"request",
		response,
	)
}

func TestAuthentication_ProtectedMethodRejectsMissingMetadata(
	t *testing.T,
) {
	validator := &interceptorTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{},
	}

	interceptor := Authentication(
		validator,
	)

	_, err := interceptor(
		context.Background(),
		nil,
		&grpc.UnaryServerInfo{
			FullMethod: "/user.v1.UserService/GetUser",
		},
		func(
			_ context.Context,
			_ any,
		) (any, error) {
			return nil, nil
		},
	)

	require.Error(t, err)

	assert.Equal(
		t,
		codes.Unauthenticated,
		status.Code(err),
	)
}

func TestAuthentication_RejectsInvalidToken(
	t *testing.T,
) {
	validator := &interceptorTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{},
	}

	interceptor := Authentication(
		validator,
	)

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(
			"authorization",
			"Bearer invalid-token",
		),
	)

	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{
			FullMethod: "/user.v1.UserService/GetUser",
		},
		func(
			_ context.Context,
			_ any,
		) (any, error) {
			return nil, nil
		},
	)

	require.Error(t, err)

	assert.Equal(
		t,
		codes.Unauthenticated,
		status.Code(err),
	)
}

func TestAuthentication_RejectsRefreshToken(
	t *testing.T,
) {
	validator := &interceptorTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{
			"refresh-token": {
				UserID:    "user-1",
				TokenType: domain.TokenTypeRefresh,
			},
		},
	}

	interceptor := Authentication(
		validator,
	)

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(
			"authorization",
			"Bearer refresh-token",
		),
	)

	_, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{
			FullMethod: "/user.v1.UserService/GetUser",
		},
		func(
			_ context.Context,
			_ any,
		) (any, error) {
			return nil, nil
		},
	)

	require.Error(t, err)

	assert.Equal(
		t,
		codes.Unauthenticated,
		status.Code(err),
	)
}

func TestAuthentication_AttachesVerifiedClaims(
	t *testing.T,
) {
	validator := &interceptorTokenValidator{
		claimsByToken: map[string]*domain.TokenClaims{
			"access-token": {
				UserID:    "user-1",
				Email:     "user@example.com",
				TokenType: domain.TokenTypeAccess,
			},
		},
	}

	interceptor := Authentication(
		validator,
	)

	ctx := metadata.NewIncomingContext(
		context.Background(),
		metadata.Pairs(
			"authorization",
			"Bearer access-token",
		),
	)

	response, err := interceptor(
		ctx,
		nil,
		&grpc.UnaryServerInfo{
			FullMethod: "/user.v1.UserService/GetUser",
		},
		func(
			ctx context.Context,
			_ any,
		) (any, error) {
			claims, err := authcontext.RequireClaims(
				ctx,
			)

			require.NoError(t, err)

			return claims.UserID, nil
		},
	)

	require.NoError(t, err)

	assert.Equal(
		t,
		"user-1",
		response,
	)
}
