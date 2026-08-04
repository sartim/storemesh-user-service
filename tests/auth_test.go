package tests

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"storemesh-user-service/internal/domain"
	"storemesh-user-service/internal/service"
)

const (
	testJWTSecret = "storemesh-test-secret-that-is-longer-than-32-characters"

	testJWTIssuer = "storemesh-user-service-test"

	testJWTAudience = "storemesh-test-platform"
)

type fakeAuthUserRepository struct {
	mu    sync.RWMutex
	users map[string]*domain.User
}

func newFakeAuthUserRepository(
	users ...*domain.User,
) *fakeAuthUserRepository {
	repository := &fakeAuthUserRepository{
		users: make(
			map[string]*domain.User,
		),
	}

	for _, user := range users {
		repository.users[user.ID] = cloneDomainUser(
			user,
		)
	}

	return repository
}

func (r *fakeAuthUserRepository) Create(
	_ context.Context,
	user *domain.User,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, existing := range r.users {
		if existing.Email == user.Email {
			return domain.ErrAlreadyExists
		}
	}

	if user.ID == "" {
		user.ID = uuid.NewString()
	}

	now := time.Now().UTC()

	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}

	user.UpdatedAt = now

	r.users[user.ID] = cloneDomainUser(
		user,
	)

	return nil
}

func (r *fakeAuthUserRepository) GetByID(
	_ context.Context,
	id string,
) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, exists := r.users[id]
	if !exists {
		return nil, domain.ErrNotFound
	}

	return cloneDomainUser(user), nil
}

func (r *fakeAuthUserRepository) GetByEmail(
	_ context.Context,
	email string,
) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, user := range r.users {
		if user.Email == email {
			return cloneDomainUser(
				user,
			), nil
		}
	}

	return nil, domain.ErrNotFound
}

func (r *fakeAuthUserRepository) Update(
	_ context.Context,
	user *domain.User,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[user.ID]; !exists {
		return domain.ErrNotFound
	}

	user.UpdatedAt = time.Now().UTC()

	r.users[user.ID] = cloneDomainUser(
		user,
	)

	return nil
}

func (r *fakeAuthUserRepository) Delete(
	_ context.Context,
	id string,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.users[id]; !exists {
		return domain.ErrNotFound
	}

	delete(
		r.users,
		id,
	)

	return nil
}

func (r *fakeAuthUserRepository) List(
	_ context.Context,
	_ domain.ListUsersRequest,
) ([]*domain.User, int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make(
		[]*domain.User,
		0,
		len(r.users),
	)

	for _, user := range r.users {
		users = append(
			users,
			cloneDomainUser(user),
		)
	}

	return users, int64(
		len(users),
	), nil
}

type fakeAuthSessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*domain.AuthSession
}

func newFakeAuthSessionStore() *fakeAuthSessionStore {
	return &fakeAuthSessionStore{
		sessions: make(
			map[string]*domain.AuthSession,
		),
	}
}

func (s *fakeAuthSessionStore) Create(
	_ context.Context,
	session *domain.AuthSession,
	_ time.Duration,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sessions[session.ID] = cloneAuthSession(
		session,
	)

	return nil
}

func (s *fakeAuthSessionStore) Get(
	_ context.Context,
	sessionID string,
) (*domain.AuthSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, exists := s.sessions[sessionID]
	if !exists {
		return nil, domain.ErrNotFound
	}

	if time.Now().UTC().After(
		session.ExpiresAt,
	) {
		return nil, domain.ErrInvalidToken
	}

	return cloneAuthSession(session), nil
}

func (s *fakeAuthSessionStore) Rotate(
	_ context.Context,
	sessionID string,
	currentRefreshTokenID string,
	next *domain.AuthSession,
	_ time.Duration,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.sessions[sessionID]
	if !exists {
		return domain.ErrInvalidToken
	}

	if current.RefreshTokenID != currentRefreshTokenID {
		return domain.ErrInvalidToken
	}

	if current.UserID != next.UserID {
		return domain.ErrInvalidToken
	}

	s.sessions[sessionID] = cloneAuthSession(
		next,
	)

	return nil
}

func (s *fakeAuthSessionStore) Delete(
	_ context.Context,
	sessionID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(
		s.sessions,
		sessionID,
	)

	return nil
}

func (s *fakeAuthSessionStore) DeleteAllForUser(
	_ context.Context,
	userID string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for sessionID, session := range s.sessions {
		if session.UserID == userID {
			delete(
				s.sessions,
				sessionID,
			)
		}
	}

	return nil
}

func TestAuthenticate_IssuesDistinctAccessAndRefreshTokens(
	t *testing.T,
) {
	userService, _, user := newAuthenticationTestService(
		t,
	)

	authenticatedUser, pair, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "correct-password",
		},
	)

	require.NoError(t, err)
	require.NotNil(t, authenticatedUser)
	require.NotNil(t, pair)

	assert.Equal(
		t,
		user.ID,
		authenticatedUser.ID,
	)

	assert.NotEmpty(
		t,
		pair.AccessToken,
	)

	assert.NotEmpty(
		t,
		pair.RefreshToken,
	)

	assert.NotEqual(
		t,
		pair.AccessToken,
		pair.RefreshToken,
	)

	accessClaims, err := userService.ValidateToken(
		context.Background(),
		pair.AccessToken,
	)
	require.NoError(t, err)

	refreshClaims, err := userService.ValidateToken(
		context.Background(),
		pair.RefreshToken,
	)
	require.NoError(t, err)

	assert.Equal(
		t,
		domain.TokenTypeAccess,
		accessClaims.TokenType,
	)

	assert.Equal(
		t,
		domain.TokenTypeRefresh,
		refreshClaims.TokenType,
	)

	assert.Equal(
		t,
		accessClaims.SessionID,
		refreshClaims.SessionID,
	)

	assert.NotEqual(
		t,
		accessClaims.TokenID,
		refreshClaims.TokenID,
	)
}

func TestRefreshToken_RejectsAccessToken(
	t *testing.T,
) {
	userService, _, user := newAuthenticationTestService(
		t,
	)

	_, pair, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "correct-password",
		},
	)
	require.NoError(t, err)

	_, err = userService.RefreshToken(
		context.Background(),
		pair.AccessToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)
}

func TestRefreshToken_RotatesBothTokens(
	t *testing.T,
) {
	userService, _, user := newAuthenticationTestService(
		t,
	)

	_, firstPair, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "correct-password",
		},
	)
	require.NoError(t, err)

	firstAccessClaims, err := userService.ValidateToken(
		context.Background(),
		firstPair.AccessToken,
	)
	require.NoError(t, err)

	secondPair, err := userService.RefreshToken(
		context.Background(),
		firstPair.RefreshToken,
	)
	require.NoError(t, err)

	assert.NotEqual(
		t,
		firstPair.AccessToken,
		secondPair.AccessToken,
	)

	assert.NotEqual(
		t,
		firstPair.RefreshToken,
		secondPair.RefreshToken,
	)

	_, err = userService.ValidateToken(
		context.Background(),
		firstPair.AccessToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)

	_, err = userService.ValidateToken(
		context.Background(),
		firstPair.RefreshToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)

	secondAccessClaims, err := userService.ValidateToken(
		context.Background(),
		secondPair.AccessToken,
	)
	require.NoError(t, err)

	assert.Equal(
		t,
		firstAccessClaims.SessionID,
		secondAccessClaims.SessionID,
	)

	assert.NotEqual(
		t,
		firstAccessClaims.TokenID,
		secondAccessClaims.TokenID,
	)
}

func TestRefreshToken_RejectsRefreshTokenReuse(
	t *testing.T,
) {
	userService, _, user := newAuthenticationTestService(
		t,
	)

	_, pair, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "correct-password",
		},
	)
	require.NoError(t, err)

	_, err = userService.RefreshToken(
		context.Background(),
		pair.RefreshToken,
	)
	require.NoError(t, err)

	_, err = userService.RefreshToken(
		context.Background(),
		pair.RefreshToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)
}

func TestLogout_InvalidatesCurrentSession(
	t *testing.T,
) {
	userService, _, user := newAuthenticationTestService(
		t,
	)

	_, pair, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "correct-password",
		},
	)
	require.NoError(t, err)

	err = userService.Logout(
		context.Background(),
		pair.AccessToken,
	)
	require.NoError(t, err)

	_, err = userService.ValidateToken(
		context.Background(),
		pair.AccessToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)

	_, err = userService.ValidateToken(
		context.Background(),
		pair.RefreshToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)
}

func TestLogoutAll_InvalidatesEveryUserSession(
	t *testing.T,
) {
	userService, _, user := newAuthenticationTestService(
		t,
	)

	_, firstPair, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "correct-password",
		},
	)
	require.NoError(t, err)

	_, secondPair, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "correct-password",
		},
	)
	require.NoError(t, err)

	err = userService.LogoutAll(
		context.Background(),
		firstPair.AccessToken,
	)
	require.NoError(t, err)

	_, err = userService.ValidateToken(
		context.Background(),
		firstPair.AccessToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)

	_, err = userService.ValidateToken(
		context.Background(),
		secondPair.AccessToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)

	_, err = userService.ValidateToken(
		context.Background(),
		secondPair.RefreshToken,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidToken,
	)
}

func TestAuthenticate_RejectsSuspendedUser(
	t *testing.T,
) {
	_, _, user := newAuthenticationTestService(
		t,
	)

	user.Status = domain.StatusSuspended

	repository := newFakeAuthUserRepository(
		user,
	)

	sessions := newFakeAuthSessionStore()

	userService := service.NewUserService(
		repository,
		sessions,
		zap.NewNop(),
		testJWTSecret,
		testJWTIssuer,
		testJWTAudience,
		15*time.Minute,
		24*time.Hour,
	)

	_, _, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "correct-password",
		},
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrForbidden,
	)
}

func TestAuthenticate_RejectsIncorrectPassword(
	t *testing.T,
) {
	userService, _, user := newAuthenticationTestService(
		t,
	)

	_, _, err := userService.Authenticate(
		context.Background(),
		domain.AuthRequest{
			Email:    user.Email,
			Password: "incorrect-password",
		},
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidPassword,
	)
}

func newAuthenticationTestService(
	t *testing.T,
) (
	domain.UserService,
	*fakeAuthSessionStore,
	*domain.User,
) {
	t.Helper()

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte("correct-password"),
		bcrypt.MinCost,
	)
	require.NoError(t, err)

	user := &domain.User{
		ID:           uuid.NewString(),
		Email:        "auth@example.com",
		PasswordHash: string(passwordHash),
		FirstName:    "Auth",
		LastName:     "User",
		Status:       domain.StatusActive,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	repository := newFakeAuthUserRepository(
		user,
	)

	sessions := newFakeAuthSessionStore()

	userService := service.NewUserService(
		repository,
		sessions,
		zap.NewNop(),
		testJWTSecret,
		testJWTIssuer,
		testJWTAudience,
		15*time.Minute,
		24*time.Hour,
	)

	return userService, sessions, user
}

func cloneDomainUser(
	user *domain.User,
) *domain.User {
	if user == nil {
		return nil
	}

	cloned := *user

	return &cloned
}

func cloneAuthSession(
	session *domain.AuthSession,
) *domain.AuthSession {
	if session == nil {
		return nil
	}

	cloned := *session

	cloned.Roles = append(
		[]string(nil),
		session.Roles...,
	)

	return &cloned
}

var _ domain.UserRepository = (*fakeAuthUserRepository)(nil)

var _ domain.AuthSessionStore = (*fakeAuthSessionStore)(nil)
