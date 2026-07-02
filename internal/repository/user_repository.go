package repository

import (
	"context"
	"errors"
	"fmt"
	"storemesh-user-service/internal/domain"
	"storemesh-user-service/internal/models"

	"gorm.io/gorm"
)

type BaseRepository struct {
	db    *gorm.DB
	model interface{}
}

func NewUserRepository(db *gorm.DB, model interface{}) *BaseRepository {
	return &BaseRepository{db: db, model: model}
}

// ── Create ────────────────────────────────────────────────────────────────────

func (r *BaseRepository) Create(ctx context.Context, user *models.User) error {
	// WithContext on the transaction so cancellation propagates correctly
	tx := r.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return fmt.Errorf("begin transaction: %w", tx.Error)
	}

	// defer rollback — only takes effect if Commit() was not called
	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			// do NOT re-panic — return the error via the named return instead
			// since this is a plain defer (not a named return func), we log and move on
			// the Rollback ensures the DB is left clean
		}
	}()

	if err := tx.Create(user).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("create user: %w", err) // return, never panic
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func (r *BaseRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("id = ? AND deleted = ?", id, false).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &user, nil
}

// ── GetByEmail ────────────────────────────────────────────────────────────────

func (r *BaseRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.db.WithContext(ctx).
		Where("email = ? AND deleted = ?", email, false).
		First(&user).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &user, nil
}

// ── List ──────────────────────────────────────────────────────────────────────

func (r *BaseRepository) List(ctx context.Context, req domain.ListUsersRequest) ([]*models.User, int64, error) {
	// normalise pagination — never mutate req, work with local vars
	page := req.Page
	if page <= 0 {
		page = 1
	}
	perPage := req.PerPage
	if perPage <= 0 || perPage > 100 {
		perPage = 20
	}
	offset := (page - 1) * perPage

	// build query — WithContext on a fresh scope, never stored back onto r.db
	query := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("deleted = ?", false)

	if req.Status != "" {
		query = query.Where("is_active = ?", req.Status == "active")
	}

	// count first — same filters, no pagination
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	// fetch page
	var users []*models.User
	if err := query.
		Order("created_at DESC").
		Limit(perPage).
		Offset(offset).
		Find(&users).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	return users, total, nil
}

// ── Update ────────────────────────────────────────────────────────────────────

func (r *BaseRepository) Update(ctx context.Context, user *models.User) error {
	result := r.db.WithContext(ctx).Save(user)
	if result.Error != nil {
		return fmt.Errorf("update user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ── Delete ────────────────────────────────────────────────────────────────────

func (r *BaseRepository) Delete(ctx context.Context, id string) error {
	// soft delete — sets deleted=true, preserves record for audit
	result := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ? AND deleted = ?", id, false).
		Updates(map[string]interface{}{
			"deleted":   true,
			"is_active": false,
		})

	if result.Error != nil {
		return fmt.Errorf("delete user: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}
