package tests

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storemesh-user-service/internal/domain"
	"storemesh-user-service/internal/repository"
	"storemesh-user-service/internal/repository/sqlite"
)

func setupUserRepo(
	t *testing.T,
) domain.UserRepository {
	t.Helper()

	db, err := sqlite.OpenSQLite()
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	return repository.NewUserRepository(db)
}

func newTestUser(email string) *domain.User {
	return &domain.User{
		Email:        email,
		PasswordHash: "$2a$10$hashedpassword",
		FirstName:    "John",
		LastName:     "Doe",
		Status:       domain.StatusActive,
	}
}

func TestUserRepo_Create_Success(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser("test@example.com")

	err := repo.Create(ctx, user)

	require.NoError(t, err)
	assert.NotEmpty(t, user.ID)

	_, err = uuid.Parse(user.ID)
	require.NoError(t, err)

	assert.False(t, user.CreatedAt.IsZero())
	assert.False(t, user.UpdatedAt.IsZero())
}

func TestUserRepo_Create_DuplicateEmail_ReturnsAlreadyExists(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	require.NoError(
		t,
		repo.Create(
			ctx,
			newTestUser("duplicate@example.com"),
		),
	)

	err := repo.Create(
		ctx,
		newTestUser("duplicate@example.com"),
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrAlreadyExists,
	)
}

func TestUserRepo_GetByID_Success(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	created := newTestUser("get-by-id@example.com")

	require.NoError(
		t,
		repo.Create(ctx, created),
	)

	found, err := repo.GetByID(
		ctx,
		created.ID,
	)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Email, found.Email)
	assert.Equal(t, created.FirstName, found.FirstName)
	assert.Equal(t, created.PasswordHash, found.PasswordHash)
	assert.Equal(t, domain.StatusActive, found.Status)
}

func TestUserRepo_GetByID_NotFound(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	_, err := repo.GetByID(
		ctx,
		uuid.NewString(),
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrNotFound,
	)
}

func TestUserRepo_GetByID_DeletedUser_ReturnsNotFound(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser("deleted-get@example.com")

	require.NoError(
		t,
		repo.Create(ctx, user),
	)

	require.NoError(
		t,
		repo.Delete(ctx, user.ID),
	)

	_, err := repo.GetByID(
		ctx,
		user.ID,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrNotFound,
	)
}

func TestUserRepo_GetByEmail_Success(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	created := newTestUser(
		"get-by-email@example.com",
	)

	require.NoError(
		t,
		repo.Create(ctx, created),
	)

	found, err := repo.GetByEmail(
		ctx,
		created.Email,
	)

	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, created.Email, found.Email)
	assert.Equal(t, created.PasswordHash, found.PasswordHash)
}

func TestUserRepo_GetByEmail_NotFound(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	_, err := repo.GetByEmail(
		ctx,
		"nobody@example.com",
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrNotFound,
	)
}

func TestUserRepo_Update_Success(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser("update@example.com")

	require.NoError(
		t,
		repo.Create(ctx, user),
	)

	user.FirstName = "Jane"
	user.LastName = "Smith"
	user.Phone = "+254711111111"

	err := repo.Update(ctx, user)

	require.NoError(t, err)

	updated, err := repo.GetByID(
		ctx,
		user.ID,
	)

	require.NoError(t, err)
	assert.Equal(t, "Jane", updated.FirstName)
	assert.Equal(t, "Smith", updated.LastName)
	assert.Equal(
		t,
		"+254711111111",
		updated.Phone,
	)
}

func TestUserRepo_Update_UpdatedAt_Changes(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser(
		"updated-at@example.com",
	)

	require.NoError(
		t,
		repo.Create(ctx, user),
	)

	originalUpdatedAt := user.UpdatedAt

	time.Sleep(10 * time.Millisecond)

	user.FirstName = "Updated"

	require.NoError(
		t,
		repo.Update(ctx, user),
	)

	assert.True(
		t,
		user.UpdatedAt.After(originalUpdatedAt),
	)
}

func TestUserRepo_Update_NonExistentUser_ReturnsNotFound(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser("ghost@example.com")
	user.ID = uuid.NewString()

	err := repo.Update(ctx, user)

	assert.ErrorIs(
		t,
		err,
		domain.ErrNotFound,
	)
}

func TestUserRepo_Delete_SoftDeletes(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	user := newTestUser(
		"soft-delete@example.com",
	)

	require.NoError(
		t,
		repo.Create(ctx, user),
	)

	err := repo.Delete(
		ctx,
		user.ID,
	)

	require.NoError(t, err)

	_, err = repo.GetByID(
		ctx,
		user.ID,
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrNotFound,
	)
}

func TestUserRepo_Delete_NonExistent_ReturnsNotFound(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	err := repo.Delete(
		ctx,
		uuid.NewString(),
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrNotFound,
	)
}

func TestUserRepo_List_ReturnsPaginatedResults(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	emails := []string{
		"a@example.com",
		"b@example.com",
		"c@example.com",
		"d@example.com",
		"e@example.com",
	}

	for _, email := range emails {
		require.NoError(
			t,
			repo.Create(
				ctx,
				newTestUser(email),
			),
		)
	}

	users, total, err := repo.List(
		ctx,
		domain.ListUsersRequest{
			Page:    1,
			PerPage: 3,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, users, 3)
}

func TestUserRepo_List_FilterByStatus(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	active := newTestUser(
		"active@example.com",
	)

	require.NoError(
		t,
		repo.Create(ctx, active),
	)

	suspended := newTestUser(
		"suspended@example.com",
	)
	suspended.Status = domain.StatusSuspended

	require.NoError(
		t,
		repo.Create(ctx, suspended),
	)

	users, total, err := repo.List(
		ctx,
		domain.ListUsersRequest{
			Status:  domain.StatusActive,
			Page:    1,
			PerPage: 20,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)
	assert.Equal(
		t,
		"active@example.com",
		users[0].Email,
	)
}

func TestUserRepo_List_RejectsUnsupportedStatus(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	_, _, err := repo.List(
		ctx,
		domain.ListUsersRequest{
			Status:  domain.UserStatus("unknown"),
			Page:    1,
			PerPage: 20,
		},
	)

	assert.ErrorIs(
		t,
		err,
		domain.ErrInvalidInput,
	)
}

func TestUserRepo_List_ExcludesDeletedUsers(
	t *testing.T,
) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	kept := newTestUser(
		"kept@example.com",
	)

	require.NoError(
		t,
		repo.Create(ctx, kept),
	)

	deleted := newTestUser(
		"deleted@example.com",
	)

	require.NoError(
		t,
		repo.Create(ctx, deleted),
	)

	require.NoError(
		t,
		repo.Delete(ctx, deleted.ID),
	)

	users, total, err := repo.List(
		ctx,
		domain.ListUsersRequest{
			Page:    1,
			PerPage: 20,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, users, 1)
	assert.Equal(
		t,
		"kept@example.com",
		users[0].Email,
	)
}

func TestUserRepo_List_SecondPage(t *testing.T) {
	repo := setupUserRepo(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		email := fmt.Sprintf(
			"user%d@example.com",
			i,
		)

		require.NoError(
			t,
			repo.Create(
				ctx,
				newTestUser(email),
			),
		)
	}

	users, total, err := repo.List(
		ctx,
		domain.ListUsersRequest{
			Page:    2,
			PerPage: 2,
		},
	)

	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, users, 2)
}
