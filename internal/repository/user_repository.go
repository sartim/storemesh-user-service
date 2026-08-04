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

func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	record, err := toUserModel(user)
	if err != nil {
		return err
	}

	if err := r.db.WithContext(ctx).Create(record).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrAlreadyExists
		}

		return fmt.Errorf("create user: %w", err)
	}

	*user = *toDomainUser(record)

	return nil
}

func (r *userRepository) GetByID(
	ctx context.Context,
	id string,
) (*domain.User, error) {
	parsedID, err := parseUserID(id)
	if err != nil {
		return nil, err
	}

	var record models.User

	err = r.db.
		WithContext(ctx).
		Where("id = ? AND deleted = ?", parsedID, false).
		First(&record).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf("get user by id: %w", err)
	}

	return toDomainUser(&record), nil
}

func (r *userRepository) GetByEmail(
	ctx context.Context,
	email string,
) (*domain.User, error) {
	var record models.User

	err := r.db.
		WithContext(ctx).
		Where("email = ? AND deleted = ?", email, false).
		First(&record).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf("get user by email: %w", err)
	}

	return toDomainUser(&record), nil
}

func (r *userRepository) List(
	ctx context.Context,
	req domain.ListUsersRequest,
) ([]*domain.User, int64, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}

	perPage := req.PerPage
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}

	query := r.db.
		WithContext(ctx).
		Model(&models.User{}).
		Where("deleted = ?", false)

	switch req.Status {
	case "":
	case domain.StatusActive:
		query = query.Where("is_active = ?", true)
	case domain.StatusSuspended:
		query = query.Where("is_active = ?", false)
	default:
		return nil, 0, fmt.Errorf(
			"%w: unsupported user status",
			domain.ErrInvalidInput,
		)
	}

	var total int64

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	var records []models.User

	if err := query.
		Order("created_at DESC").
		Limit(perPage).
		Offset((page - 1) * perPage).
		Find(&records).
		Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	users := make([]*domain.User, 0, len(records))
	for i := range records {
		users = append(users, toDomainUser(&records[i]))
	}

	return users, total, nil
}

func (r *userRepository) Update(
	ctx context.Context,
	user *domain.User,
) error {
	record, err := toUserModel(user)
	if err != nil {
		return err
	}

	now := time.Now().UTC()

	result := r.db.
		WithContext(ctx).
		Model(&models.User{}).
		Where("id = ? AND deleted = ?", record.ID, false).
		Updates(map[string]any{
			"first_name": record.FirstName,
			"last_name":  record.LastName,
			"email":      record.Email,
			"phone":      record.Phone,
			"password":   record.Password,
			"is_active":  record.IsActive,
			"updated_at": now,
		})

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrDuplicatedKey) {
			return domain.ErrAlreadyExists
		}

		return fmt.Errorf("update user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	record.UpdatedAt = now
	*user = *toDomainUser(record)

	return nil
}

func (r *userRepository) Delete(ctx context.Context, id string) error {
	parsedID, err := parseUserID(id)
	if err != nil {
		return err
	}

	result := r.db.
		WithContext(ctx).
		Model(&models.User{}).
		Where("id = ? AND deleted = ?", parsedID, false).
		Updates(map[string]any{
			"deleted":    true,
			"is_active":  false,
			"updated_at": time.Now().UTC(),
		})

	if result.Error != nil {
		return fmt.Errorf("delete user: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	return nil
}
