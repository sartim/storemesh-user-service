package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"storemesh-user-service/internal/domain"
)

const maximumRotationAttempts = 3

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

func NewSessionCache(
	rdb *redis.Client,
) *SessionCache {
	return &SessionCache{
		rdb: rdb,
	}
}

func (c *SessionCache) Create(
	ctx context.Context,
	session *domain.AuthSession,
	ttl time.Duration,
) error {
	if err := validateSession(
		session,
		ttl,
	); err != nil {
		return err
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf(
			"marshal auth session: %w",
			err,
		)
	}

	_, err = c.rdb.TxPipelined(
		ctx,
		func(
			pipeline redis.Pipeliner,
		) error {
			pipeline.Set(
				ctx,
				authSessionKey(session.ID),
				payload,
				ttl,
			)

			pipeline.SAdd(
				ctx,
				userSessionsKey(session.UserID),
				session.ID,
			)

			pipeline.Expire(
				ctx,
				userSessionsKey(session.UserID),
				ttl,
			)

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf(
			"create auth session: %w",
			err,
		)
	}

	return nil
}

func (c *SessionCache) Get(
	ctx context.Context,
	sessionID string,
) (*domain.AuthSession, error) {
	if sessionID == "" {
		return nil, domain.ErrInvalidToken
	}

	payload, err := c.rdb.Get(
		ctx,
		authSessionKey(sessionID),
	).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf(
			"get auth session: %w",
			err,
		)
	}

	session, err := unmarshalSession(payload)
	if err != nil {
		return nil, err
	}

	if time.Now().UTC().After(
		session.ExpiresAt,
	) {
		_ = c.Delete(
			ctx,
			session.ID,
		)

		return nil, domain.ErrInvalidToken
	}

	return session, nil
}

func (c *SessionCache) Rotate(
	ctx context.Context,
	sessionID string,
	currentRefreshTokenID string,
	next *domain.AuthSession,
	ttl time.Duration,
) error {
	if sessionID == "" ||
		currentRefreshTokenID == "" {
		return domain.ErrInvalidToken
	}

	if err := validateSession(
		next,
		ttl,
	); err != nil {
		return err
	}

	if next.ID != sessionID {
		return fmt.Errorf(
			"%w: session id cannot change during rotation",
			domain.ErrInvalidInput,
		)
	}

	nextPayload, err := json.Marshal(next)
	if err != nil {
		return fmt.Errorf(
			"marshal rotated auth session: %w",
			err,
		)
	}

	key := authSessionKey(sessionID)

	var lastTransactionError error

	for attempt := 0; attempt < maximumRotationAttempts; attempt++ {
		err := c.rdb.Watch(
			ctx,
			func(
				transaction *redis.Tx,
			) error {
				currentPayload, err := transaction.Get(
					ctx,
					key,
				).Bytes()
				if err != nil {
					if errors.Is(
						err,
						redis.Nil,
					) {
						return domain.ErrInvalidToken
					}

					return err
				}

				current, err := unmarshalSession(
					currentPayload,
				)
				if err != nil {
					return err
				}

				if current.RefreshTokenID != currentRefreshTokenID {
					return domain.ErrInvalidToken
				}

				if current.UserID != next.UserID {
					return domain.ErrInvalidToken
				}

				_, err = transaction.TxPipelined(
					ctx,
					func(
						pipeline redis.Pipeliner,
					) error {
						pipeline.Set(
							ctx,
							key,
							nextPayload,
							ttl,
						)

						pipeline.SAdd(
							ctx,
							userSessionsKey(next.UserID),
							next.ID,
						)

						pipeline.Expire(
							ctx,
							userSessionsKey(next.UserID),
							ttl,
						)

						return nil
					},
				)

				return err
			},
			key,
		)

		if err == nil {
			return nil
		}

		if errors.Is(
			err,
			domain.ErrInvalidToken,
		) {
			return domain.ErrInvalidToken
		}

		if errors.Is(
			err,
			redis.TxFailedErr,
		) {
			lastTransactionError = err
			continue
		}

		return fmt.Errorf(
			"rotate auth session: %w",
			err,
		)
	}

	return fmt.Errorf(
		"rotate auth session after %d attempts: %w",
		maximumRotationAttempts,
		lastTransactionError,
	)
}

func (c *SessionCache) Delete(
	ctx context.Context,
	sessionID string,
) error {
	if sessionID == "" {
		return nil
	}

	key := authSessionKey(sessionID)

	payload, err := c.rdb.Get(
		ctx,
		key,
	).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil
		}

		return fmt.Errorf(
			"get session before deletion: %w",
			err,
		)
	}

	session, err := unmarshalSession(payload)
	if err != nil {
		return err
	}

	_, err = c.rdb.TxPipelined(
		ctx,
		func(
			pipeline redis.Pipeliner,
		) error {
			pipeline.Del(
				ctx,
				key,
			)

			pipeline.SRem(
				ctx,
				userSessionsKey(session.UserID),
				session.ID,
			)

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf(
			"delete auth session: %w",
			err,
		)
	}

	return nil
}

func (c *SessionCache) DeleteAllForUser(
	ctx context.Context,
	userID string,
) error {
	if userID == "" {
		return fmt.Errorf(
			"%w: user id is required",
			domain.ErrInvalidInput,
		)
	}

	setKey := userSessionsKey(userID)

	sessionIDs, err := c.rdb.SMembers(
		ctx,
		setKey,
	).Result()
	if err != nil {
		return fmt.Errorf(
			"list user auth sessions: %w",
			err,
		)
	}

	keys := make(
		[]string,
		0,
		len(sessionIDs)+1,
	)

	for _, sessionID := range sessionIDs {
		keys = append(
			keys,
			authSessionKey(sessionID),
		)
	}

	keys = append(
		keys,
		setKey,
	)

	if err := c.rdb.Del(
		ctx,
		keys...,
	).Err(); err != nil {
		return fmt.Errorf(
			"delete user auth sessions: %w",
			err,
		)
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
		return fmt.Errorf(
			"marshal user profile: %w",
			err,
		)
	}

	if err := c.rdb.Set(
		ctx,
		profileKey(user.ID),
		payload,
		ttl,
	).Err(); err != nil {
		return fmt.Errorf(
			"cache user profile: %w",
			err,
		)
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
		if errors.Is(err, redis.Nil) {
			return nil, domain.ErrNotFound
		}

		return nil, fmt.Errorf(
			"get cached user profile: %w",
			err,
		)
	}

	var profile cachedUserProfile

	if err := json.Unmarshal(
		payload,
		&profile,
	); err != nil {
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

func NewRedisClient(
	redisURL string,
) (*redis.Client, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf(
			"parse redis url: %w",
			err,
		)
	}

	client := redis.NewClient(options)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()

		return nil, fmt.Errorf(
			"ping redis: %w",
			err,
		)
	}

	return client, nil
}

func validateSession(
	session *domain.AuthSession,
	ttl time.Duration,
) error {
	if session == nil {
		return fmt.Errorf(
			"%w: auth session is required",
			domain.ErrInvalidInput,
		)
	}

	if session.ID == "" {
		return fmt.Errorf(
			"%w: auth session id is required",
			domain.ErrInvalidInput,
		)
	}

	if session.UserID == "" {
		return fmt.Errorf(
			"%w: auth session user id is required",
			domain.ErrInvalidInput,
		)
	}

	if session.AccessTokenID == "" {
		return fmt.Errorf(
			"%w: access token id is required",
			domain.ErrInvalidInput,
		)
	}

	if session.RefreshTokenID == "" {
		return fmt.Errorf(
			"%w: refresh token id is required",
			domain.ErrInvalidInput,
		)
	}

	if ttl <= 0 {
		return fmt.Errorf(
			"%w: session ttl must be positive",
			domain.ErrInvalidInput,
		)
	}

	return nil
}

func unmarshalSession(
	payload []byte,
) (*domain.AuthSession, error) {
	var session domain.AuthSession

	if err := json.Unmarshal(
		payload,
		&session,
	); err != nil {
		return nil, fmt.Errorf(
			"unmarshal auth session: %w",
			err,
		)
	}

	return &session, nil
}

func authSessionKey(
	sessionID string,
) string {
	return "auth:session:" + sessionID
}

func userSessionsKey(
	userID string,
) string {
	return "auth:user-sessions:" + userID
}

func profileKey(
	userID string,
) string {
	return "user:profile:" + userID
}

var _ domain.AuthSessionStore = (*SessionCache)(nil)
