package tests

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"storemesh-user-service/internal/domain"
	"storemesh-user-service/internal/repository"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

func setupUserRepo(t *testing.T) domain.UserRepository {
	t.Helper()
	db, err := repository.OpenSQLite()
	require.NoError(t, err)
	return repository.NewUserRepository(db)
}

func setupAddressRepo(t *testing.T) (domain.UserRepository, domain.AddressRepository) {
	t.Helper()
	db, err := repository.OpenSQLite()
	require.NoError(t, err)
	return repository.NewUserRepository(db), repository.NewAddressRepository(db)
}

func newTestUser() *domain.User {
	return &domain.User{
		Email:        "test@example.com",
		PasswordHash: "$2a$10$hashedpassword",
		FirstName:    "John",
		LastName:     "Doe",
		Phone:        "+254700000000",
		Status:       domain.StatusActive,
	}
}

// ── Create ────────────────────────────────────────────────────────────────────

func TestUserRepo_Create_Success(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser()
	err := repo.Create(ctx, user)

	require.NoError(t, err)
	assert.NotEmpty(t, user.ID, "ID should be assigned by repository")
	assert.NotZero(t, user.CreatedAt, "CreatedAt should be set")
	assert.NotZero(t, user.UpdatedAt, "UpdatedAt should be set")
}

func TestUserRepo_Create_DefaultsStatusToActive(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser()
	user.Status = "" // no status provided
	err := repo.Create(ctx, user)

	require.NoError(t, err)
	assert.Equal(t, domain.StatusActive, user.Status)
}

func TestUserRepo_Create_DuplicateEmail_ReturnsAlreadyExists(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser()
	require.NoError(t, repo.Create(ctx, user))

	duplicate := newTestUser() // same email
	err := repo.Create(ctx, duplicate)

	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

// ── GetByID ───────────────────────────────────────────────────────────────────

func TestUserRepo_GetByID_Success(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	created := newTestUser()
	require.NoError(t, repo.Create(ctx, created))

	found, err := repo.GetByID(ctx, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Email, found.Email)
	assert.Equal(t, created.FirstName, found.FirstName)
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUserRepo_GetByID_DeletedUser_ReturnsNotFound(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser()
	require.NoError(t, repo.Create(ctx, user))
	require.NoError(t, repo.Delete(ctx, user.ID))

	_, err := repo.GetByID(ctx, user.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── GetByEmail ────────────────────────────────────────────────────────────────

func TestUserRepo_GetByEmail_Success(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	created := newTestUser()
	require.NoError(t, repo.Create(ctx, created))

	found, err := repo.GetByEmail(ctx, created.Email)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Email, found.Email)
}

func TestUserRepo_GetByEmail_NotFound(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	_, err := repo.GetByEmail(ctx, "nobody@example.com")

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── Update ────────────────────────────────────────────────────────────────────

func TestUserRepo_Update_Success(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser()
	require.NoError(t, repo.Create(ctx, user))

	user.FirstName = "Jane"
	user.LastName = "Smith"
	user.Phone = "+254711111111"

	err := repo.Update(ctx, user)
	require.NoError(t, err)

	updated, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "Jane", updated.FirstName)
	assert.Equal(t, "Smith", updated.LastName)
	assert.Equal(t, "+254711111111", updated.Phone)
}

func TestUserRepo_Update_UpdatedAt_Changes(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser()
	require.NoError(t, repo.Create(ctx, user))

	originalUpdatedAt := user.UpdatedAt
	time.Sleep(10 * time.Millisecond) // ensure time advances

	user.FirstName = "Updated"
	require.NoError(t, repo.Update(ctx, user))

	assert.True(t, user.UpdatedAt.After(originalUpdatedAt),
		"UpdatedAt should advance after Update")
}

func TestUserRepo_Update_NonExistentUser_ReturnsNotFound(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := &domain.User{
		ID:        "00000000-0000-0000-0000-000000000000",
		FirstName: "Ghost",
	}
	err := repo.Update(ctx, user)

	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── Delete ────────────────────────────────────────────────────────────────────

func TestUserRepo_Delete_SoftDeletes(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser()
	require.NoError(t, repo.Create(ctx, user))

	err := repo.Delete(ctx, user.ID)
	require.NoError(t, err)

	// should no longer be findable
	_, err = repo.GetByID(ctx, user.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestUserRepo_Delete_NonExistent_ReturnsNotFound(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	err := repo.Delete(ctx, "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// ── List ──────────────────────────────────────────────────────────────────────

func TestUserRepo_List_ReturnsPaginatedResults(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	// create 5 users
	emails := []string{
		"a@example.com", "b@example.com", "c@example.com",
		"d@example.com", "e@example.com",
	}
	for _, email := range emails {
		u := newTestUser()
		u.Email = email
		require.NoError(t, repo.Create(ctx, u))
	}

	req := domain.ListUsersRequest{Page: 1, PerPage: 3}
	users, total, err := repo.List(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, users, 3)
}

func TestUserRepo_List_FilterByStatus(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	active := newTestUser()
	active.Email = "active@example.com"
	require.NoError(t, repo.Create(ctx, active))

	suspended := newTestUser()
	suspended.Email = "suspended@example.com"
	require.NoError(t, repo.Create(ctx, suspended))
	// manually set suspended
	require.NoError(t, repo.Update(ctx, &domain.User{
		ID:        suspended.ID,
		FirstName: suspended.FirstName,
		LastName:  suspended.LastName,
		Status:    domain.StatusSuspended,
	}))

	req := domain.ListUsersRequest{Status: string(domain.StatusActive), Page: 1, PerPage: 20}
	users, total, err := repo.List(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)
	assert.Equal(t, "active@example.com", users[0].Email)
}

func TestUserRepo_List_ExcludesDeletedUsers(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	kept := newTestUser()
	kept.Email = "kept@example.com"
	require.NoError(t, repo.Create(ctx, kept))

	deleted := newTestUser()
	deleted.Email = "deleted@example.com"
	require.NoError(t, repo.Create(ctx, deleted))
	require.NoError(t, repo.Delete(ctx, deleted.ID))

	req := domain.ListUsersRequest{Page: 1, PerPage: 20}
	users, total, err := repo.List(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)
	assert.Equal(t, "kept@example.com", users[0].Email)
}

func TestUserRepo_List_SecondPage(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		u := newTestUser()
		u.Email = "user" + string(rune('0'+i)) + "@example.com"
		require.NoError(t, repo.Create(ctx, u))
	}

	req := domain.ListUsersRequest{Page: 2, PerPage: 2}
	users, total, err := repo.List(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, users, 2)
}
