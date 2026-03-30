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
	r := gin.New()

	// 添加中间件
	r.Use(middleware.GinLogger())
	r.Use(middleware.GinRecovery())
	r.Use(middleware.CorsMiddleware())

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
	friendHandler := handler.NewFriendHandler(friendService, userService, jwtManager)
	chatService := service.NewChatService(friendRepo)
	chatHandler := handler.NewChatHandler(chatService, jwtManager)

	r.POST("/api/user/register", userHandler.Register)
	r.POST("/api/user/login", userHandler.Login)
	r.POST("/api/user/refresh", userHandler.RefreshToken)
	r.GET("/api/oss/upload-url", ossHandler.GetUploadURL)
	r.GET("/api/oss/download-url", ossHandler.GetDownloadURL)
	r.GET("/ws/chat", jwtMiddleware.Auth(), chatHandler.Connect)
	r.POST("/api/user/avatar", jwtMiddleware.Auth(), userHandler.UpdateAvatar)
	r.POST("/api/user/name_update", jwtMiddleware.Auth(), userHandler.UpdateName)
	r.POST("/api/user/password_update", jwtMiddleware.Auth(), userHandler.UpdatePassword)
	r.POST("/api/user/profile_update", jwtMiddleware.Auth(), userHandler.UpdateProfile)
	r.POST("/api/user/self", jwtMiddleware.Auth(), userHandler.GetSelf)
	r.GET("/api/user/search", userHandler.SearchUser)
	r.POST("/api/friend/request", jwtMiddleware.Auth(), friendHandler.Create)
	r.GET("/api/friend/requests", jwtMiddleware.Auth(), friendHandler.GetFriendRequests)
	r.POST("/api/friend/handle", jwtMiddleware.Auth(), friendHandler.HandleFriendRequest)
	r.POST("/api/friend/delete", jwtMiddleware.Auth(), friendHandler.Delete)
	r.GET("/api/friend/list", jwtMiddleware.Auth(), friendHandler.GetByUserID)
	r.POST("/api/friend/check", jwtMiddleware.Auth(), friendHandler.CheckFriendship)
	r.POST("/api/friend/remark_update", jwtMiddleware.Auth(), friendHandler.UpdateRemark)
	r.POST("/api/user/delete", jwtMiddleware.Auth(), userHandler.Delete)
	return r
}
