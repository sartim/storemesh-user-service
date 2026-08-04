package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"storemesh-user-service/internal/models"
)

func OpenPostgres(
	dsn string,
	maxOpen int,
	maxIdle int,
	connMaxLifetime time.Duration,
) (*gorm.DB, error) {
	db, err := gorm.Open(
		postgres.Open(dsn),
		&gorm.Config{
			SkipDefaultTransaction: true,
			TranslateError:         true,
			NamingStrategy: schema.NamingStrategy{
				SingularTable: true,
			},
			Logger: logger.Default.LogMode(logger.Warn),
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open postgres: %w",
			err,
		)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf(
			"get postgres connection pool: %w",
			err,
		)
	}

	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()

		return nil, fmt.Errorf(
			"ping postgres: %w",
			err,
		)
	}

	return db, nil
}

// MigrateAndSeed runs the current development migration and seeds default
// roles.
//
// AutoMigrate should be replaced with versioned migrations before production
// rollout.
func MigrateAndSeed(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.User{},
		&models.Role{},
	); err != nil {
		return fmt.Errorf(
			"auto migrate: %w",
			err,
		)
	}

	roles := []models.Role{
		{
			Name:        "customer",
			Description: "Standard customer account",
		},
		{
			Name:        "admin",
			Description: "Platform administrator",
		},
		{
			Name:        "seller",
			Description: "Marketplace seller",
		},
	}

	for i := range roles {
		role := &roles[i]

		if err := db.
			Where(models.Role{Name: role.Name}).
			FirstOrCreate(role).
			Error; err != nil {
			return fmt.Errorf(
				"seed role %q: %w",
				role.Name,
				err,
			)
		}
	}

	return nil
}
