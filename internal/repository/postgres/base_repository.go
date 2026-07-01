package postgres

import (
	"context"
	"reflect"
	"storemesh-user-service/internal/domain"
	"storemesh-user-service/internal/models"

	"gorm.io/gorm"
)

type BaseRepository struct {
	db    *gorm.DB
	model interface{}
}

type PaginationParams struct {
	Page  int `form:"page,default=1"`
	Limit int `form:"limit,default=100"`
}

func NewBaseRepository(
	db *gorm.DB, model interface{}) *BaseRepository {
	return &BaseRepository{db, model}
}

func (ctrl *BaseRepository) List(ctx context.Context, req domain.ListUsersRequest) ([]*models.User, int64, error) {
	var params PaginationParams
	var limitParam int
	// check if limit query parameter is provided and parse it
	if limitParam == req.PerPage {
		parsedLimit := limitParam
		params.Limit = parsedLimit
	}

	// use reflection to create a new slice of the correct type
	sliceType := reflect.SliceOf(reflect.TypeOf(ctrl.model))
	records := reflect.New(sliceType).Interface()

	// calculate offset based on page and limit
	offset := (params.Page - 1) * params.Limit

	// pass ctx to database queries
	// use WithContext() method of gorm.DB to pass the context
	ctrl.db = ctrl.db.WithContext(ctx)

	// pass a pointer to the slice to Offset() and Limit() methods
	ctrl.db.Offset(offset).Limit(params.Limit).Find(records)

	// check if records are empty and return 404 if true
	if reflect.ValueOf(records).Elem().Len() == 0 {
		//
		return nil, 0, nil
	}

	// count := int64(reflect.ValueOf(records).Elem().Len())

	// convert slice of user models to slice of interfaces
	var interfaceSlice []interface{}
	for _, record := range reflect.ValueOf(records).Elem().Interface().([]models.User) {
		interfaceSlice = append(interfaceSlice, record)
	}

	// baseURL := ""
	//response := gin.H{
	//	"count": count,
	//	"url":   baseURL,
	//	"data":  interfaceSlice,
	//}
	return nil, 0, nil
}

func (ctrl *BaseRepository) GetByID(ctx context.Context, id string) (*models.User, error) {

	// use reflection to create a new slice of the correct type
	sliceType := reflect.SliceOf(reflect.TypeOf(ctrl.model))
	record := reflect.New(sliceType).Interface()

	if err := ctrl.db.First(record, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return nil, nil
}

func (ctrl *BaseRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {

	// use reflection to create a new slice of the correct type
	sliceType := reflect.SliceOf(reflect.TypeOf(ctrl.model))
	record := reflect.New(sliceType).Interface()

	if err := ctrl.db.First(record, "email = ?", email).Error; err != nil {
		return nil, err
	}
	return nil, nil
}

func (ctrl *BaseRepository) Create(ctx context.Context, user *models.User) error {
	model := reflect.New(reflect.TypeOf(ctrl.model)).Interface()

	tx := ctrl.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := tx.Create(model).Error; err != nil {
		tx.Rollback()
		panic(err)
		return err
	}

	tx.Commit()

	return nil
}

func (ctrl *BaseRepository) Update(ctx context.Context, user *models.User) error {
	//model := reflect.New(reflect.TypeOf(ctrl.model)).Interface()

	ctrl.db.Save(&ctrl.model)
	return nil
}

func (ctrl *BaseRepository) Delete(ctx context.Context, id string) error {
	var record interface{}
	if err := ctrl.db.First(&record, id).Error; err != nil {
		return err
	}
	ctrl.db.Delete(&record)
	return nil
}
