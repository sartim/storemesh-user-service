package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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

	roleNames := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		roleNames = append(roleNames, role.Name)
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(record).Error; err != nil {
			return err
		}

		roles, err := findRolesByName(tx, roleNames)
		if err != nil {
			return err
		}

		if len(roles) == 0 {
			return nil
		}

		if err := tx.Model(record).Association("Roles").Replace(roles); err != nil {
			return fmt.Errorf("persist user roles: %w", err)
		}

		record.Roles = roles

		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return domain.ErrAlreadyExists
		}

		if errors.Is(err, domain.ErrNotFound) {
			return err
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
		Preload("Roles", "deleted = ?", false).
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
		Preload("Roles", "deleted = ?", false).
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
		Preload("Roles", "deleted = ?", false).
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

func (r *userRepository) ListRoles(
	ctx context.Context,
) ([]domain.Role, error) {
	var records []models.Role

	if err := r.db.
		WithContext(ctx).
		Where("deleted = ?", false).
		Order("name ASC").
		Find(&records).
		Error; err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}

	roles := make([]domain.Role, 0, len(records))
	for i := range records {
		roles = append(roles, toDomainRole(&records[i]))
	}

	return roles, nil
}

func (r *userRepository) AssignRole(
	ctx context.Context,
	userID string,
	roleName string,
) error {
	parsedUserID, err := parseUserID(userID)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureActiveUserExists(tx, parsedUserID); err != nil {
			return err
		}

		role, err := findRoleByName(tx, roleName)
		if err != nil {
			return err
		}

		assignment := models.UserRole{
			UserID: parsedUserID,
			RoleID: role.ID,
		}

		var count int64
		if err := tx.Model(&models.UserRole{}).
			Where("user_id = ? AND role_id = ?", assignment.UserID, assignment.RoleID).
			Count(&count).
			Error; err != nil {
			return fmt.Errorf("check user role assignment: %w", err)
		}

		if count > 0 {
			return domain.ErrAlreadyExists
		}

		if err := tx.Create(&assignment).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return domain.ErrAlreadyExists
			}

			return fmt.Errorf("assign user role: %w", err)
		}

		return nil
	})
}

func (r *userRepository) RevokeRole(
	ctx context.Context,
	userID string,
	roleName string,
) error {
	parsedUserID, err := parseUserID(userID)
	if err != nil {
		return err
	}

	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureActiveUserExists(tx, parsedUserID); err != nil {
			return err
		}

		role, err := findRoleByName(tx, roleName)
		if err != nil {
			return err
		}

		result := tx.
			Where("user_id = ? AND role_id = ?", parsedUserID, role.ID).
			Delete(&models.UserRole{})
		if result.Error != nil {
			return fmt.Errorf("revoke user role: %w", result.Error)
		}

		if result.RowsAffected == 0 {
			return domain.ErrNotFound
		}

		return nil
	})
}

func findRolesByName(
	tx *gorm.DB,
	names []string,
) ([]models.Role, error) {
	if len(names) == 0 {
		return nil, nil
	}

	var roles []models.Role
	if err := tx.
		Where("name IN ? AND deleted = ?", names, false).
		Order("name ASC").
		Find(&roles).
		Error; err != nil {
		return nil, fmt.Errorf("find roles: %w", err)
	}

	if len(roles) != len(names) {
		return nil, domain.ErrNotFound
	}

	return roles, nil
}

func findRoleByName(
	tx *gorm.DB,
	name string,
) (*models.Role, error) {
	var role models.Role

	if err := tx.
		Where("name = ? AND deleted = ?", name, false).
		First(&role).
		Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf("find role: %w", err)
	}

	return &role, nil
}

func ensureActiveUserExists(
	tx *gorm.DB,
	userID uuid.UUID,
) error {
	var count int64

	if err := tx.Model(&models.User{}).
		Where("id = ? AND deleted = ?", userID, false).
		Count(&count).
		Error; err != nil {
		return fmt.Errorf("check user: %w", err)
	}

	if count == 0 {
		return domain.ErrNotFound
	}

	return nil
}
