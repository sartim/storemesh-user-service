package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	Model

	ID uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`

	FirstName string `json:"first_name" gorm:"column:first_name;not null"`
	LastName  string `json:"last_name" gorm:"column:last_name;not null"`

	Email string `json:"email" gorm:"column:email;not null;uniqueIndex:idx_users_email"`

	Phone string `json:"phone" gorm:"column:phone;uniqueIndex:idx_users_phone,where:phone <> ''"`

	Password string `json:"-" gorm:"column:password;not null"`

	IsActive bool `json:"is_active" gorm:"column:is_active;not null;default:false"`

	Deleted bool `json:"deleted" gorm:"column:deleted;not null;default:false;index"`
}

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}

	return nil
}

func (User) TableName() string {
	return "user"
}
