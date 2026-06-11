package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	GRPCPort    string
	HTTPPort    string
	Environment string

	DatabaseURL       string
	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	RedisURL string

	JWTSecret          string
	JWTAccessTokenTTL  time.Duration
	JWTRefreshTokenTTL time.Duration

	OTLPEndpoint   string
	ServiceName    string
	ServiceVersion string
}

func Load() (*Config, error) {
	cfg := &Config{
		GRPCPort:           envOrDefault("GRPC_PORT", "50051"),
		HTTPPort:           envOrDefault("HTTP_PORT", "8080"),
		Environment:        envOrDefault("ENVIRONMENT", "development"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		DBMaxOpenConns:     envInt("DB_MAX_OPEN_CONNS", 25),
		DBMaxIdleConns:     envInt("DB_MAX_IDLE_CONNS", 5),
		DBConnMaxLifetime:  envDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
		RedisURL:           envOrDefault("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		JWTAccessTokenTTL:  envDuration("JWT_ACCESS_TTL", 30*time.Minute),
		JWTRefreshTokenTTL: envDuration("JWT_REFRESH_TTL", 7*24*time.Hour),
		OTLPEndpoint:       envOrDefault("OTLP_ENDPOINT", "localhost:4317"),
		ServiceName:        envOrDefault("SERVICE_NAME", "storemesh-user-service"),
		ServiceVersion:     envOrDefault("SERVICE_VERSION", "1.0.0"),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func DatabaseConfig() string {
	return os.Getenv("DB_URL")
}
