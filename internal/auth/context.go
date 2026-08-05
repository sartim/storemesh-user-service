package auth

import (
	"context"
	"strings"

	"storemesh-user-service/internal/domain"
)

type contextKey struct{}

// TokenValidator defines the minimum authentication behavior needed by the
// HTTP and gRPC middleware.
//
// domain.UserService satisfies this interface without coupling the middleware
// to the complete service contract.
type TokenValidator interface {
	ValidateToken(
		ctx context.Context,
		token string,
	) (*domain.TokenClaims, error)
}

// WithClaims returns a context containing verified authentication claims.
func WithClaims(
	ctx context.Context,
	claims *domain.TokenClaims,
) context.Context {
	return context.WithValue(
		ctx,
		contextKey{},
		claims,
	)
}

// Claims retrieves verified authentication claims from a request context.
func Claims(
	ctx context.Context,
) (*domain.TokenClaims, bool) {
	claims, ok := ctx.Value(
		contextKey{},
	).(*domain.TokenClaims)

	if !ok || claims == nil {
		return nil, false
	}

	return claims, true
}

// RequireClaims retrieves verified claims or returns ErrUnauthorized.
func RequireClaims(
	ctx context.Context,
) (*domain.TokenClaims, error) {
	claims, ok := Claims(ctx)
	if !ok {
		return nil, domain.ErrUnauthorized
	}

	return claims, nil
}

// ParseBearerToken extracts a token from an HTTP or gRPC Authorization value.
func ParseBearerToken(
	authorization string,
) (string, error) {
	parts := strings.Fields(
		strings.TrimSpace(authorization),
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

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", domain.ErrUnauthorized
	}

	return token, nil
}

// HasAnyRole reports whether the verified claims contain at least one of the
// requested roles.
func HasAnyRole(
	claims *domain.TokenClaims,
	roles ...string,
) bool {
	if claims == nil || len(roles) == 0 {
		return false
	}

	for _, claimRole := range claims.Roles {
		for _, requiredRole := range roles {
			if strings.EqualFold(
				claimRole,
				requiredRole,
			) {
				return true
			}
		}
	}

	return false
}
