package main

import (
	"log"
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/db"
	"sleet0922/graduation_project/internal/router"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.InitConfig()

	gin.SetMode(cfg.Server.Mode_Release)

	database := db.InitDB(cfg)

	r := router.InitRouter(database)

	err := r.Run(cfg.Server.Port)
	if err != nil {
		log.Fatalf("启动服务器失败: %v", err)
	}
}
