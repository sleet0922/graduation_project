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

	// 添加中间件
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
	friendRepo := repo.NewFriendRepository(db)
	friendService := service.NewFriendService(friendRepo)
	friendHandler := handler.NewFriendHandler(friendService, jwtManager)

	// api 路由
	r.POST("/api/user/register", userHandler.Register)
	r.POST("/api/user/login", userHandler.Login)
	r.GET("/api/oss/upload-url", ossHandler.GetUploadURL)
	r.GET("/api/oss/download-url", ossHandler.GetDownloadURL)
	r.POST("/api/user/avatar", jwtMiddleware.Auth(), userHandler.UpdateAvatar)
	r.POST("/api/user/name_update", jwtMiddleware.Auth(), userHandler.UpdateName)
	r.POST("/api/user/password_update", jwtMiddleware.Auth(), userHandler.UpdatePassword)
	r.POST("/api/user/self", jwtMiddleware.Auth(), userHandler.GetSelf)
	r.POST("/api/friend/request", jwtMiddleware.Auth(), friendHandler.Create)
	r.GET("/api/friend/requests", jwtMiddleware.Auth(), friendHandler.GetFriendRequests)
	r.POST("/api/friend/handle", jwtMiddleware.Auth(), friendHandler.HandleFriendRequest)
	r.POST("/api/friend/delete", jwtMiddleware.Auth(), friendHandler.Delete)
	r.GET("/api/friend/list", jwtMiddleware.Auth(), friendHandler.GetByUserID)
	r.POST("/api/friend/check", jwtMiddleware.Auth(), friendHandler.CheckFriendship)
	return r
}
