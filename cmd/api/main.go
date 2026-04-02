package main

import (
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/db"
	"sleet0922/graduation_project/internal/router"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/redis"

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
	redis.InitRedis(cfg)

	r := router.InitRouter(database, cfg)

	logger.Info("服务器启动", zap.String("port", cfg.Server.Port))
	var err error
	if cfg.Server.Mode == "release" {
		err = r.RunTLS(
			cfg.Server.Port,
			cfg.Server.CertFile, // 证书
			cfg.Server.KeyFile,  // 私钥
		)
	} else {
		err = r.Run(cfg.Server.Port)
	}

	if err != nil {
		logger.Fatal("启动服务器失败", zap.Error(err))
	}
}
