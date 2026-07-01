package domain

import (
	"context"
	"errors"
	"storemesh-user-service/internal/models"
)

// ── Sentinel errors ───────────────────────────────────────────────────────────

var (
	ErrNotFound        = errors.New("not found")
	ErrAlreadyExists   = errors.New("already exists")
	ErrInvalidPassword = errors.New("invalid credentials")
	ErrInvalidToken    = errors.New("invalid or expired token")
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrForbidden       = errors.New("forbidden")
)

// ── Enums ─────────────────────────────────────────────────────────────────────

type UserStatus string

const (
	StatusActive    UserStatus = "active"
	StatusSuspended UserStatus = "suspended"
	StatusDeleted   UserStatus = "deleted"
)

type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type TokenClaims struct {
	UserID string
	Email  string
	Roles  []string
}

// ── Request / response value objects ─────────────────────────────────────────

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

type CreateAddressRequest struct {
	UserID     string
	Label      string
	Line1      string
	Line2      string
	City       string
	State      string
	PostalCode string
	Country    string
	IsDefault  bool
}

type ListUsersRequest struct {
	Status  string
	Page    int
	PerPage int
}

type ListUsersResponse struct {
	Users      []*models.User
	TotalItems int64
	TotalPages int
	Page       int
	PerPage    int
}

// ── Repository interfaces ─────────────────────────────────────────────────────

type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, req ListUsersRequest) ([]*models.User, int64, error)
}

// ── Service interface — what both HTTP and gRPC handlers depend on ────────────

type UserService interface {
	CreateUser(ctx context.Context, req CreateUserRequest) (*models.User, error)
	GetUser(ctx context.Context, id string) (*models.User, error)
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)
	UpdateUser(ctx context.Context, req UpdateUserRequest) (*models.User, error)
	DeleteUser(ctx context.Context, id string) error
	ListUsers(ctx context.Context, req ListUsersRequest) (*ListUsersResponse, error)
	Authenticate(ctx context.Context, req AuthRequest) (*models.User, *TokenPair, error)
	ValidateToken(ctx context.Context, token string) (*TokenClaims, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenPair, error)
}
