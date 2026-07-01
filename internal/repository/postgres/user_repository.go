package postgres

import (
	"gorm.io/gorm"
)

func NewBaseController(
	db *gorm.DB, model interface{}, schema interface{}) *BaseRepository {
	return &BaseRepository{db, model}
}
