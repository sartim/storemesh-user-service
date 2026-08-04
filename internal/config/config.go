package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRedisURL        = "redis://localhost:6379"
	defaultGRPCPort        = "50051"
	defaultHTTPPort        = "8080"
	defaultEnvironment     = "development"
	defaultJWTAccessTTL    = 30 * time.Minute
	defaultJWTRefreshTTL   = 7 * 24 * time.Hour
	defaultDBMaxOpenConns  = 25
	defaultDBMaxIdleConns  = 5
	defaultDBConnLifetime  = 5 * time.Minute
	defaultOTLPEndpoint    = "localhost:4317"
	defaultServiceName     = "storemesh-user-service"
	defaultServiceVersion  = "1.0.0"
	defaultJWTIssuer       = "storemesh-user-service"
	defaultJWTAudience     = "storemesh-platform"
	minimumJWTSecretLength = 32
)

type Config struct {
	DatabaseURL string
	RedisURL    string

	GRPCPort string
	HTTPPort string

	Environment string

	JWTSecret          string
	JWTIssuer          string
	JWTAudience        string
	JWTAccessTokenTTL  time.Duration
	JWTRefreshTokenTTL time.Duration

	DBMaxOpenConns    int
	DBMaxIdleConns    int
	DBConnMaxLifetime time.Duration

	OTLPEndpoint   string
	ServiceName    string
	ServiceVersion string
}

func Load() (*Config, error) {
	databaseURL, err := requiredString("DATABASE_URL")
	if err != nil {
		return nil, err
	}

	jwtSecret, err := requiredString("JWT_SECRET")
	if err != nil {
		return nil, err
	}

	if len(jwtSecret) < minimumJWTSecretLength {
		return nil, fmt.Errorf(
			"JWT_SECRET must contain at least %d characters",
			minimumJWTSecretLength,
		)
	}

	accessTTL, err := durationValue(
		"JWT_ACCESS_TTL",
		defaultJWTAccessTTL,
	)
	if err != nil {
		return nil, err
	}

	refreshTTL, err := durationValue(
		"JWT_REFRESH_TTL",
		defaultJWTRefreshTTL,
	)
	if err != nil {
		return nil, err
	}

	if accessTTL <= 0 {
		return nil, fmt.Errorf(
			"JWT_ACCESS_TTL must be greater than zero",
		)
	}

	if refreshTTL <= 0 {
		return nil, fmt.Errorf(
			"JWT_REFRESH_TTL must be greater than zero",
		)
	}

	if refreshTTL <= accessTTL {
		return nil, fmt.Errorf(
			"JWT_REFRESH_TTL must be greater than JWT_ACCESS_TTL",
		)
	}

	maxOpenConns, err := intValue(
		"DB_MAX_OPEN_CONNS",
		defaultDBMaxOpenConns,
	)
	if err != nil {
		return nil, err
	}

	maxIdleConns, err := intValue(
		"DB_MAX_IDLE_CONNS",
		defaultDBMaxIdleConns,
	)
	if err != nil {
		return nil, err
	}

	connMaxLifetime, err := durationValue(
		"DB_CONN_MAX_LIFETIME",
		defaultDBConnLifetime,
	)
	if err != nil {
		return nil, err
	}

	if maxOpenConns <= 0 {
		return nil, fmt.Errorf(
			"DB_MAX_OPEN_CONNS must be greater than zero",
		)
	}

	if maxIdleConns < 0 {
		return nil, fmt.Errorf(
			"DB_MAX_IDLE_CONNS cannot be negative",
		)
	}

	if maxIdleConns > maxOpenConns {
		return nil, fmt.Errorf(
			"DB_MAX_IDLE_CONNS cannot exceed DB_MAX_OPEN_CONNS",
		)
	}

	if connMaxLifetime <= 0 {
		return nil, fmt.Errorf(
			"DB_CONN_MAX_LIFETIME must be greater than zero",
		)
	}

	environment := stringValue(
		"ENVIRONMENT",
		defaultEnvironment,
	)

	switch environment {
	case "development", "production", "test":
	default:
		return nil, fmt.Errorf(
			"ENVIRONMENT must be development, production, or test",
		)
	}

	jwtIssuer := stringValue(
		"JWT_ISSUER",
		defaultJWTIssuer,
	)

	jwtAudience := stringValue(
		"JWT_AUDIENCE",
		defaultJWTAudience,
	)

	if strings.TrimSpace(jwtIssuer) == "" {
		return nil, fmt.Errorf(
			"JWT_ISSUER cannot be empty",
		)
	}

	if strings.TrimSpace(jwtAudience) == "" {
		return nil, fmt.Errorf(
			"JWT_AUDIENCE cannot be empty",
		)
	}

	return &Config{
		DatabaseURL: databaseURL,

		RedisURL: stringValue(
			"REDIS_URL",
			defaultRedisURL,
		),

		GRPCPort: stringValue(
			"GRPC_PORT",
			defaultGRPCPort,
		),

		HTTPPort: stringValue(
			"HTTP_PORT",
			defaultHTTPPort,
		),

		Environment: environment,

		JWTSecret:          jwtSecret,
		JWTIssuer:          jwtIssuer,
		JWTAudience:        jwtAudience,
		JWTAccessTokenTTL:  accessTTL,
		JWTRefreshTokenTTL: refreshTTL,

		DBMaxOpenConns:    maxOpenConns,
		DBMaxIdleConns:    maxIdleConns,
		DBConnMaxLifetime: connMaxLifetime,

		OTLPEndpoint: stringValue(
			"OTLP_ENDPOINT",
			defaultOTLPEndpoint,
		),

		ServiceName: stringValue(
			"SERVICE_NAME",
			defaultServiceName,
		),

		ServiceVersion: stringValue(
			"SERVICE_VERSION",
			defaultServiceVersion,
		),
	}, nil
}

func requiredString(
	name string,
) (string, error) {
	value := strings.TrimSpace(
		os.Getenv(name),
	)

	if value == "" {
		return "", fmt.Errorf(
			"%s is required",
			name,
		)
	}

	return value, nil
}

func stringValue(
	name string,
	defaultValue string,
) string {
	value := strings.TrimSpace(
		os.Getenv(name),
	)

	if value == "" {
		return defaultValue
	}

	return value
}

func durationValue(
	name string,
	defaultValue time.Duration,
) (time.Duration, error) {
	rawValue := strings.TrimSpace(
		os.Getenv(name),
	)

	if rawValue == "" {
		return defaultValue, nil
	}

	value, err := time.ParseDuration(rawValue)
	if err != nil {
		return 0, fmt.Errorf(
			"parse %s: %w",
			name,
			err,
		)
	}

	return value, nil
}

func intValue(
	name string,
	defaultValue int,
) (int, error) {
	rawValue := strings.TrimSpace(
		os.Getenv(name),
	)

	if rawValue == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(rawValue)
	if err != nil {
		return 0, fmt.Errorf(
			"parse %s: %w",
			name,
			err,
		)
	}

	return value, nil
}
