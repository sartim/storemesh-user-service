package postgres

import (
	"log"
	"storemesh-user-service/internal/models"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var DB *gorm.DB

func OpenPostgres(dsn string, maxOpen, maxIdle int, connMaxLifetime time.Duration) (*gorm.DB, error) {
	var err error

	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		Logger: logger.Default.LogMode(logger.Warn),
	})

	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	_, err = applyPoolSettings(DB, maxOpen, maxIdle, connMaxLifetime)

	if err != nil {
		log.Fatal(err)
		return nil, err
	}
	return DB, nil
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
	if err := db.AutoMigrate(&models.User{}, &models.Role{}); err != nil {
		return err
	}
	roles := []models.Role{
		{Name: "customer", Description: "Standard customer account"},
		{Name: "admin", Description: "Platform administrator"},
		{Name: "seller", Description: "Marketplace seller"},
	}
	for _, r := range roles {
		db.Where(models.Role{Name: r.Name}).FirstOrCreate(&r)
	}
	return nil
}
