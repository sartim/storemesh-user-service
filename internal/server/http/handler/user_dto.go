package handler

import (
	"time"

	"storemesh-user-service/internal/domain"
)

type createUserRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

func (r createUserRequest) toDomain() domain.CreateUserRequest {
	return domain.CreateUserRequest{
		Email:     r.Email,
		Password:  r.Password,
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Phone:     r.Phone,
	}
}

type updateUserRequest struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
}

func (r updateUserRequest) toDomain(
	id string,
) domain.UpdateUserRequest {
	return domain.UpdateUserRequest{
		ID:        id,
		FirstName: r.FirstName,
		LastName:  r.LastName,
		Phone:     r.Phone,
	}
}

type userResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     string    `json:"phone"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserResponse(
	user *domain.User,
) *userResponse {
	if user == nil {
		return nil
	}

	return &userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Phone:     user.Phone,
		IsActive:  user.IsActive(),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
