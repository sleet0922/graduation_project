package redis

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/pkg/logger"

	goredis "github.com/redis/go-redis/v9"
)

var RedisClient *goredis.Client

func InitRedis(cfg *config.ViperConfig) {
	RedisClient = goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr + ":" + strconv.Itoa(cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	_, err := RedisClient.Ping(context.Background()).Result()
	if err != nil {
		logger.Warn("Redis连接失败", slog.Any("error", err))
	} else {
		logger.Info("Redis连接成功")
	}
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

// SetUserSession 设置用户当前有效 session_id，返回被替换的旧 session_id）
func SetUserSession(userID uint, sessionID string, ttl time.Duration) (string, error) {
	if RedisClient == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	ctx := context.Background()
	key := sessionKey(userID)

	oldSessionID, err := RedisClient.GetSet(ctx, key, sessionID).Result()
	if err != nil && err != goredis.Nil {
		return "", err
	}
	if err := RedisClient.Expire(ctx, key, ttl).Err(); err != nil {
		return "", err
	}
	return oldSessionID, nil
}

// GetUserSession 获取用户当前有效 session_id
func GetUserSession(userID uint) (string, error) {
	if RedisClient == nil {
		return "", fmt.Errorf("redis client not initialized")
	}
	ctx := context.Background()
	sessionID, err := RedisClient.Get(ctx, sessionKey(userID)).Result()
	if err == goredis.Nil {
		return "", nil
	}
	return sessionID, err
}

func ExpireUserSession(userID uint, ttl time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	return RedisClient.Expire(context.Background(), sessionKey(userID), ttl).Err()
}

// DelUserSession 删除用户 session（登出时调用）
func DelUserSession(userID uint) error {
	if RedisClient == nil {
		return nil
	}
	ctx := context.Background()
	if err := RedisClient.Del(ctx, sessionKey(userID)).Err(); err != nil {
		return err
	}
	keys, err := RedisClient.Keys(ctx, fmt.Sprintf("%s%d:*", refreshKeyPrefix, userID)).Result()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return RedisClient.Del(ctx, keys...).Err()
}

// IsSessionValid 校验 session_id 是否与 Redis 中一致
func IsSessionValid(userID uint, sessionID string) bool {
	if RedisClient == nil || sessionID == "" {
		return false
	}
	current, err := GetUserSession(userID)
	if err != nil {
		logger.Warn("校验Redis session失败", slog.Any("user_id", userID), slog.Any("error", err))
		return false
	}
	return current == sessionID
}

func SetRefreshTokenID(userID uint, sessionID, refreshID string, ttl time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	if sessionID == "" || refreshID == "" {
		return fmt.Errorf("session id and refresh id are required")
	}
	return RedisClient.Set(context.Background(), refreshKey(userID, sessionID), refreshID, ttl).Err()
}

func RotateRefreshTokenID(userID uint, sessionID, oldRefreshID, newRefreshID string, ttl time.Duration) error {
	if RedisClient == nil {
		return fmt.Errorf("redis client not initialized")
	}
	if sessionID == "" || oldRefreshID == "" || newRefreshID == "" {
		return fmt.Errorf("session id and refresh ids are required")
	}

	ctx := context.Background()
	key := refreshKey(userID, sessionID)
	return RedisClient.Watch(ctx, func(tx *goredis.Tx) error {
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
