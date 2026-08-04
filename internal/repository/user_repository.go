package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"storemesh-user-service/internal/domain"
	"storemesh-user-service/internal/models"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(
	db *gorm.DB,
) domain.UserRepository {
	return &userRepository{
		db: db,
	}
}

func (r *userRepository) Create(
	ctx context.Context,
	user *models.User,
) error {
	err := r.db.
		WithContext(ctx).
		Create(user).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrAlreadyExists
		}

		return fmt.Errorf(
			"create user: %w",
			err,
		)
	}

	return nil
}

func (r *userRepository) GetByID(
	ctx context.Context,
	id string,
) (*models.User, error) {
	var user models.User

	err := r.db.
		WithContext(ctx).
		Where(
			"id = ? AND deleted = ?",
			id,
			false,
		).
		First(&user).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf(
			"get user by id: %w",
			err,
		)
	}

	return &user, nil
}

func (r *userRepository) GetByEmail(
	ctx context.Context,
	email string,
) (*models.User, error) {
	var user models.User

	err := r.db.
		WithContext(ctx).
		Where(
			"email = ? AND deleted = ?",
			email,
			false,
		).
		First(&user).
		Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf(
			"get user by email: %w",
			err,
		)
	}

	return &user, nil
}

func (r *userRepository) List(
	ctx context.Context,
	req domain.ListUsersRequest,
) ([]*models.User, int64, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	perPage := req.PerPage
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}

	offset := (page - 1) * perPage

	query := r.db.
		WithContext(ctx).
		Model(&models.User{}).
		Where("deleted = ?", false)

	if req.Status != "" {
		query = query.Where(
			"is_active = ?",
			req.Status == string(domain.StatusActive),
		)
	}

	var total int64

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf(
			"count users: %w",
			err,
		)
	}

	var users []*models.User

	if err := query.
		Order("created_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&users).
		Error; err != nil {
		return nil, 0, fmt.Errorf(
			"list users: %w",
			err,
		)
	}

	return users, total, nil
}

func (r *userRepository) Update(
	ctx context.Context,
	user *models.User,
) error {
	now := time.Now().UTC()

	result := r.db.
		WithContext(ctx).
		Model(&models.User{}).
		Where(
			"id = ? AND deleted = ?",
			user.ID,
			false,
		).
		Updates(
			map[string]any{
				"first_name": user.FirstName,
				"last_name":  user.LastName,
				"email":      user.Email,
				"phone":      user.Phone,
				"password":   user.Password,
				"is_active":  user.IsActive,
				"updated_at": now,
			},
		)

	if result.Error != nil {
		if errors.Is(
			result.Error,
			gorm.ErrDuplicatedKey,
		) {
			return domain.ErrAlreadyExists
		}

		return fmt.Errorf(
			"update user: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	user.UpdatedAt = now

	return nil
}

func (r *userRepository) Delete(
	ctx context.Context,
	id string,
) error {
	result := r.db.
		WithContext(ctx).
		Model(&models.User{}).
		Where(
			"id = ? AND deleted = ?",
			id,
			false,
		).
		Updates(
			map[string]any{
				"deleted":    true,
				"is_active":  false,
				"updated_at": time.Now().UTC(),
			},
		)

	if result.Error != nil {
		return fmt.Errorf(
			"delete user: %w",
			result.Error,
		)
	}

	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}
