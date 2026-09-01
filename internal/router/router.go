package router

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"sleet0922/graduation_project/internal/config"
	dbstore "sleet0922/graduation_project/internal/db"
	"sleet0922/graduation_project/internal/handler"
	"sleet0922/graduation_project/internal/middleware"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/oss"
	redisPkg "sleet0922/graduation_project/pkg/redis"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// ReadinessCheck is a named dependency probe used by /health/ready. Keeping
// checks as functions lets tests provide deterministic fakes and keeps the
// router independent of concrete infrastructure clients.
type ReadinessCheck struct {
	Name  string
	Check func(context.Context) error
}

// Dependencies is the complete object graph required by the HTTP API. The
// production constructor NewDependencies wires repositories and services;
// tests can inject fakes directly without opening a database or Redis server.
type Dependencies struct {
	Config *config.ViperConfig
	DB     *gorm.DB
	Redis  *goredis.Client
	Logger *slog.Logger

	JWTManager   *jwt.JWTManager
	SessionStore redisPkg.SessionStore
	OSSClient    *oss.QiniuKodo

	UserService   service.UserService
	FriendService service.FriendService
	GroupService  service.GroupService
	ChatService   service.ChatService
	RTCService    service.RTCService
	E2EEService   service.E2EEService
	FeedService   service.FeedService

	ReadinessChecks []ReadinessCheck
}

// NewDependencies wires the default repository/service graph. It is separate
// from NewRouter so composition can be replaced in tests or future binaries.
func NewDependencies(database *gorm.DB, cfg *config.ViperConfig, redisClient *goredis.Client) (Dependencies, error) {
	if database == nil {
		return Dependencies{}, fmt.Errorf("database dependency is nil")
	}
	if cfg == nil {
		return Dependencies{}, fmt.Errorf("config dependency is nil")
	}
	if redisClient == nil {
		return Dependencies{}, fmt.Errorf("redis dependency is nil")
	}
	jwtManager := jwt.NewJWTManager(cfg.JWT.SecretKey)
	userRepo := repo.NewUserRepository(database)
	friendRepo := repo.NewFriendRepository(database)
	groupRepo := repo.NewGroupRepository(database)
	e2eeKeyRepo := repo.NewE2EEKeyRepository(database)
	e2eeGroupKeyRepo := repo.NewE2EEGroupKeyRepository(database)
	feedRepo := repo.NewFeedRepository(database)

	userService := service.NewUserService(userRepo)
	// chatService must be initialized before friendService because friend
	// requests use it to push notifications.
	chatOptions := []service.ChatServiceOption{
		service.WithE2EEMessageValidation(e2eeKeyRepo, e2eeGroupKeyRepo),
		service.WithRedisClient(redisClient),
	}
	chatService := service.NewChatService(friendRepo, groupRepo, chatOptions...)
	friendService := service.NewFriendService(friendRepo, userRepo, chatService)
	e2eeService := service.NewE2EEService(e2eeKeyRepo, groupRepo, e2eeGroupKeyRepo, friendRepo, chatService)
	groupService := service.NewGroupService(groupRepo, friendRepo, userRepo, e2eeService, chatService)
	feedService := service.NewFeedService(feedRepo, userRepo)

	rtcTokenTTL := time.Duration(cfg.LiveKit.TokenExpireSeconds) * time.Second
	if rtcTokenTTL <= 0 {
		rtcTokenTTL = 2 * time.Hour
	}
	rtcService := service.NewRTCService(cfg.LiveKit.URL, cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, rtcTokenTTL, userRepo, friendRepo, groupRepo, chatService)

	checks := defaultReadinessChecks(database, redisClient)

	sessionStore, err := redisPkg.NewSessionStore(redisClient)
	if err != nil {
		return Dependencies{}, fmt.Errorf("session store dependency: %w", err)
	}
	return Dependencies{
		Config:          cfg,
		DB:              database,
		Redis:           redisClient,
		JWTManager:      jwtManager,
		SessionStore:    sessionStore,
		OSSClient:       oss.NewQiniuKodo(cfg),
		UserService:     userService,
		FriendService:   friendService,
		GroupService:    groupService,
		ChatService:     chatService,
		RTCService:      rtcService,
		E2EEService:     e2eeService,
		FeedService:     feedService,
		ReadinessChecks: checks,
	}, nil
}

func (d Dependencies) Validate() error {
	if d.Config == nil {
		return fmt.Errorf("router config dependency is nil")
	}
	missing := make([]string, 0, 7)
	if d.JWTManager == nil {
		missing = append(missing, "jwt_manager")
	}
	if d.OSSClient == nil {
		missing = append(missing, "oss_client")
	}
	if d.UserService == nil {
		missing = append(missing, "user_service")
	}
	if d.FriendService == nil {
		missing = append(missing, "friend_service")
	}
	if d.GroupService == nil {
		missing = append(missing, "group_service")
	}
	if d.ChatService == nil {
		missing = append(missing, "chat_service")
	}
	if d.RTCService == nil {
		missing = append(missing, "rtc_service")
	}
	if d.E2EEService == nil {
		missing = append(missing, "e2ee_service")
	}
	if d.FeedService == nil {
		missing = append(missing, "feed_service")
	}
	if len(missing) > 0 {
		return fmt.Errorf("router dependencies missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

// NewRouter builds a Fiber app from an explicit dependency graph. It performs
// no network or database initialization and returns configuration errors to the
// caller rather than panicking.
func NewRouter(deps Dependencies) (*fiber.App, error) {
	if err := deps.Validate(); err != nil {
		return nil, err
	}
	if deps.SessionStore == nil {
		if deps.Redis != nil {
			store, err := redisPkg.NewSessionStore(deps.Redis)
			if err != nil {
				return nil, err
			}
			deps.SessionStore = store
		} else {
			return nil, fmt.Errorf("router session store dependency is nil (provide SessionStore or Redis)")
		}
	}
	if len(deps.ReadinessChecks) == 0 {
		deps.ReadinessChecks = defaultReadinessChecks(deps.DB, deps.Redis)
	}

	r := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          apiErrorHandler(deps.Logger),
	})

	r.Use(middleware.Logger(deps.Logger))
	r.Use(middleware.Recovery(deps.Logger))
	r.Use(middleware.Cors(deps.Config.Server.AllowedOrigins))
	registerHealthRoutes(r, deps.ReadinessChecks, deps.Logger)

	userHandler, err := handler.NewUserHandler(
		deps.UserService,
		deps.JWTManager,
		tokenTTL(deps.Config.JWT.AccessTokenExpireSeconds, 24*time.Hour),
		tokenTTL(deps.Config.JWT.RefreshTokenExpireSeconds, 30*24*time.Hour),
		deps.ChatService,
		handler.WithSessionStore(deps.SessionStore),
	)
	if err != nil {
		return nil, fmt.Errorf("build user handler: %w", err)
	}
	ossHandler := handler.NewOssHandler(deps.OSSClient)
	friendHandler := handler.NewFriendHandler(deps.FriendService)
	groupHandler := handler.NewGroupHandler(deps.GroupService)
	chatHandler := handler.NewChatHandler(deps.ChatService, deps.RTCService)
	onlineHandler := handler.NewOnlineHandler(deps.ChatService)
	rtcHandler := handler.NewRTCHandler(deps.RTCService)
	e2eeHandler := handler.NewE2EEHandler(deps.E2EEService)
	feedHandler := handler.NewFeedHandler(deps.FeedService)

	registerPublicRoutes(r, userHandler, ossHandler)
	if err := registerAuthenticatedRoutes(r, deps, userHandler, ossHandler, friendHandler, groupHandler, chatHandler, onlineHandler, rtcHandler, e2eeHandler, feedHandler); err != nil {
		return nil, fmt.Errorf("register authenticated routes: %w", err)
	}
	return r, nil
}

func defaultReadinessChecks(database *gorm.DB, redisClient *goredis.Client) []ReadinessCheck {
	checks := make([]ReadinessCheck, 0, 2)
	if database != nil {
		checks = append(checks, ReadinessCheck{Name: "database", Check: func(ctx context.Context) error {
			return dbstore.Ping(ctx, database)
		}})
	}
	if redisClient != nil {
		checks = append(checks, ReadinessCheck{Name: "redis", Check: func(ctx context.Context) error {
			return redisPkg.Ping(ctx, redisClient)
		}})
	}
	return checks
}

func apiErrorHandler(log *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		status := fiber.StatusInternalServerError
		if fiberErr, ok := err.(*fiber.Error); ok && fiberErr.Code > 0 {
			status = fiberErr.Code
		}
		if status >= fiber.StatusInternalServerError && log != nil {
			log.Error("unhandled HTTP error", "error", err, "path", c.Path(), "method", c.Method())
		}
		return response.Result(c, status, intErrorCode(status), nil)
	}
}

func intErrorCode(status int) int {
	if status >= 400 && status <= 599 {
		return status
	}
	return errcode.InternalServerError
}

// NewHealthRouter builds an isolated health/readiness app for probes and unit
// tests. Production uses the same registration through NewRouter.
func NewHealthRouter(checks ...ReadinessCheck) *fiber.App {
	r := fiber.New(fiber.Config{DisableStartupMessage: true})
	registerHealthRoutes(r, checks, nil)
	return r
}

func tokenTTL(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func registerHealthRoutes(r *fiber.App, checks []ReadinessCheck, logger *slog.Logger) {
	liveness := func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "1.0.0",
			"time":    time.Now().Unix(),
		})
	}
	readiness := func(c *fiber.Ctx) error {
		ctx := c.UserContext()
		if ctx == nil {
			ctx = context.Background()
		}
		statuses := make(map[string]any, len(checks))
		ready := true
		for _, check := range checks {
			if strings.TrimSpace(check.Name) == "" || check.Check == nil {
				ready = false
				continue
			}
			start := time.Now()
			if err := check.Check(ctx); err != nil {
				ready = false
				statuses[check.Name] = fiber.Map{
					"status":      "error",
					"error":       err.Error(),
					"latency_ms":  time.Since(start).Milliseconds(),
					"checked_at":  time.Now().Unix(),
				}
				if logger != nil {
					logger.Warn("readiness check failed", "check", check.Name, "error", err)
				}
				continue
			}
			statuses[check.Name] = fiber.Map{
				"status":     "ok",
				"latency_ms": time.Since(start).Milliseconds(),
				"checked_at": time.Now().Unix(),
			}
		}
		status := fiber.StatusOK
		state := "ok"
		if !ready {
			status = fiber.StatusServiceUnavailable
			state = "not_ready"
		}
		return c.Status(status).JSON(fiber.Map{
			"status":  state,
			"checks":  statuses,
			"version": "1.0.0",
			"time":    time.Now().Unix(),
		})
	}

	// /health is preserved as the liveness endpoint used by existing clients.
	r.Get("/health", liveness)
	r.Get("/healthz", liveness)
	r.Get("/health/live", liveness)
	r.Get("/livez", liveness)
	r.Get("/health/ready", readiness)
	r.Get("/ready", readiness)
	r.Get("/readyz", readiness)
}

func registerPublicRoutes(r *fiber.App, userHandler *handler.UserHandler, ossHandler *handler.OssHandler) {
	r.Post("/api/user/register", userHandler.Register)
	r.Post("/api/user/login", userHandler.Login)
	r.Post("/api/user/refresh", userHandler.RefreshToken)
	r.Get("/api/oss/download-url", ossHandler.GetDownloadURL)
}

func registerAuthenticatedRoutes(
	r *fiber.App,
	deps Dependencies,
	userHandler *handler.UserHandler,
	ossHandler *handler.OssHandler,
	friendHandler *handler.FriendHandler,
	groupHandler *handler.GroupHandler,
	chatHandler *handler.ChatHandler,
	onlineHandler *handler.OnlineHandler,
	rtcHandler *handler.RTCHandler,
	e2eeHandler *handler.E2EEHandler,
	feedHandler *handler.FeedHandler,
) error {
	jwtMiddleware, err := middleware.NewJWTMiddleware(deps.JWTManager, deps.SessionStore)
	if err != nil {
		return err
	}
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

	wsAuth := jwtMiddleware.WSAuth()
	r.Use("/ws", wsAuth, func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	r.Get("/ws/chat", chatHandler.Connect())
	r.Get("/ws/online", onlineHandler.Connect())
	return nil
}
