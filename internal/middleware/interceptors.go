package middleware

import (
	"context"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"

	authcontext "storemesh-user-service/internal/auth"
	"storemesh-user-service/internal/domain"
)

// Logging records one structured log entry for every unary gRPC request.
func Logging(
	log *zap.Logger,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		startedAt := time.Now()

		response, err := handler(
			ctx,
			req,
		)

		fields := []zap.Field{
			zap.String(
				"grpc.method",
				info.FullMethod,
			),
			zap.String(
				"grpc.code",
				status.Code(err).String(),
			),
			zap.Duration(
				"grpc.duration",
				time.Since(startedAt),
			),
		}

		if remotePeer, ok := peer.FromContext(ctx); ok {
			fields = append(
				fields,
				zap.String(
					"grpc.peer",
					remotePeer.Addr.String(),
				),
			)
		}

		if claims, ok := authcontext.Claims(ctx); ok {
			fields = append(
				fields,
				zap.String(
					"auth.user_id",
					claims.UserID,
				),
				zap.String(
					"auth.session_id",
					claims.SessionID,
				),
			)
		}

		if err != nil {
			log.Warn(
				"gRPC request completed",
				append(
					fields,
					zap.Error(err),
				)...,
			)
		} else {
			log.Info(
				"gRPC request completed",
				fields...,
			)
		}

		return response, err
	}
}

// Recovery converts panics into a generic internal gRPC error.
func Recovery(
	log *zap.Logger,
) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (
		response any,
		err error,
	) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			log.Error(
				"panic recovered from gRPC request",
				zap.String(
					"grpc.method",
					info.FullMethod,
				),
				zap.Any(
					"panic",
					recovered,
				),
				zap.ByteString(
					"stack",
					debug.Stack(),
				),
			)

			response = nil

			err = status.Error(
				codes.Internal,
				"internal server error",
			)
		}()

		return handler(
			ctx,
			req,
		)
	}
}

// Authentication validates access-token metadata for protected unary gRPC
// operations and propagates verified claims through the request context.
func Authentication(
	validator authcontext.TokenValidator,
	publicMethods ...string,
) grpc.UnaryServerInterceptor {
	allowlist := make(
		map[string]struct{},
		len(publicMethods),
	)

	for _, method := range publicMethods {
		allowlist[method] = struct{}{}
	}

	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		if _, public := allowlist[info.FullMethod]; public {
			return handler(
				ctx,
				req,
			)
		}

		requestMetadata, ok := metadata.FromIncomingContext(
			ctx,
		)
		if !ok {
			return nil, unauthenticatedError()
		}

		authorizationValues := requestMetadata.Get(
			"authorization",
		)
		if len(authorizationValues) == 0 {
			return nil, unauthenticatedError()
		}

		token, err := authcontext.ParseBearerToken(
			authorizationValues[0],
		)
		if err != nil {
			return nil, unauthenticatedError()
		}

		claims, err := validator.ValidateToken(
			ctx,
			token,
		)
		if err != nil {
			return nil, unauthenticatedError()
		}

		if claims.TokenType != domain.TokenTypeAccess {
			return nil, unauthenticatedError()
		}

		authenticatedContext := authcontext.WithClaims(
			ctx,
			claims,
		)

		return handler(
			authenticatedContext,
			req,
		)
	}
}

// TracingStatsHandler instruments gRPC server traffic with OpenTelemetry.
func TracingStatsHandler() stats.Handler {
	return otelgrpc.NewServerHandler()
}

// TracingOption is retained for callers that prefer a grpc.ServerOption.
func TracingOption() grpc.ServerOption {
	return grpc.StatsHandler(
		TracingStatsHandler(),
	)
}

func unauthenticatedError() error {
	return status.Error(
		codes.Unauthenticated,
		"invalid or expired access token",
	)
}
