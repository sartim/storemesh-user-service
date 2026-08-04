package service

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"storemesh-user-service/internal/domain"
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
	return &userService{
		users:      users,
		log:        log,
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (s *userService) CreateUser(
	ctx context.Context,
	req domain.CreateUserRequest,
) (*domain.User, error) {
	if req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf(
			"%w: email and password are required",
			domain.ErrInvalidInput,
		)
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Phone:        req.Phone,
		Status:       domain.StatusActive,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}

	s.log.Info(
		"user created",
		zap.String("id", user.ID),
		zap.String("email", user.Email),
	)

	return user, nil
}

func (s *userService) GetUser(
	ctx context.Context,
	id string,
) (*domain.User, error) {
	if id == "" {
		return nil, fmt.Errorf(
			"%w: id is required",
			domain.ErrInvalidInput,
		)
	}

	return s.users.GetByID(ctx, id)
}

func (s *userService) GetUserByEmail(
	ctx context.Context,
	email string,
) (*domain.User, error) {
	if email == "" {
		return nil, fmt.Errorf(
			"%w: email is required",
			domain.ErrInvalidInput,
		)
	}

	return s.users.GetByEmail(ctx, email)
}

func (s *userService) UpdateUser(
	ctx context.Context,
	req domain.UpdateUserRequest,
) (*domain.User, error) {
	if req.ID == "" {
		return nil, fmt.Errorf(
			"%w: id is required",
			domain.ErrInvalidInput,
		)
	}

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

func (s *userService) DeleteUser(
	ctx context.Context,
	id string,
) error {
	if id == "" {
		return fmt.Errorf(
			"%w: id is required",
			domain.ErrInvalidInput,
		)
	}

	return s.users.Delete(ctx, id)
}

func (s *userService) ListUsers(
	ctx context.Context,
	req domain.ListUsersRequest,
) (*domain.ListUsersResponse, error) {
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

	return &domain.ListUsersResponse{
		Users:      users,
		TotalItems: total,
		TotalPages: int(
			math.Ceil(
				float64(total) /
					float64(req.PerPage),
			),
		),
		Page:    req.Page,
		PerPage: req.PerPage,
	}, nil
}

func (s *userService) Authenticate(
	ctx context.Context,
	req domain.AuthRequest,
) (*domain.User, *domain.TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, nil, domain.ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		return nil, nil, domain.ErrInvalidPassword
	}

	pair, err := s.issueTokenPair(user)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"issue tokens: %w",
			err,
		)
	}

	s.log.Info(
		"user authenticated",
		zap.String("user_id", user.ID),
	)

	return user, pair, nil
}

func (s *userService) ValidateToken(
	_ context.Context,
	tokenString string,
) (*domain.TokenClaims, error) {
	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf(
					"unexpected signing method: %v",
					token.Header["alg"],
				)
			}

			return s.secret, nil
		},
	)
	if err != nil || !token.Valid {
		return nil, domain.ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, domain.ErrInvalidToken
	}

	userID, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	rawRoles, _ := claims["roles"].([]any)
	roles := make([]string, 0, len(rawRoles))

	for _, rawRole := range rawRoles {
		role, ok := rawRole.(string)
		if ok {
			roles = append(roles, role)
		}
	}

	return &domain.TokenClaims{
		UserID: userID,
		Email:  email,
		Roles:  roles,
	}, nil
}

func (s *userService) RefreshToken(
	ctx context.Context,
	refreshToken string,
) (*domain.TokenPair, error) {
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

func (s *userService) issueTokenPair(
	user *domain.User,
) (*domain.TokenPair, error) {
	accessToken, err := s.signJWT(
		user,
		s.accessTTL,
	)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.signJWT(
		user,
		s.refreshTTL,
	)
	if err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *userService) signJWT(
	user *domain.User,
	ttl time.Duration,
) (string, error) {
	now := time.Now().UTC()

	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	return token.SignedString(s.secret)
}

var _ domain.UserService = (*userService)(nil)
