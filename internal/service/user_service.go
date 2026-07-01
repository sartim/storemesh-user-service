package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"storemesh-user-service/internal/models"
	"time"

	"storemesh-user-service/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

type userService struct {
	users      domain.UserRepository
	log        *zap.Logger
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewUserService(
	users domain.UserRepository,
	log *zap.Logger,
	secret string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) domain.UserService {
	if secret == "" {
		secret = os.Getenv("JWT_SECRET")
	}
	return &userService{
		users:      users,
		log:        log,
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// ── CreateUser ────────────────────────────────────────────────────────────────
// Direct equivalent of your controller's Create() — hashes password, saves user.

func (s *userService) CreateUser(ctx context.Context, req domain.CreateUserRequest) (*models.User, error) {
	if req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("%w: email and password are required", domain.ErrInvalidInput)
	}

	// same as your crypto.HashPassword(input.Password)
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		Email:     req.Email,
		Password:  string(hash),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		IsActive:  true, //
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	s.log.Info("user created", zap.String("id", user.ID.String()), zap.String("email", user.Email))
	return user, nil
}

// ── GetUser ───────────────────────────────────────────────────────────────────

func (s *userService) GetUser(ctx context.Context, id string) (*models.User, error) {
	if id == "" {
		return nil, fmt.Errorf("%w: id is required", domain.ErrInvalidInput)
	}
	return s.users.GetByID(ctx, id)
}

func (s *userService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	if email == "" {
		return nil, fmt.Errorf("%w: email is required", domain.ErrInvalidInput)
	}
	return s.users.GetByEmail(ctx, email)
}

// ── UpdateUser ────────────────────────────────────────────────────────────────

func (s *userService) UpdateUser(ctx context.Context, req domain.UpdateUserRequest) (*models.User, error) {
	user, err := s.users.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if err := s.users.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// ── DeleteUser ────────────────────────────────────────────────────────────────

func (s *userService) DeleteUser(ctx context.Context, id string) error {
	return s.users.Delete(ctx, id)
}

// ── ListUsers ─────────────────────────────────────────────────────────────────

func (s *userService) ListUsers(ctx context.Context, req domain.ListUsersRequest) (*domain.ListUsersResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PerPage <= 0 || req.PerPage > 100 {
		req.PerPage = 20
	}

	users, total, err := s.users.List(ctx, req)
	if err != nil {
		return nil, err
	}

	pages := int(math.Ceil(float64(total) / float64(req.PerPage)))
	return &domain.ListUsersResponse{
		Users:      users,
		TotalItems: total,
		TotalPages: pages,
		Page:       req.Page,
		PerPage:    req.PerPage,
	}, nil
}

// ── Authenticate ──────────────────────────────────────────────────────────────
// Equivalent of a login endpoint — same bcrypt check your controller implies.

func (s *userService) Authenticate(ctx context.Context, req domain.AuthRequest) (*models.User, *domain.TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		// return same error whether user missing or wrong password — prevents enumeration
		return nil, nil, domain.ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, nil, domain.ErrInvalidPassword
	}

	pair, err := s.issueTokenPair(user)
	if err != nil {
		return nil, nil, fmt.Errorf("issue tokens: %w", err)
	}

	s.log.Info("user authenticated", zap.String("user_id", user.ID.String()))
	return user, pair, nil
}

// ── ValidateToken ─────────────────────────────────────────────────────────────

func (s *userService) ValidateToken(ctx context.Context, tokenStr string) (*domain.TokenClaims, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !token.Valid {
		return nil, domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrInvalidToken
	}

	userID, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)
	rawRoles, _ := claims["roles"].([]interface{})
	roles := make([]string, len(rawRoles))
	for i, r := range rawRoles {
		roles[i], _ = r.(string)
	}

	return &domain.TokenClaims{UserID: userID, Email: email, Roles: roles}, nil
}

// ── RefreshToken ──────────────────────────────────────────────────────────────

func (s *userService) RefreshToken(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	claims, err := s.ValidateToken(ctx, refreshToken)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	user, err := s.users.GetByID(ctx, claims.UserID)
	if err != nil {
		return nil, err
	}

	return s.issueTokenPair(user)
}

// ── JWT helpers ───────────────────────────────────────────────────────────────

func (s *userService) issueTokenPair(user *models.User) (*domain.TokenPair, error) {
	access, err := s.signJWT(user, s.accessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := s.signJWT(user, s.refreshTTL)
	if err != nil {
		return nil, err
	}
	return &domain.TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func (s *userService) signJWT(user *models.User, ttl time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(ttl).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ensure compile-time interface satisfaction
var _ domain.UserService = (*userService)(nil)

// ── helpers ───────────────────────────────────────────────────────────────────

func isNotFound(err error) bool { return errors.Is(err, domain.ErrNotFound) }
