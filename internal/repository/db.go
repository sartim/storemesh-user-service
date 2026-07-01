package repository

import (
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenPostgres opens a production PostgreSQL connection via GORM.
func OpenPostgres(dsn string, maxOpen, maxIdle int, connMaxLifetime time.Duration) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	return applyPoolSettings(db, maxOpen, maxIdle, connMaxLifetime)
}

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
	if err := db.AutoMigrate(&gormUser{}, &gormRole{}, &gormAddress{}); err != nil {
		return nil, err
	}
	// seed roles
	roles := []gormRole{
		{ID: "00000000-0000-0000-0000-000000000001", Name: "customer", Description: "Standard customer"},
		{ID: "00000000-0000-0000-0000-000000000002", Name: "admin", Description: "Administrator"},
		{ID: "00000000-0000-0000-0000-000000000003", Name: "seller", Description: "Marketplace seller"},
	}
	for _, r := range roles {
		db.FirstOrCreate(&r, gormRole{Name: r.Name})
	}
	return db, nil
}

func applyPoolSettings(db *gorm.DB, maxOpen, maxIdle int, connMaxLifetime time.Duration) (*gorm.DB, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	return db, nil
}

// MigrateAndSeed runs AutoMigrate and seeds default roles.
// Call once on startup before serving traffic.
func MigrateAndSeed(db *gorm.DB) error {
	if err := db.AutoMigrate(&gormUser{}, &gormRole{}, &gormAddress{}); err != nil {
		return err
	}
	roles := []gormRole{
		{Name: "customer", Description: "Standard customer account"},
		{Name: "admin", Description: "Platform administrator"},
		{Name: "seller", Description: "Marketplace seller"},
	}
	for _, r := range roles {
		db.Where(gormRole{Name: r.Name}).FirstOrCreate(&r)
	}
	return nil
}
