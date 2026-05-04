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

func sessionKey(userID uint) string {
	return fmt.Sprintf("%s%d", sessionKeyPrefix, userID)
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

// DelUserSession 删除用户 session（登出时调用）
func DelUserSession(userID uint) error {
	if RedisClient == nil {
		return nil
	}
	return RedisClient.Del(context.Background(), sessionKey(userID)).Err()
}

// IsSessionValid 校验 session_id 是否与 Redis 中一致
func IsSessionValid(userID uint, sessionID string) bool {
	if RedisClient == nil || sessionID == "" {
		return true // 无 Redis 或旧版 token 无 session_id 时放行
	}
	current, err := GetUserSession(userID)
	if err != nil {
		return true // Redis 异常时放行，避免误伤
	}
	return current == "" || current == sessionID
}
