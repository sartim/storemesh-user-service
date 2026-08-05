package models

import (
	"time"

	"github.com/google/uuid"
)

// UserRole is the explicit join record for persisted user-role assignments.
type UserRole struct {
	UserID    uuid.UUID `gorm:"column:user_id;type:uuid;primaryKey"`
	RoleID    uuid.UUID `gorm:"column:role_id;type:uuid;primaryKey"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (UserRole) TableName() string {
	return "user_role"
}
