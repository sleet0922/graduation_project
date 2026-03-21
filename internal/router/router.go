package router

import (
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/handler"
	"sleet0922/graduation_project/internal/middleware"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB, cfg *config.ViperConfig) *gin.Engine {
	r := gin.Default()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 初始化JWT
	jwtManager := jwt.NewJWTManager(cfg.JWT.SecretKey)
	jwtMiddleware := middleware.NewJWTMiddleware(jwtManager)

	// 依赖注入
	userRepo := repo.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	userHandler := handler.NewUserHandler(userService, jwtManager)
	ossHandler := handler.NewOssHandler(cfg)

	// api 路由
	r.POST("/api/user/add", userHandler.Add)
	r.POST("/api/user/login", userHandler.Login)
	r.GET("/api/oss/upload-url", ossHandler.GetUploadURL)
	r.GET("/api/oss/download-url", ossHandler.GetDownloadURL)
	r.POST("/api/user/avatar", jwtMiddleware.Auth(), userHandler.UpdateAvatar)
	r.POST("/api/delete_all", userHandler.DeleteAll)
	r.POST("/api/add_test_user", userHandler.AddTestUser)

	return r
}
