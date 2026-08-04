// Command server runs storemesh-user-service.
//
// It starts two servers from one process:
//   - gRPC on :50051 for internal service-to-service traffic
//   - HTTP on :8080 for health checks and the REST API
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	userv1 "storemesh-user-service/gen/user/v1"
	"storemesh-user-service/internal/config"
	"storemesh-user-service/internal/domain"
	"storemesh-user-service/internal/helpers/env"
	"storemesh-user-service/internal/middleware"
	"storemesh-user-service/internal/repository"
	postgres "storemesh-user-service/internal/repository/postgres"
	redisrepository "storemesh-user-service/internal/repository/redis"
	grpcserver "storemesh-user-service/internal/server/grpc"
	"storemesh-user-service/internal/server/http/handler"
	"storemesh-user-service/internal/service"
)

const grpcUserServiceName = "user.v1.UserService"

func main() {
	if _, err := os.Stat(".env"); err == nil {
		if err := env.LoadEnvVars(); err != nil {
			fmt.Fprintf(
				os.Stderr,
				"load .env: %v\n",
				err,
			)

			os.Exit(1)
		}
	}

	log, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(
			os.Stderr,
			"create logger: %v\n",
			err,
		)

		os.Exit(1)
	}

	defer func() {
		_ = log.Sync()
	}()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(
			"load config",
			zap.Error(err),
		)
	}

	tracerProvider, err := initTracer(cfg)
	if err != nil {
		log.Warn(
			"tracing unavailable, continuing without it",
			zap.Error(err),
		)
	} else {
		otel.SetTracerProvider(
			tracerProvider,
		)

		defer func() {
			ctx, cancel := context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
			defer cancel()

			if err := tracerProvider.Shutdown(
				ctx,
			); err != nil {
				log.Warn(
					"shutdown tracer provider",
					zap.Error(err),
				)
			}
		}()
	}

	db, err := postgres.OpenPostgres(
		cfg.DatabaseURL,
		cfg.DBMaxOpenConns,
		cfg.DBMaxIdleConns,
		cfg.DBConnMaxLifetime,
	)
	if err != nil {
		log.Fatal(
			"open postgres",
			zap.Error(err),
		)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(
			"get postgres connection pool",
			zap.Error(err),
		)
	}

	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Warn(
				"close postgres",
				zap.Error(err),
			)
		}
	}()

	log.Info("postgres connected")

	if err := postgres.MigrateAndSeed(
		db,
	); err != nil {
		log.Fatal(
			"migrate and seed",
			zap.Error(err),
		)
	}

	log.Info("migrations applied")

	redisClient, err := redisrepository.NewRedisClient(
		cfg.RedisURL,
	)
	if err != nil {
		log.Fatal(
			"open redis",
			zap.Error(err),
		)
	}

	defer func() {
		if err := redisClient.Close(); err != nil {
			log.Warn(
				"close redis",
				zap.Error(err),
			)
		}
	}()

	log.Info("redis connected")

	userRepository := repository.NewUserRepository(
		db,
	)

	sessionStore := redisrepository.NewSessionCache(
		redisClient,
	)

	userService := service.NewUserService(
		userRepository,
		sessionStore,
		log,
		cfg.JWTSecret,
		cfg.JWTIssuer,
		cfg.JWTAudience,
		cfg.JWTAccessTokenTTL,
		cfg.JWTRefreshTokenTTL,
	)

	serverErrors := make(
		chan error,
		2,
	)

	grpcServer, healthServer := buildGRPCServer(
		userService,
		log,
	)

	go func() {
		listener, err := net.Listen(
			"tcp",
			":"+cfg.GRPCPort,
		)
		if err != nil {
			serverErrors <- fmt.Errorf(
				"grpc listen: %w",
				err,
			)

			return
		}

		log.Info(
			"gRPC server listening",
			zap.String(
				"port",
				cfg.GRPCPort,
			),
		)

		if err := grpcServer.Serve(
			listener,
		); err != nil {
			serverErrors <- fmt.Errorf(
				"grpc serve: %w",
				err,
			)
		}
	}()

	httpServer := buildHTTPServer(
		cfg,
		userService,
		log,
	)

	go func() {
		log.Info(
			"HTTP server listening",
			zap.String(
				"port",
				cfg.HTTPPort,
			),
		)

		if err := httpServer.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			serverErrors <- fmt.Errorf(
				"http serve: %w",
				err,
			)
		}
	}()

	shutdownSignals := make(
		chan os.Signal,
		1,
	)

	signal.Notify(
		shutdownSignals,
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	defer signal.Stop(
		shutdownSignals,
	)

	select {
	case signalReceived := <-shutdownSignals:
		log.Info(
			"shutdown signal received",
			zap.String(
				"signal",
				signalReceived.String(),
			),
		)

	case serverErr := <-serverErrors:
		log.Error(
			"fatal server error, shutting down",
			zap.Error(serverErr),
		)
	}

	gracefulShutdown(
		log,
		healthServer,
		grpcServer,
		httpServer,
	)
}

func buildGRPCServer(
	userService domain.UserService,
	log *zap.Logger,
) (*grpc.Server, *health.Server) {
	server := grpc.NewServer(
		middleware.TracingStatsHandler(),

		grpc.ChainUnaryInterceptor(
			middleware.Recovery(log),
			middleware.Logging(log),
		),
	)

	userv1.RegisterUserServiceServer(
		server,
		grpcserver.NewUserGRPCServer(
			userService,
		),
	)

	healthServer := health.NewServer()

	grpc_health_v1.RegisterHealthServer(
		server,
		healthServer,
	)

	healthServer.SetServingStatus(
		grpcUserServiceName,
		grpc_health_v1.HealthCheckResponse_SERVING,
	)

	reflection.Register(server)

	return server, healthServer
}

func buildHTTPServer(
	cfg *config.Config,
	userService domain.UserService,
	log *zap.Logger,
) *http.Server {
	if cfg.Environment == "production" {
		gin.SetMode(
			gin.ReleaseMode,
		)
	}

	router := gin.New()

	router.Use(
		middleware.RequestID(),
		gin.Recovery(),
		middleware.CORS(),
	)

	router.GET(
		"/healthz",
		func(
			c *gin.Context,
		) {
			c.JSON(
				http.StatusOK,
				gin.H{
					"status": "ok",
				},
			)
		},
	)

	router.GET(
		"/readyz",
		func(
			c *gin.Context,
		) {
			c.JSON(
				http.StatusOK,
				gin.H{
					"status": "ready",
				},
			)
		},
	)

	apiV1 := router.Group(
		"/api/v1",
	)

	handler.NewUserHandler(
		userService,
		log,
	).RegisterRoutes(apiV1)

	return &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func gracefulShutdown(
	log *zap.Logger,
	healthServer *health.Server,
	grpcServer *grpc.Server,
	httpServer *http.Server,
) {
	log.Info("shutting down")

	healthServer.SetServingStatus(
		grpcUserServiceName,
		grpc_health_v1.HealthCheckResponse_NOT_SERVING,
	)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	if err := httpServer.Shutdown(
		ctx,
	); err != nil {
		log.Error(
			"http shutdown",
			zap.Error(err),
		)
	}

	grpcStopped := make(
		chan struct{},
	)

	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()

	select {
	case <-grpcStopped:
		log.Info(
			"gRPC server stopped gracefully",
		)

	case <-ctx.Done():
		log.Warn(
			"gRPC graceful shutdown timed out; forcing stop",
		)

		grpcServer.Stop()
	}

	log.Info("stopped cleanly")
}

func initTracer(
	cfg *config.Config,
) (*sdktrace.TracerProvider, error) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	exporter, err := otlptracegrpc.New(
		ctx,

		otlptracegrpc.WithEndpoint(
			cfg.OTLPEndpoint,
		),

		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create OTLP trace exporter: %w",
			err,
		)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),

		sdktrace.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,

				semconv.ServiceName(
					cfg.ServiceName,
				),

				semconv.ServiceVersion(
					cfg.ServiceVersion,
				),
			),
		),
	)

	return tracerProvider, nil
}
