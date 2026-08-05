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

type listUsersQuery struct {
	Status  string `form:"status"`
	Page    int    `form:"page"`
	PerPage int    `form:"per_page"`
}

func (q listUsersQuery) toDomain() domain.ListUsersRequest {
	return domain.ListUsersRequest{
		Status:  domain.UserStatus(q.Status),
		Page:    q.Page,
		PerPage: q.PerPage,
	}
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
	Roles     []string  `json:"roles"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toUserResponse(
	user *domain.User,
) *userResponse {
	if user == nil {
		return nil
	}

	roles := make([]string, 0, len(user.Roles))
	for _, role := range user.Roles {
		roles = append(roles, role.Name)
	}

	return &userResponse{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Phone:     user.Phone,
		IsActive:  user.IsActive(),
		Roles:     roles,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

type roleResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type listUsersResponse struct {
	Users      []*userResponse `json:"users"`
	TotalItems int64           `json:"total_items"`
	TotalPages int             `json:"total_pages"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
}

func toListUsersResponse(result *domain.ListUsersResponse) *listUsersResponse {
	if result == nil {
		return nil
	}

	users := make([]*userResponse, 0, len(result.Users))
	for _, user := range result.Users {
		users = append(users, toUserResponse(user))
	}

	return &listUsersResponse{
		Users:      users,
		TotalItems: result.TotalItems,
		TotalPages: result.TotalPages,
		Page:       result.Page,
		PerPage:    result.PerPage,
	}
}

func toRoleResponses(roles []domain.Role) []roleResponse {
	responses := make([]roleResponse, 0, len(roles))
	for _, role := range roles {
		responses = append(responses, roleResponse{
			ID:          role.ID,
			Name:        role.Name,
			Description: role.Description,
		})
	}

	return responses
}
