package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Role struct {
	Model

	ID uuid.UUID `json:"id" gorm:"column:id;type:uuid;primaryKey"`

	Name string `json:"name" gorm:"column:name;not null;uniqueIndex:idx_roles_name"`

	Description string `json:"description" gorm:"column:description"`

	Deleted bool `json:"deleted" gorm:"column:deleted;not null;default:false;index"`
}

func (r *Role) BeforeCreate(_ *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}

	return nil
}

func (Role) TableName() string {
	return "role"
}
