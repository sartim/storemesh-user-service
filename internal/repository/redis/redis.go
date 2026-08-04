package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"storemesh-user-service/internal/domain"
)

type sessionData struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	ExpiresAt time.Time `json:"expires_at"`
}

// cachedUserProfile deliberately excludes PasswordHash. Cached profile data is
// intended for read-only presentation use and must never contain credentials.
type cachedUserProfile struct {
	ID        string            `json:"id"`
	Email     string            `json:"email"`
	FirstName string            `json:"first_name"`
	LastName  string            `json:"last_name"`
	Phone     string            `json:"phone"`
	Status    domain.UserStatus `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type SessionCache struct {
	rdb *redis.Client
}

func NewSessionCache(rdb *redis.Client) *SessionCache {
	return &SessionCache{rdb: rdb}
}

func (c *SessionCache) StoreSession(
	ctx context.Context,
	tokenHash string,
	claims *domain.TokenClaims,
	ttl time.Duration,
) error {
	data := sessionData{
		UserID:    claims.UserID,
		Email:     claims.Email,
		Roles:     claims.Roles,
		ExpiresAt: time.Now().Add(ttl),
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}

	if err := c.rdb.Set(
		ctx,
		sessionKey(tokenHash),
		payload,
		ttl,
	).Err(); err != nil {
		return fmt.Errorf("store session: %w", err)
	}

	return nil
}

func (c *SessionCache) GetSession(
	ctx context.Context,
	tokenHash string,
) (*domain.TokenClaims, error) {
	payload, err := c.rdb.Get(
		ctx,
		sessionKey(tokenHash),
	).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf("get session: %w", err)
	}

	var data sessionData
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	if time.Now().After(data.ExpiresAt) {
		_ = c.rdb.Del(ctx, sessionKey(tokenHash)).Err()
		return nil, domain.ErrInvalidToken
	}

	return &domain.TokenClaims{
		UserID: data.UserID,
		Email:  data.Email,
		Roles:  data.Roles,
	}, nil
}

func (c *SessionCache) RevokeSession(
	ctx context.Context,
	tokenHash string,
) error {
	if err := c.rdb.Del(
		ctx,
		sessionKey(tokenHash),
	).Err(); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func (c *SessionCache) RevokeAllUserSessions(
	ctx context.Context,
	userID string,
) error {
	keys, err := c.rdb.SMembers(
		ctx,
		userSessionsKey(userID),
	).Result()
	if err != nil {
		return fmt.Errorf("list user sessions: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	pipeline := c.rdb.Pipeline()
	for _, key := range keys {
		pipeline.Del(ctx, key)
	}
	pipeline.Del(ctx, userSessionsKey(userID))

	if _, err := pipeline.Exec(ctx); err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}

	return nil
}

func (c *SessionCache) TrackUserSession(
	ctx context.Context,
	userID string,
	tokenHash string,
	ttl time.Duration,
) error {
	pipeline := c.rdb.Pipeline()
	pipeline.SAdd(
		ctx,
		userSessionsKey(userID),
		sessionKey(tokenHash),
	)
	pipeline.Expire(
		ctx,
		userSessionsKey(userID),
		ttl,
	)

	if _, err := pipeline.Exec(ctx); err != nil {
		return fmt.Errorf("track user session: %w", err)
	}

	return nil
}

func (c *SessionCache) CacheUserProfile(
	ctx context.Context,
	user *domain.User,
	ttl time.Duration,
) error {
	if user == nil {
		return fmt.Errorf(
			"%w: user is required",
			domain.ErrInvalidInput,
		)
	}

	profile := cachedUserProfile{
		ID:        user.ID,
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Phone:     user.Phone,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}

	payload, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("marshal user profile: %w", err)
	}

	if err := c.rdb.Set(
		ctx,
		profileKey(user.ID),
		payload,
		ttl,
	).Err(); err != nil {
		return fmt.Errorf("cache user profile: %w", err)
	}

	return nil
}

func (c *SessionCache) GetCachedUserProfile(
	ctx context.Context,
	userID string,
) (*domain.User, error) {
	payload, err := c.rdb.Get(
		ctx,
		profileKey(userID),
	).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf(
			"get cached user profile: %w",
			err,
		)
	}

	var profile cachedUserProfile
	if err := json.Unmarshal(payload, &profile); err != nil {
		return nil, fmt.Errorf(
			"unmarshal user profile: %w",
			err,
		)
	}

	return &domain.User{
		ID:        profile.ID,
		Email:     profile.Email,
		FirstName: profile.FirstName,
		LastName:  profile.LastName,
		Phone:     profile.Phone,
		Status:    profile.Status,
		CreatedAt: profile.CreatedAt,
		UpdatedAt: profile.UpdatedAt,
	}, nil
}

func (c *SessionCache) InvalidateUserProfile(
	ctx context.Context,
	userID string,
) error {
	if err := c.rdb.Del(
		ctx,
		profileKey(userID),
	).Err(); err != nil {
		return fmt.Errorf(
			"invalidate user profile: %w",
			err,
		)
	}

	return nil
}

func sessionKey(tokenHash string) string {
	return "session:" + tokenHash
}

func userSessionsKey(userID string) string {
	return "user:sessions:" + userID
}

func profileKey(userID string) string {
	return "user:profile:" + userID
}

func NewRedisClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}
