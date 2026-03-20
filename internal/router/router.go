package router

import (
	"sleet0922/graduation_project/internal/handler"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB) *gin.Engine {
	r := gin.Default()

	// 添加中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 依赖注入
	userRepo := repo.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService)

	// 注册路由
	r.POST("/api/user/add", userHandler.Add)

	return r
}
