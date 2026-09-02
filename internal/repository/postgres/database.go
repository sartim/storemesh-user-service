package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
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
	if err := db.SetupJoinTable(
		&models.User{},
		"Roles",
		&models.UserRole{},
	); err != nil {
		return fmt.Errorf("configure user role join table: %w", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.UserRole{},
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

// DemoUser contains the minimum credentials needed by the disposable local
// and CI environments. Demo users are only created when explicitly enabled by
// the application entrypoint; production deployments never seed them by
// default.
type DemoUser struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
	Phone     string
	Role      string
}

// SeedDemoUsers creates deterministic demo accounts without replacing an
// existing password. This makes repeated pod starts safe and keeps credentials
// out of the application image.
func SeedDemoUsers(db *gorm.DB, users []DemoUser) error {
	for _, demo := range users {
		if demo.Email == "" || demo.Password == "" || demo.Role == "" {
			return fmt.Errorf("demo user requires email, password, and role")
		}

		var role models.Role
		if err := db.Where(models.Role{Name: demo.Role}).First(&role).Error; err != nil {
			return fmt.Errorf("find demo role %q: %w", demo.Role, err)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(demo.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash demo password for %q: %w", demo.Email, err)
		}

		user := models.User{}
		result := db.Where("email = ?", demo.Email).First(&user)
		if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("find demo user %q: %w", demo.Email, result.Error)
		}
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			user = models.User{
				FirstName: demo.FirstName,
				LastName:  demo.LastName,
				Email:     demo.Email,
				Phone:     demo.Phone,
				Password:  string(hash),
				IsActive:  true,
			}
			if err := db.Create(&user).Error; err != nil {
				return fmt.Errorf("create demo user %q: %w", demo.Email, err)
			}
		}

		if err := db.Model(&user).Association("Roles").Append(&role); err != nil {
			return fmt.Errorf("assign demo role %q to %q: %w", demo.Role, demo.Email, err)
		}
	}

	return nil
}
