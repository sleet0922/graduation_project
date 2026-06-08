package main

import (
	"log/slog"
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/db"
	"sleet0922/graduation_project/internal/router"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/redis"
)

func main() {
	cfg := config.InitConfig()
	logger.InitLogger(cfg)
	database := db.InitDB(cfg)
	redis.InitRedis(cfg)
	r := router.InitRouter(database, cfg)
	logger.Info("服务器启动", slog.String("port", cfg.Server.Port))
	err := r.Listen(cfg.Server.Port)
	if err != nil {
		logger.Fatal("启动服务器失败", slog.Any("error", err))
	}
}
