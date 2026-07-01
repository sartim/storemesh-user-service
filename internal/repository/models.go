package repository

import (
	"time"

	"gorm.io/gorm"
)

type gormUser struct {
	ID              string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email           string `gorm:"uniqueIndex;not null"`
	PasswordHash    string `gorm:"column:password_hash;not null"`
	FirstName       string `gorm:"not null;default:''"`
	LastName        string `gorm:"not null;default:''"`
	Phone           string `gorm:"not null;default:''"`
	Status          string `gorm:"not null;default:'active'"`
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       gorm.DeletedAt `gorm:"index"`
	Roles           []gormRole     `gorm:"many2many:user_roles;"`
}

func (gormUser) TableName() string { return "users" }

type gormRole struct {
	ID          string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name        string `gorm:"uniqueIndex;not null"`
	Description string `gorm:"not null;default:''"`
}

func (gormRole) TableName() string { return "roles" }

type gormAddress struct {
	ID         string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     string `gorm:"type:uuid;not null;index"`
	Label      string `gorm:"not null;default:'home'"`
	Line1      string `gorm:"not null"`
	Line2      string `gorm:"not null;default:''"`
	City       string `gorm:"not null"`
	State      string `gorm:"not null;default:''"`
	PostalCode string `gorm:"not null"`
	Country    string `gorm:"not null"`
	IsDefault  bool   `gorm:"not null;default:false"`
	CreatedAt  time.Time
}

func (gormAddress) TableName() string { return "addresses" }
