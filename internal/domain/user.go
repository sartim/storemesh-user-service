package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrInvalidPassword = errors.New("invalid credentials")
	ErrInvalidToken    = errors.New("invalid or expired token")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
)

type UserStatus string

const (
	StatusActive    UserStatus = "active"
	StatusSuspended UserStatus = "suspended"
	StatusDeleted   UserStatus = "deleted"
)

// User is the framework-independent representation used by the service and
// repository contracts. Persistence and transport layers must map their own
// types to and from this entity.
type User struct {
	ID           string
	Email        string
	PasswordHash string
	FirstName    string
	LastName     string
	Phone        string
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (u *User) IsActive() bool {
	return u != nil && u.Status == StatusActive
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type TokenClaims struct {
	UserID string
	Email  string
	Roles  []string
}

type CreateUserRequest struct {
	Email     string
	Password  string
	FirstName string
	LastName  string
	Phone     string
}

type UpdateUserRequest struct {
	ID        string
	FirstName string
	LastName  string
	Phone     string
}

type AuthRequest struct {
	Email    string
	Password string
}

type ListUsersRequest struct {
	Status  UserStatus
	Page    int
	PerPage int
}

type ListUsersResponse struct {
	Users      []*User
	TotalItems int64
	TotalPages int
	Page       int
	PerPage    int
}

type UserRepository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id string) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, req ListUsersRequest) ([]*User, int64, error)
}

type UserService interface {
	CreateUser(ctx context.Context, req CreateUserRequest) (*User, error)
	GetUser(ctx context.Context, id string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	UpdateUser(ctx context.Context, req UpdateUserRequest) (*User, error)
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context, req ListUsersRequest) (*ListUsersResponse, error)
	Authenticate(ctx context.Context, req AuthRequest) (*User, *TokenPair, error)
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
}
