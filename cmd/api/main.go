package main

import (
	"log"
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/db"
	"sleet0922/graduation_project/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	// 设置全局日志格式，包含日期、时间以及文件名和行号
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// 初始化配置
	cfg := config.InitConfig()
	gin.SetMode(cfg.Server.Mode_Release)
	database := db.InitDB(cfg)
	r := router.InitRouter(database, cfg)

	log.Printf("服务器启动, 监听端口 %s", cfg.Server.Port)
	err := r.Run(cfg.Server.Port)
	if err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
