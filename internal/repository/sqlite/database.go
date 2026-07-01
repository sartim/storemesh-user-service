package sqlite

import (
	"storemesh-user-service/internal/models"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenSQLite opens an in-memory SQLite connection — used in unit tests only.
// Tests never need a running Postgres instance.
func OpenSQLite() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}
	// run AutoMigrate so tables exist for tests
	if err := db.AutoMigrate(&models.User{}, &models.Role{}); err != nil {
		return nil, err
	}
	// seed roles
	id := uuid.New()
	roles := []models.Role{
		{ID: id, Name: "customer", Description: "Standard customer"},
		{ID: id, Name: "admin", Description: "Administrator"},
		{ID: id, Name: "seller", Description: "Marketplace seller"},
	}
	for _, r := range roles {
		db.FirstOrCreate(&r, models.Role{Name: r.Name})
	}
	return db, nil
}
