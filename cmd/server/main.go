// Command server runs storemesh-user-service.
//
// It starts TWO servers from ONE process:
//   - gRPC on :50051 — the real traffic path, called by the gqlgen GraphQL server
//   - HTTP on :8080  — health checks (/healthz, /readyz) + REST API
//
// Both servers share the same domain.UserService instance, same DB pool, same
// Redis client. internal/server/grpc and internal/server/http/handler are thin
// translation layers over that one shared service — no business logic duplication.
//
// This is intentionally ONE binary, not two. They are not independently scaled,
// they share all infrastructure, and Kubernetes manages them as a single
// Deployment/Pod. Use separate cmd/ entrypoints only for genuinely separate
// processes — e.g. a future cmd/migrate CLI or cmd/worker background consumer.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"storemesh-user-service/internal/helpers/env"
	"storemesh-user-service/internal/models"
	postgres "storemesh-user-service/internal/repository/postgres"
	"storemesh-user-service/internal/repository/redis"
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
	"storemesh-user-service/internal/middleware"
	"storemesh-user-service/internal/repository"
	grpcserver "storemesh-user-service/internal/server/grpc"
	"storemesh-user-service/internal/server/http/handler"
	"storemesh-user-service/internal/service"
)

func init() {
	gin.ForceConsoleColor()

	// Override logging
	//log.SetPrefix("\u001b[31mERROR: \u001b[0m")
	//log.SetFlags(log.LstdFlags | log.Ldate | log.Lmicroseconds | log.Llongfile)

	// Load environment variables
	// Check if .env file exists
	if _, err := os.Stat(".env"); err == nil {
		env.LoadEnvVars()
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}
	if cfg != nil {
		dsn := cfg.DatabaseURL
		maxOpen := 100
		maxIdle := 100
		connMaxLifetime := time.Duration(10 * time.Minute)

		_, err := postgres.OpenPostgres(dsn, maxOpen, maxIdle, connMaxLifetime)
		if err != nil {
			//
		}
	}

}

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("load config", zap.Error(err))
	}

	tp, err := initTracer(cfg)
	if err != nil {
		log.Warn("tracing unavailable, continuing without it", zap.Error(err))
	} else {
		otel.SetTracerProvider(tp)
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tp.Shutdown(ctx)
		}()
	}

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := postgres.OpenPostgres(cfg.DatabaseURL, cfg.DBMaxOpenConns, cfg.DBMaxIdleConns, cfg.DBConnMaxLifetime)
	if err != nil {
		log.Fatal("open postgres", zap.Error(err))
	}
	log.Info("postgres connected")

	if err := postgres.MigrateAndSeed(db); err != nil {
		log.Fatal("migrate and seed", zap.Error(err))
	}
	log.Info("migrations applied")

	// ── Redis ─────────────────────────────────────────────────────────────────
	rdb, err := redis.NewRedisClient(cfg.RedisURL)
	if err != nil {
		log.Fatal("open redis", zap.Error(err))
	}
	defer rdb.Close()
	log.Info("redis connected")

	userModel := models.User{}
	// ── Repositories + ONE shared service instance ───────────────────────────
	userRepo := repository.NewUserRepository(postgres.DB, userModel)

	svc := service.NewUserService(
		userRepo,
		log,
		cfg.JWTSecret,
		cfg.JWTAccessTokenTTL,
		cfg.JWTRefreshTokenTTL,
	)

	// receives a fatal error from either server so the process exits cleanly
	errCh := make(chan error, 2)

	// ── 1. Start gRPC server ──────────────────────────────────────────────────
	grpcSrv, healthSrv := buildGRPCServer(svc, log)

	go func() {
		lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
		if err != nil {
			errCh <- fmt.Errorf("grpc listen: %w", err)
			return
		}
		log.Info("gRPC server listening", zap.String("port", cfg.GRPCPort))
		if err := grpcSrv.Serve(lis); err != nil {
			errCh <- fmt.Errorf("grpc serve: %w", err)
		}
	}()

	// ── 2. Start HTTP server ──────────────────────────────────────────────────
	httpSrv := buildHTTPServer(cfg, svc, log)

	go func() {
		log.Info("HTTP server listening", zap.String("port", cfg.HTTPPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("http serve: %w", err)
		}
	}()

	// ── 3. Block until shutdown signal or a fatal error from either server ───
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		log.Info("shutdown signal received", zap.String("signal", sig.String()))
	case err := <-errCh:
		log.Error("fatal server error, shutting down", zap.Error(err))
	}

	gracefulShutdown(log, healthSrv, grpcSrv, httpSrv)
}

// ── gRPC server ───────────────────────────────────────────────────────────────

func buildGRPCServer(svc domain.UserService, log *zap.Logger) (*grpc.Server, *health.Server) {
	srv := grpc.NewServer(
		middleware.TracingStatsHandler(),
		grpc.ChainUnaryInterceptor(
			middleware.Recovery(log),
			middleware.Logging(log),
		),
	)

	userv1.RegisterUserServiceServer(
		srv,
		grpcserver.NewUserGRPCServer(svc),
	)

	// health check — Kubernetes readiness/liveness probes and Istio outlier
	// detection both depend on this being registered
	healthSrv := health.NewServer()
	grpc_health_v1.RegisterHealthServer(srv, healthSrv)
	healthSrv.SetServingStatus("user.UserService", grpc_health_v1.HealthCheckResponse_SERVING)

	// reflection enables grpcurl and Kiali to introspect the service without
	// needing the .proto file locally
	reflection.Register(srv)

	return srv, healthSrv
}

// ── HTTP server ───────────────────────────────────────────────────────────────

func buildHTTPServer(cfg *config.Config, svc domain.UserService, log *zap.Logger) *http.Server {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	router.Use(
		middleware.RequestID(),
		//middleware.Recovery(log),
		//middleware.Logging(log),
		middleware.CORS(),
	)

	// unauthenticated health endpoints, kept outside /api/v1 so k8s probes stay simple
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	router.GET("/readyz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	v1 := router.Group("/api/v1")
	handler.NewUserHandler(svc, log).RegisterRoutes(v1)

	return &http.Server{
		Addr:         ":" + cfg.HTTPPort,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// ── Graceful shutdown ─────────────────────────────────────────────────────────

func gracefulShutdown(log *zap.Logger, healthSrv *health.Server, grpcSrv *grpc.Server, httpSrv *http.Server) {
	log.Info("shutting down...")

	// flip health status to NOT_SERVING FIRST — this tells Istio/k8s to stop
	// routing new traffic to this pod while existing requests below still drain
	healthSrv.SetServingStatus("user.UserService", grpc_health_v1.HealthCheckResponse_NOT_SERVING)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		log.Error("http shutdown", zap.Error(err))
	}

	grpcSrv.GracefulStop() // blocks until active RPCs finish, then stops

	log.Info("stopped cleanly")
}

// ── Tracing ───────────────────────────────────────────────────────────────────

func initTracer(cfg *config.Config) (*sdktrace.TracerProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	exp, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(), // Istio sidecar mTLS already secures the transport
	)
	if err != nil {
		return nil, err
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		)),
	), nil
}
