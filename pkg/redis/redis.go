package redis

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"sleet0922/graduation_project/internal/config"
)

// Open creates and verifies a Redis client using the caller's context. The
// caller owns the returned client and should close it during shutdown.
func Open(ctx context.Context, cfg *config.ViperConfig) (*goredis.Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	host := strings.TrimSpace(cfg.Redis.Addr)
	addr := net.JoinHostPort(host, strconv.Itoa(cfg.Redis.Port))
	if host == "" || cfg.Redis.Port <= 0 || cfg.Redis.Port > 65535 {
		return nil, fmt.Errorf("redis address %q is invalid", addr)
	}
	if cfg.Redis.DB < 0 || cfg.Redis.DB > 15 {
		return nil, fmt.Errorf("redis database index %d is invalid (must be 0..15)", cfg.Redis.DB)
	}
	client := goredis.NewClient(&goredis.Options{
		Addr:     addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return client, nil
}

// Ping verifies a Redis client and is suitable for readiness probes.
func Ping(ctx context.Context, client *goredis.Client) error {
	if client == nil {
		return fmt.Errorf("redis client is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

// Close closes a Redis client owned by the caller. It is safe to call with a
// nil client and returns the underlying close error with context.
func Close(client *goredis.Client) error {
	if client == nil {
		return nil
	}
	err := client.Close()
	if err != nil {
		return fmt.Errorf("close redis: %w", err)
	}
	return nil
}

// SessionStore captures the Redis operations needed by authentication handlers
// and middleware. It makes those components unit-testable without a live Redis
// server and keeps the backing client explicit for every application instance.
type SessionStore interface {
	SetUserSession(userID uint, sessionID string, ttl time.Duration) (string, error)
	GetUserSession(userID uint) (string, error)
	ExpireUserSession(userID uint, ttl time.Duration) error
	DelUserSession(userID uint) error
	IsSessionValid(userID uint, sessionID string) (bool, error)
	SetRefreshTokenID(userID uint, sessionID, refreshID string, ttl time.Duration) error
	RotateRefreshTokenID(userID uint, sessionID, oldRefreshID, newRefreshID string, ttl time.Duration) error
}

// NewSessionStore binds session operations to one Redis client. A nil client
// is rejected at composition time so authentication cannot be configured with
// a store that will only fail after the first request.
func NewSessionStore(client *goredis.Client) (SessionStore, error) {
	if client == nil {
		return nil, fmt.Errorf("session store: redis client dependency is nil")
	}
	return &clientSessionStore{client: client}, nil
}

type clientSessionStore struct {
	client *goredis.Client
}

func (s *clientSessionStore) requireClient() (*goredis.Client, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("redis client not initialized")
	}
	return s.client, nil
}

func (s *clientSessionStore) SetUserSession(userID uint, sessionID string, ttl time.Duration) (string, error) {
	client, err := s.requireClient()
	if err != nil {
		return "", err
	}
	if sessionID == "" {
		return "", fmt.Errorf("session id is required")
	}
	if err := validateTTL(ttl); err != nil {
		return "", err
	}
	// SET with GET and TTL performs the replacement and expiry atomically. It
	// avoids the race window of GETSET followed by a separate EXPIRE command.
	old, err := client.SetArgs(context.Background(), sessionKey(userID), sessionID, goredis.SetArgs{TTL: ttl, Get: true}).Result()
	if err != nil && err != goredis.Nil {
		return "", err
	}
	return old, nil
}

func (s *clientSessionStore) GetUserSession(userID uint) (string, error) {
	client, err := s.requireClient()
	if err != nil {
		return "", err
	}
	value, err := client.Get(context.Background(), sessionKey(userID)).Result()
	if err == goredis.Nil {
		return "", nil
	}
	return value, err
}

func (s *clientSessionStore) ExpireUserSession(userID uint, ttl time.Duration) error {
	client, err := s.requireClient()
	if err != nil {
		return err
	}
	if err := validateTTL(ttl); err != nil {
		return err
	}
	return client.Expire(context.Background(), sessionKey(userID), ttl).Err()
}

func (s *clientSessionStore) DelUserSession(userID uint) error {
	client, err := s.requireClient()
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := client.Del(ctx, sessionKey(userID)).Err(); err != nil {
		return err
	}
	// SCAN is incremental and does not block Redis the way KEYS does. Delete
	// refresh-token keys in bounded batches while walking the cursor.
	pattern := fmt.Sprintf("%s%d:*", refreshKeyPrefix, userID)
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func (s *clientSessionStore) IsSessionValid(userID uint, sessionID string) (bool, error) {
	if sessionID == "" {
		return false, nil
	}
	current, err := s.GetUserSession(userID)
	if err != nil {
		return false, fmt.Errorf("validate session for user %d: %w", userID, err)
	}
	return current == sessionID, nil
}

func (s *clientSessionStore) SetRefreshTokenID(userID uint, sessionID, refreshID string, ttl time.Duration) error {
	client, err := s.requireClient()
	if err != nil {
		return err
	}
	if sessionID == "" || refreshID == "" {
		return fmt.Errorf("session id and refresh id are required")
	}
	if err := validateTTL(ttl); err != nil {
		return err
	}
	return client.Set(context.Background(), refreshKey(userID, sessionID), refreshID, ttl).Err()
}

func (s *clientSessionStore) RotateRefreshTokenID(userID uint, sessionID, oldRefreshID, newRefreshID string, ttl time.Duration) error {
	client, err := s.requireClient()
	if err != nil {
		return err
	}
	if sessionID == "" || oldRefreshID == "" || newRefreshID == "" {
		return fmt.Errorf("session id and refresh ids are required")
	}
	if err := validateTTL(ttl); err != nil {
		return err
	}
	ctx := context.Background()
	key := refreshKey(userID, sessionID)
	return client.Watch(ctx, func(tx *goredis.Tx) error {
		current, err := tx.Get(ctx, key).Result()
		if err != nil {
			if err == goredis.Nil {
				return fmt.Errorf("refresh token is no longer valid")
			}
			return err
		}
		if current != oldRefreshID {
			return fmt.Errorf("refresh token has already been rotated")
		}
		_, err = tx.TxPipelined(ctx, func(pipe goredis.Pipeliner) error {
			pipe.Set(ctx, key, newRefreshID, ttl)
			return nil
		})
		return err
	}, key)
}

// ---------- 用户会话管理（多设备踢下线）----------

const sessionKeyPrefix = "user:session:"
const refreshKeyPrefix = "user:refresh:"

func sessionKey(userID uint) string {
	return fmt.Sprintf("%s%d", sessionKeyPrefix, userID)
}

func refreshKey(userID uint, sessionID string) string {
	return fmt.Sprintf("%s%d:%s", refreshKeyPrefix, userID, sessionID)
}

func validateTTL(ttl time.Duration) error {
	if ttl <= 0 {
		return fmt.Errorf("session TTL must be positive")
	}
	return nil
}
