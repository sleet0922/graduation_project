package main

import (
	"log"
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/db"
	"sleet0922/graduation_project/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化配置
	cfg := config.InitConfig()

	// 服务器模式 (必须在初始化路由之前设置)
	gin.SetMode(cfg.Server.Mode_Release)

	// 初始化数据库
	database := db.InitDB(cfg)

	// 初始化路由
	r := router.InitRouter(database)

	// 启动服务器
	err := r.Run(cfg.Server.Port)
	if err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
