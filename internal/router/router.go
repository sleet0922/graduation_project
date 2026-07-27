package router

import (
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/handler"
	"sleet0922/graduation_project/internal/middleware"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/oss"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB, cfg *config.ViperConfig) *fiber.App {
	r := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	r.Use(middleware.Logger())
	r.Use(middleware.Recovery())
	r.Use(middleware.Cors())

	jwtManager := jwt.NewJWTManager(cfg.JWT.SecretKey)
	jwtMiddleware := middleware.NewJWTMiddleware(jwtManager)

	userRepo := repo.NewUserRepository(db)
	friendRepo := repo.NewFriendRepository(db)
	groupRepo := repo.NewGroupRepository(db)
	e2eeKeyRepo := repo.NewE2EEKeyRepository(db)
	e2eeGroupKeyRepo := repo.NewE2EEGroupKeyRepository(db)
	feedRepo := repo.NewFeedRepository(db)

	userService := service.NewUserService(userRepo)
	// chatService 需先初始化，friendService 依赖它推送好友申请通知
	chatService := service.NewChatService(friendRepo, groupRepo)
	friendService := service.NewFriendService(friendRepo, userRepo, chatService)
	e2eeService := service.NewE2EEService(e2eeKeyRepo, groupRepo, e2eeGroupKeyRepo, friendRepo, chatService)
	groupService := service.NewGroupService(groupRepo, friendRepo, userRepo, e2eeService, chatService)
	feedService := service.NewFeedService(feedRepo, userRepo, db)

	rtcTokenTTL := time.Duration(cfg.LiveKit.TokenExpireSeconds) * time.Second
	if rtcTokenTTL <= 0 {
		rtcTokenTTL = 2 * time.Hour
	}
	rtcService := service.NewRTCService(cfg.LiveKit.URL, cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, rtcTokenTTL, userRepo, friendRepo, groupRepo, chatService)

	accessTokenTTL := time.Duration(cfg.JWT.AccessTokenExpireSeconds) * time.Second
	refreshTokenTTL := time.Duration(cfg.JWT.RefreshTokenExpireSeconds) * time.Second
	userHandler := handler.NewUserHandler(userService, jwtManager, accessTokenTTL, refreshTokenTTL, chatService)

	kodoClient := oss.NewQiniuKodo(cfg)
	ossHandler := handler.NewOssHandler(kodoClient)

	friendHandler := handler.NewFriendHandler(friendService)
	groupHandler := handler.NewGroupHandler(groupService)
	chatHandler := handler.NewChatHandler(chatService, rtcService)
	onlineHandler := handler.NewOnlineHandler(chatService)
	rtcHandler := handler.NewRTCHandler(rtcService)
	e2eeHandler := handler.NewE2EEHandler(e2eeService)
	feedHandler := handler.NewFeedHandler(feedService)

	// 公开路由
	r.Post("/api/user/register", userHandler.Register)
	r.Post("/api/user/login", userHandler.Login)
	r.Post("/api/user/refresh", userHandler.RefreshToken)
	r.Get("/api/oss/download-url", ossHandler.GetDownloadURL)

	// 需要认证的路由
	auth := jwtMiddleware.Auth()
	r.Get("/api/oss/upload-url", auth, ossHandler.GetUploadURL)
	r.Post("/api/chat/upload/image", auth, ossHandler.UploadChatImage)
	r.Post("/api/chat/upload/video", auth, ossHandler.UploadChatVideo)
	r.Post("/api/user/avatar_update", auth, userHandler.UpdateAvatar)
	r.Post("/api/user/name_update", auth, userHandler.UpdateName)
	r.Post("/api/user/password_update", auth, userHandler.UpdatePassword)
	r.Post("/api/user/profile_update", auth, userHandler.UpdateProfile)
	r.Post("/api/user/self", auth, userHandler.GetSelf)
	r.Post("/api/user/location", auth, userHandler.ReportLocation)
	r.Get("/api/user/search", auth, userHandler.SearchUser)
	r.Post("/api/friend/request", auth, friendHandler.Create)
	r.Get("/api/friend/requests", auth, friendHandler.GetFriendRequests)
	r.Post("/api/friend/handle", auth, friendHandler.HandleFriendRequest)
	r.Post("/api/friend/delete", auth, friendHandler.Delete)
	r.Get("/api/friend/list", auth, friendHandler.GetByUserID)
	r.Post("/api/friend/check", auth, friendHandler.CheckFriendship)
	r.Post("/api/friend/remark_update", auth, friendHandler.UpdateRemark)
	r.Post("/api/group/create", auth, groupHandler.Create)
	r.Post("/api/group/member/add", auth, groupHandler.AddMembers)
	r.Post("/api/group/member/remove", auth, groupHandler.RemoveMember)
	r.Post("/api/group/leave", auth, groupHandler.Leave)
	r.Post("/api/group/delete", auth, groupHandler.Delete)
	r.Get("/api/group/list", auth, groupHandler.GetGroups)
	r.Get("/api/group/members", auth, groupHandler.GetMembers)
	r.Post("/api/rtc/call/invite", auth, rtcHandler.Invite)
	r.Post("/api/rtc/call/accept", auth, rtcHandler.Accept)
	r.Post("/api/rtc/call/reject", auth, rtcHandler.Reject)
	r.Post("/api/rtc/call/cancel", auth, rtcHandler.Cancel)
	r.Post("/api/rtc/call/hangup", auth, rtcHandler.Hangup)
	r.Post("/api/rtc/token", auth, rtcHandler.GetToken)
	r.Post("/api/e2ee/keys/publish", auth, e2eeHandler.PublishPublicKey)
	r.Get("/api/e2ee/keys/public", auth, e2eeHandler.GetPublicKey)
	r.Post("/api/e2ee/group/key/publish", auth, e2eeHandler.PublishGroupKeyBoxes)
	r.Post("/api/e2ee/group/key/rotate", auth, e2eeHandler.RotateGroupKey)
	r.Get("/api/e2ee/group/key/current", auth, e2eeHandler.GetGroupCurrentKey)
	r.Get("/api/e2ee/group/key/by-version", auth, e2eeHandler.GetGroupKeyByVersion)
	r.Post("/api/user/delete", auth, userHandler.Delete)

	// 朋友圈/动态
	r.Post("/api/feed/create", auth, feedHandler.CreatePost)
	r.Delete("/api/feed/delete", auth, feedHandler.DeletePost)
	r.Get("/api/feed/detail", auth, feedHandler.GetDetail)
	r.Get("/api/feed/list", auth, feedHandler.ListFeed)
	r.Get("/api/feed/my_posts", auth, feedHandler.ListMyPosts)
	r.Post("/api/feed/like", auth, feedHandler.ToggleLike)
	r.Get("/api/feed/is_liked", auth, feedHandler.IsLiked)
	r.Post("/api/feed/comment", auth, feedHandler.CreateComment)
	r.Delete("/api/feed/comment", auth, feedHandler.DeleteComment)
	r.Get("/api/feed/comments", auth, feedHandler.ListComments)

	// WebSocket 路由（需要 WSAuth 认证）
	wsAuth := jwtMiddleware.WSAuth()
	r.Use("/ws", wsAuth, func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	r.Get("/ws/chat", chatHandler.Connect())
	r.Get("/ws/online", onlineHandler.Connect())

	return r
}
