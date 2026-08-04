package repository

import (
	"fmt"

	"github.com/google/uuid"

	"storemesh-user-service/internal/domain"
	"storemesh-user-service/internal/models"
)

func toUserModel(user *domain.User) (*models.User, error) {
	if user == nil {
		return nil, fmt.Errorf("%w: user is required", domain.ErrInvalidInput)
	}

	var id uuid.UUID

	if user.ID != "" {
		parsedID, err := uuid.Parse(user.ID)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid user id", domain.ErrInvalidInput)
		}

		id = parsedID
	}

	return &models.User{
		Model: models.Model{
			CreatedAt: user.CreatedAt,
			UpdatedAt: user.UpdatedAt,
		},
		ID:        id,
		Email:     user.Email,
		Password:  user.PasswordHash,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Phone:     user.Phone,
		IsActive:  user.Status == domain.StatusActive,
		Deleted:   user.Status == domain.StatusDeleted,
	}, nil
}

func toDomainUser(user *models.User) *domain.User {
	if user == nil {
		return nil
	}

	status := domain.StatusSuspended

	switch {
	case user.Deleted:
		status = domain.StatusDeleted
	case user.IsActive:
		status = domain.StatusActive
	}

	return &domain.User{
		ID:           user.ID.String(),
		Email:        user.Email,
		PasswordHash: user.Password,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Phone:        user.Phone,
		Status:       status,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func parseUserID(id string) (uuid.UUID, error) {
	parsedID, err := uuid.Parse(id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: invalid user id", domain.ErrInvalidInput)
	}

	return parsedID, nil
}
