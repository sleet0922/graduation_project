package redis

import (
	"context"
	"strconv"

	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/pkg/logger"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
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
		logger.Warn("Redis连接失败", zap.Error(err))
	} else {
		logger.Info("Redis连接成功")
	}
}
