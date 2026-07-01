package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"storemesh-user-service/internal/models"
	"time"

	"storemesh-user-service/internal/domain"

	"github.com/redis/go-redis/v9"
)

// sessionData is what we store in Redis — not the domain.User directly,
// just the fields needed to reconstruct a TokenClaims without a DB round-trip.
type sessionData struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Roles     []string  `json:"roles"`
	ExpiresAt time.Time `json:"expires_at"`
}

type SessionCache struct {
	rdb *redis.Client
}

func NewSessionCache(rdb *redis.Client) *SessionCache {
	return &SessionCache{rdb: rdb}
}

// StoreSession persists a validated token's claims so ValidateToken
// can be answered from Redis without a DB hit on every gRPC call.
func (c *SessionCache) StoreSession(ctx context.Context, tokenHash string, claims *domain.TokenClaims, ttl time.Duration) error {
	data := sessionData{
		UserID:    claims.UserID,
		Email:     claims.Email,
		Roles:     claims.Roles,
		ExpiresAt: time.Now().Add(ttl),
	}
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	return c.rdb.Set(ctx, sessionKey(tokenHash), b, ttl).Err()
}

// GetSession retrieves cached claims by token hash.
// Returns domain.ErrNotFound when the key has expired or never existed.
func (c *SessionCache) GetSession(ctx context.Context, tokenHash string) (*domain.TokenClaims, error) {
	b, err := c.rdb.Get(ctx, sessionKey(tokenHash)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrNotFound
		}
		return nil, fmt.Errorf("get session: %w", err)
	}

	var data sessionData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, fmt.Errorf("unmarshal session: %w", err)
	}

	if time.Now().After(data.ExpiresAt) {
		_ = c.rdb.Del(ctx, sessionKey(tokenHash))
		return nil, domain.ErrInvalidToken
	}

	return &domain.TokenClaims{
		UserID: data.UserID,
		Email:  data.Email,
		Roles:  data.Roles,
	}, nil
}

// RevokeSession deletes a session — used on logout or forced sign-out.
func (c *SessionCache) RevokeSession(ctx context.Context, tokenHash string) error {
	return c.rdb.Del(ctx, sessionKey(tokenHash)).Err()
}

// RevokeAllUserSessions deletes all active sessions for a user.
// Uses a user→sessions index set to track keys.
func (c *SessionCache) RevokeAllUserSessions(ctx context.Context, userID string) error {
	keys, err := c.rdb.SMembers(ctx, userSessionsKey(userID)).Result()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	pipe := c.rdb.Pipeline()
	for _, k := range keys {
		pipe.Del(ctx, k)
	}
	pipe.Del(ctx, userSessionsKey(userID))
	_, err = pipe.Exec(ctx)
	return err
}

// TrackUserSession adds a session key to the user's session index.
// Call after StoreSession so RevokeAllUserSessions can find it.
func (c *SessionCache) TrackUserSession(ctx context.Context, userID, tokenHash string, ttl time.Duration) error {
	pipe := c.rdb.Pipeline()
	pipe.SAdd(ctx, userSessionsKey(userID), sessionKey(tokenHash))
	pipe.Expire(ctx, userSessionsKey(userID), ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// ── Profile cache ─────────────────────────────────────────────────────────────

// CacheUserProfile stores a lightweight user profile for fast reads
// by the gqlgen GraphQL server — avoids a DB call per resolver field.
func (c *SessionCache) CacheUserProfile(ctx context.Context, user *models.User, ttl time.Duration) error {
	b, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, profileKey(user.ID.String()), b, ttl).Err()
}

// GetCachedUserProfile retrieves a cached user profile.
func (c *SessionCache) GetCachedUserProfile(ctx context.Context, userID string) (*models.User, error) {
	b, err := c.rdb.Get(ctx, profileKey(userID)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	var user models.User
	return &user, json.Unmarshal(b, &user)
}

// InvalidateUserProfile removes the cached profile — call after Update or Delete.
func (c *SessionCache) InvalidateUserProfile(ctx context.Context, userID string) error {
	return c.rdb.Del(ctx, profileKey(userID)).Err()
}

// ── Key helpers ───────────────────────────────────────────────────────────────

func sessionKey(tokenHash string) string   { return "session:" + tokenHash }
func userSessionsKey(userID string) string { return "user:sessions:" + userID }
func profileKey(userID string) string      { return "user:profile:" + userID }

// ── Connection helper ─────────────────────────────────────────────────────────

// NewRedisClient parses a Redis URL and returns a connected client.
func NewRedisClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	rdb := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return rdb, nil
}
