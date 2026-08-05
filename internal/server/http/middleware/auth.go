package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	authcontext "storemesh-user-service/internal/auth"
	"storemesh-user-service/internal/domain"
)

// Authentication validates the request bearer token and places verified
// access-token claims into the underlying request context.
func Authentication(
	validator authcontext.TokenValidator,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := authcontext.ParseBearerToken(
			c.GetHeader("Authorization"),
		)
		if err != nil {
			writeUnauthorized(c)
			return
		}

		claims, err := validator.ValidateToken(
			c.Request.Context(),
			token,
		)
		if err != nil {
			writeUnauthorized(c)
			return
		}

		if claims.TokenType != domain.TokenTypeAccess {
			writeUnauthorized(c)
			return
		}

		ctx := authcontext.WithClaims(
			c.Request.Context(),
			claims,
		)

		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequireSelfOrRole allows the request when the authenticated user ID matches
// the named path parameter or the user has one of the supplied roles.
func RequireSelfOrRole(
	userIDParameter string,
	roles ...string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := authcontext.RequireClaims(
			c.Request.Context(),
		)
		if err != nil {
			writeUnauthorized(c)
			return
		}

		requestedUserID := c.Param(
			userIDParameter,
		)

		if requestedUserID == claims.UserID {
			c.Next()
			return
		}

		if authcontext.HasAnyRole(
			claims,
			roles...,
		) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(
			http.StatusForbidden,
			gin.H{
				"error": "forbidden",
			},
		)
	}
}

// RequireRole allows the request when the authenticated user has one of the
// supplied roles.
func RequireRole(
	roles ...string,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, err := authcontext.RequireClaims(c.Request.Context())
		if err != nil {
			writeUnauthorized(c)
			return
		}

		if authcontext.HasAnyRole(claims, roles...) {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(
			http.StatusForbidden,
			gin.H{"error": "forbidden"},
		)
	}
}

func writeUnauthorized(
	c *gin.Context,
) {
	c.AbortWithStatusJSON(
		http.StatusUnauthorized,
		gin.H{
			"error": "invalid or expired access token",
		},
	)
}
