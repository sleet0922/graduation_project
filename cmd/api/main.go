package main

import (
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/db"
	"sleet0922/graduation_project/internal/router"
	"sleet0922/graduation_project/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func main() {
	cfg := config.InitConfig()

	// 初始化日志
	logger.InitLogger(cfg)
	defer logger.Log.Sync()

	gin.SetMode(cfg.Server.Mode)
	database := db.InitDB(cfg)
	db.InitRedis(cfg)

	r := router.InitRouter(database, cfg)

	logger.Info("服务器启动", zap.String("port", cfg.Server.Port))
	err := r.Run(cfg.Server.Port)
	if err != nil {
		logger.Fatal("启动服务器失败", zap.Error(err))
	}
}
