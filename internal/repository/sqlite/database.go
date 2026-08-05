package sqlite

import (
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"storemesh-user-service/internal/models"
)

// OpenSQLite opens an isolated in-memory SQLite database for repository tests.
func OpenSQLite() (*gorm.DB, error) {
	db, err := gorm.Open(
		sqlite.Open(":memory:"),
		&gorm.Config{
			TranslateError: true,
			Logger: logger.Default.LogMode(
				logger.Silent,
			),
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"open sqlite: %w",
			err,
		)
	}

	// SQLite in-memory databases are connection-local. Restricting the pool
	// to one connection prevents tests from observing an empty database after
	// GORM obtains a different connection from the pool.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf(
			"get sqlite connection pool: %w",
			err,
		)
	}

	sqlDB.SetMaxOpenConns(1)

	if err := db.SetupJoinTable(
		&models.User{},
		"Roles",
		&models.UserRole{},
	); err != nil {
		_ = sqlDB.Close()

		return nil, fmt.Errorf(
			"configure sqlite user role join table: %w",
			err,
		)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Role{},
		&models.UserRole{},
	); err != nil {
		_ = sqlDB.Close()

		return nil, fmt.Errorf(
			"migrate sqlite: %w",
			err,
		)
	}

	roles := []models.Role{
		{
			Name:        "customer",
			Description: "Standard customer",
		},
		{
			Name:        "admin",
			Description: "Administrator",
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
			_ = sqlDB.Close()

			return nil, fmt.Errorf(
				"seed sqlite role %q: %w",
				role.Name,
				err,
			)
		}
	}

	return db, nil
}
