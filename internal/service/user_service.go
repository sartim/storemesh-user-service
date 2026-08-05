package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"storemesh-user-service/internal/domain"
)

const (
	minimumPasswordLength = 8
	tokenValidationLeeway = 30 * time.Second
)

type jwtClaims struct {
	Email     string           `json:"email"`
	Roles     []string         `json:"roles,omitempty"`
	TokenType domain.TokenType `json:"token_type"`
	SessionID string           `json:"sid"`

	jwt.RegisteredClaims
}

type userService struct {
	users    domain.UserRepository
	sessions domain.AuthSessionStore
	log      *zap.Logger

	secret   []byte
	issuer   string
	audience string

	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewUserService(
	users domain.UserRepository,
	sessions domain.AuthSessionStore,
	log *zap.Logger,
	secret string,
	issuer string,
	audience string,
	accessTTL time.Duration,
	refreshTTL time.Duration,
) domain.UserService {
	return &userService{
		users:      users,
		sessions:   sessions,
		log:        log,
		secret:     []byte(secret),
		issuer:     issuer,
		audience:   audience,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (s *userService) CreateUser(
	ctx context.Context,
	req domain.CreateUserRequest,
) (*domain.User, error) {
	req.Email = normalizeEmail(req.Email)

	if req.Email == "" {
		return nil, fmt.Errorf(
			"%w: email is required",
			domain.ErrInvalidInput,
		)
	}

	if len(req.Password) < minimumPasswordLength {
		return nil, fmt.Errorf(
			"%w: password must contain at least %d characters",
			domain.ErrInvalidInput,
			minimumPasswordLength,
		)
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(req.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"hash password: %w",
			err,
		)
	}

	user := &domain.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		FirstName:    strings.TrimSpace(req.FirstName),
		LastName:     strings.TrimSpace(req.LastName),
		Phone:        strings.TrimSpace(req.Phone),
		Status:       domain.StatusActive,
		Roles: []domain.Role{
			{Name: domain.RoleCustomer},
		},
	}

	if err := s.users.Create(
		ctx,
		user,
	); err != nil {
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
	id = strings.TrimSpace(id)

	if id == "" {
		return nil, fmt.Errorf(
			"%w: id is required",
			domain.ErrInvalidInput,
		)
	}

	return s.users.GetByID(
		ctx,
		id,
	)
}

func (s *userService) GetUserByEmail(
	ctx context.Context,
	email string,
) (*domain.User, error) {
	email = normalizeEmail(email)

	if email == "" {
		return nil, fmt.Errorf(
			"%w: email is required",
			domain.ErrInvalidInput,
		)
	}

	return s.users.GetByEmail(
		ctx,
		email,
	)
}

func (s *userService) UpdateUser(
	ctx context.Context,
	req domain.UpdateUserRequest,
) (*domain.User, error) {
	req.ID = strings.TrimSpace(req.ID)

	if req.ID == "" {
		return nil, fmt.Errorf(
			"%w: id is required",
			domain.ErrInvalidInput,
		)
	}

	user, err := s.users.GetByID(
		ctx,
		req.ID,
	)
	if err != nil {
		return nil, err
	}

	if value := strings.TrimSpace(
		req.FirstName,
	); value != "" {
		user.FirstName = value
	}

	if value := strings.TrimSpace(
		req.LastName,
	); value != "" {
		user.LastName = value
	}

	if value := strings.TrimSpace(
		req.Phone,
	); value != "" {
		user.Phone = value
	}

	if err := s.users.Update(
		ctx,
		user,
	); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *userService) DeleteUser(
	ctx context.Context,
	id string,
) error {
	id = strings.TrimSpace(id)

	if id == "" {
		return fmt.Errorf(
			"%w: id is required",
			domain.ErrInvalidInput,
		)
	}

	if err := s.sessions.DeleteAllForUser(
		ctx,
		id,
	); err != nil {
		return fmt.Errorf(
			"revoke user sessions: %w",
			err,
		)
	}

	return s.users.Delete(
		ctx,
		id,
	)
}

func (s *userService) ListUsers(
	ctx context.Context,
	req domain.ListUsersRequest,
) (*domain.ListUsersResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}

	if req.PerPage <= 0 ||
		req.PerPage > 100 {
		req.PerPage = 20
	}

	users, total, err := s.users.List(
		ctx,
		req,
	)
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

func (s *userService) ListRoles(
	ctx context.Context,
) ([]domain.Role, error) {
	return s.users.ListRoles(ctx)
}

func (s *userService) GetUserRoles(
	ctx context.Context,
	userID string,
) ([]domain.Role, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf(
			"%w: user id is required",
			domain.ErrInvalidInput,
		)
	}

	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return append([]domain.Role(nil), user.Roles...), nil
}

func (s *userService) AssignRole(
	ctx context.Context,
	userID string,
	roleName string,
) (*domain.User, error) {
	userID, roleName, err := normalizeRoleOperation(userID, roleName)
	if err != nil {
		return nil, err
	}

	// Revoke active sessions before changing authorization data. This fails
	// closed: no previously issued token can retain stale permissions after a
	// successful role change.
	if err := s.sessions.DeleteAllForUser(ctx, userID); err != nil {
		return nil, fmt.Errorf("revoke user sessions: %w", err)
	}

	if err := s.users.AssignRole(ctx, userID, roleName); err != nil {
		return nil, err
	}

	s.log.Info(
		"user role assigned",
		zap.String("user_id", userID),
		zap.String("role", roleName),
	)

	return s.users.GetByID(ctx, userID)
}

func (s *userService) RevokeRole(
	ctx context.Context,
	userID string,
	roleName string,
) (*domain.User, error) {
	userID, roleName, err := normalizeRoleOperation(userID, roleName)
	if err != nil {
		return nil, err
	}

	if err := s.sessions.DeleteAllForUser(ctx, userID); err != nil {
		return nil, fmt.Errorf("revoke user sessions: %w", err)
	}

	if err := s.users.RevokeRole(ctx, userID, roleName); err != nil {
		return nil, err
	}

	s.log.Info(
		"user role revoked",
		zap.String("user_id", userID),
		zap.String("role", roleName),
	)

	return s.users.GetByID(ctx, userID)
}

func (s *userService) Authenticate(
	ctx context.Context,
	req domain.AuthRequest,
) (*domain.User, *domain.TokenPair, error) {
	req.Email = normalizeEmail(req.Email)

	if req.Email == "" ||
		req.Password == "" {
		return nil, nil, domain.ErrInvalidPassword
	}

	user, err := s.users.GetByEmail(
		ctx,
		req.Email,
	)
	if err != nil {
		return nil, nil, domain.ErrInvalidPassword
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(req.Password),
	); err != nil {
		return nil, nil, domain.ErrInvalidPassword
	}

	if !user.IsActive() {
		return nil, nil, domain.ErrForbidden
	}

	sessionID := uuid.NewString()
	createdAt := time.Now().UTC()

	pair, session, err := s.issueTokenPair(
		user,
		sessionID,
		createdAt,
	)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"issue token pair: %w",
			err,
		)
	}

	if err := s.sessions.Create(
		ctx,
		session,
		s.refreshTTL,
	); err != nil {
		return nil, nil, fmt.Errorf(
			"create authentication session: %w",
			err,
		)
	}

	s.log.Info(
		"user authenticated",
		zap.String("user_id", user.ID),
		zap.String("session_id", session.ID),
	)

	return user, pair, nil
}

func (s *userService) ValidateToken(
	ctx context.Context,
	tokenString string,
) (*domain.TokenClaims, error) {
	claims, _, err := s.validateToken(
		ctx,
		tokenString,
		"",
	)
	if err != nil {
		return nil, err
	}

	return claims, nil
}

func (s *userService) RefreshToken(
	ctx context.Context,
	refreshToken string,
) (*domain.TokenPair, error) {
	claims, currentSession, err := s.validateToken(
		ctx,
		refreshToken,
		domain.TokenTypeRefresh,
	)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	user, err := s.users.GetByID(
		ctx,
		claims.UserID,
	)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	if !user.IsActive() {
		return nil, domain.ErrForbidden
	}

	nextPair, nextSession, err := s.issueTokenPair(
		user,
		currentSession.ID,
		currentSession.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"issue rotated token pair: %w",
			err,
		)
	}

	err = s.sessions.Rotate(
		ctx,
		currentSession.ID,
		claims.TokenID,
		nextSession,
		s.refreshTTL,
	)
	if err != nil {
		if errors.Is(
			err,
			domain.ErrInvalidToken,
		) ||
			errors.Is(
				err,
				domain.ErrNotFound,
			) {
			return nil, domain.ErrInvalidToken
		}

		return nil, fmt.Errorf(
			"rotate authentication session: %w",
			err,
		)
	}

	s.log.Info(
		"authentication session rotated",
		zap.String("user_id", user.ID),
		zap.String(
			"session_id",
			currentSession.ID,
		),
	)

	return nextPair, nil
}

func (s *userService) Logout(
	ctx context.Context,
	accessToken string,
) error {
	claims, _, err := s.validateToken(
		ctx,
		accessToken,
		domain.TokenTypeAccess,
	)
	if err != nil {
		return domain.ErrInvalidToken
	}

	if err := s.sessions.Delete(
		ctx,
		claims.SessionID,
	); err != nil {
		return fmt.Errorf(
			"delete authentication session: %w",
			err,
		)
	}

	s.log.Info(
		"user logged out",
		zap.String("user_id", claims.UserID),
		zap.String("session_id", claims.SessionID),
	)

	return nil
}

func (s *userService) LogoutAll(
	ctx context.Context,
	accessToken string,
) error {
	claims, _, err := s.validateToken(
		ctx,
		accessToken,
		domain.TokenTypeAccess,
	)
	if err != nil {
		return domain.ErrInvalidToken
	}

	if err := s.sessions.DeleteAllForUser(
		ctx,
		claims.UserID,
	); err != nil {
		return fmt.Errorf(
			"delete all authentication sessions: %w",
			err,
		)
	}

	s.log.Info(
		"user logged out from all sessions",
		zap.String("user_id", claims.UserID),
	)

	return nil
}

func (s *userService) validateToken(
	ctx context.Context,
	tokenString string,
	expectedType domain.TokenType,
) (*domain.TokenClaims, *domain.AuthSession, error) {
	parsedClaims, err := s.parseToken(
		tokenString,
	)
	if err != nil {
		return nil, nil, domain.ErrInvalidToken
	}

	if expectedType != "" &&
		parsedClaims.TokenType != expectedType {
		return nil, nil, domain.ErrInvalidToken
	}

	session, err := s.sessions.Get(
		ctx,
		parsedClaims.SessionID,
	)
	if err != nil {
		return nil, nil, domain.ErrInvalidToken
	}

	if session.UserID != parsedClaims.Subject {
		return nil, nil, domain.ErrInvalidToken
	}

	if session.Email != parsedClaims.Email {
		return nil, nil, domain.ErrInvalidToken
	}

	if !slices.Equal(session.Roles, parsedClaims.Roles) {
		return nil, nil, domain.ErrInvalidToken
	}

	switch parsedClaims.TokenType {
	case domain.TokenTypeAccess:
		if session.AccessTokenID != parsedClaims.ID {
			return nil, nil, domain.ErrInvalidToken
		}

	case domain.TokenTypeRefresh:
		if session.RefreshTokenID != parsedClaims.ID {
			return nil, nil, domain.ErrInvalidToken
		}

	default:
		return nil, nil, domain.ErrInvalidToken
	}

	if parsedClaims.ExpiresAt == nil {
		return nil, nil, domain.ErrInvalidToken
	}

	return &domain.TokenClaims{
		UserID:    parsedClaims.Subject,
		Email:     parsedClaims.Email,
		Roles:     append([]string(nil), parsedClaims.Roles...),
		TokenType: parsedClaims.TokenType,
		TokenID:   parsedClaims.ID,
		SessionID: parsedClaims.SessionID,
		ExpiresAt: parsedClaims.ExpiresAt.Time,
	}, session, nil
}

func (s *userService) parseToken(
	tokenString string,
) (*jwtClaims, error) {
	tokenString = strings.TrimSpace(
		tokenString,
	)

	if tokenString == "" {
		return nil, domain.ErrInvalidToken
	}

	claims := &jwtClaims{}

	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(
			token *jwt.Token,
		) (any, error) {
			if token.Method.Alg() !=
				jwt.SigningMethodHS256.Alg() {
				return nil, domain.ErrInvalidToken
			}

			return s.secret, nil
		},
		jwt.WithValidMethods(
			[]string{
				jwt.SigningMethodHS256.Alg(),
			},
		),
		jwt.WithIssuer(s.issuer),
		jwt.WithAudience(s.audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(
			tokenValidationLeeway,
		),
	)
	if err != nil ||
		token == nil ||
		!token.Valid {
		return nil, domain.ErrInvalidToken
	}

	if claims.Subject == "" ||
		claims.ID == "" ||
		claims.Email == "" ||
		claims.SessionID == "" {
		return nil, domain.ErrInvalidToken
	}

	switch claims.TokenType {
	case domain.TokenTypeAccess,
		domain.TokenTypeRefresh:
	default:
		return nil, domain.ErrInvalidToken
	}

	return claims, nil
}

func (s *userService) issueTokenPair(
	user *domain.User,
	sessionID string,
	sessionCreatedAt time.Time,
) (*domain.TokenPair, *domain.AuthSession, error) {
	now := time.Now().UTC()

	accessTokenID := uuid.NewString()
	refreshTokenID := uuid.NewString()

	accessExpiresAt := now.Add(
		s.accessTTL,
	)

	refreshExpiresAt := now.Add(
		s.refreshTTL,
	)

	roles := roleNames(user.Roles)

	accessToken, err := s.signJWT(
		user,
		roles,
		domain.TokenTypeAccess,
		sessionID,
		accessTokenID,
		now,
		accessExpiresAt,
	)
	if err != nil {
		return nil, nil, err
	}

	refreshToken, err := s.signJWT(
		user,
		roles,
		domain.TokenTypeRefresh,
		sessionID,
		refreshTokenID,
		now,
		refreshExpiresAt,
	)
	if err != nil {
		return nil, nil, err
	}

	if sessionCreatedAt.IsZero() {
		sessionCreatedAt = now
	}

	return &domain.TokenPair{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
		}, &domain.AuthSession{
			ID:             sessionID,
			UserID:         user.ID,
			Email:          user.Email,
			Roles:          roles,
			AccessTokenID:  accessTokenID,
			RefreshTokenID: refreshTokenID,
			CreatedAt:      sessionCreatedAt,
			ExpiresAt:      refreshExpiresAt,
		}, nil
}

func (s *userService) signJWT(
	user *domain.User,
	roles []string,
	tokenType domain.TokenType,
	sessionID string,
	tokenID string,
	issuedAt time.Time,
	expiresAt time.Time,
) (string, error) {
	claims := jwtClaims{
		Email:     user.Email,
		Roles:     append([]string(nil), roles...),
		TokenType: tokenType,
		SessionID: sessionID,

		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Subject:   user.ID,
			Audience:  jwt.ClaimStrings{s.audience},
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			ID:        tokenID,
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		claims,
	)

	signedToken, err := token.SignedString(
		s.secret,
	)
	if err != nil {
		return "", fmt.Errorf(
			"sign %s token: %w",
			tokenType,
			err,
		)
	}

	return signedToken, nil
}

func normalizeEmail(
	email string,
) string {
	return strings.ToLower(
		strings.TrimSpace(email),
	)
}

func normalizeRoleOperation(
	userID string,
	roleName string,
) (string, string, error) {
	userID = strings.TrimSpace(userID)
	roleName = strings.ToLower(strings.TrimSpace(roleName))

	if userID == "" {
		return "", "", fmt.Errorf(
			"%w: user id is required",
			domain.ErrInvalidInput,
		)
	}

	if roleName == "" {
		return "", "", fmt.Errorf(
			"%w: role name is required",
			domain.ErrInvalidInput,
		)
	}

	return userID, roleName, nil
}

func roleNames(roles []domain.Role) []string {
	names := make([]string, 0, len(roles))
	for _, role := range roles {
		if role.Name != "" {
			names = append(names, role.Name)
		}
	}

	sort.Strings(names)

	return names
}

var _ domain.UserService = (*userService)(nil)
