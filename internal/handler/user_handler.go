package handler

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/jwt"
	redisPkg "sleet0922/graduation_project/pkg/redis"
	"sleet0922/graduation_project/pkg/response"
	"time"

	"github.com/gofiber/fiber/v2"
)

type UserHandler struct {
	userService           service.UserService
	chatService           service.ChatService
	jwtManager            *jwt.JWTManager
	accessTokenExpiresIn  time.Duration
	refreshTokenExpiresIn time.Duration
}

// ----------用户 handler 私有方法----------
// 生成 32 字符的随机 session ID
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// ----------用户 handler 构造函数----------
func NewUserHandler(userService service.UserService, jwtManager *jwt.JWTManager, accessTokenTTL, refreshTokenTTL time.Duration, chatService service.ChatService) *UserHandler {
	if accessTokenTTL <= 0 {
		accessTokenTTL = 24 * time.Hour
	}
	if refreshTokenTTL <= 0 {
		refreshTokenTTL = 30 * 24 * time.Hour
	}

	return &UserHandler{
		userService:           userService,
		chatService:           chatService,
		jwtManager:            jwtManager,
		accessTokenExpiresIn:  accessTokenTTL,
		refreshTokenExpiresIn: refreshTokenTTL,
	}
}

// ----------用户 handler 方法----------
// 获取当前用户信息
func (h *UserHandler) GetSelf(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	user, err := h.userService.GetSelf(c.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.ErrorUserNotExist, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	return response.Success(c, user, "获取用户信息成功")
}

// 搜索用户
func (h *UserHandler) SearchUser(c *fiber.Ctx) error {
	keyword := c.Query("keyword")
	if keyword == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	user, err := h.userService.SearchUser(c.Context(), keyword)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.ErrorUserNotExist, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	return response.Success(c, fiber.Map{
		"id":       user.ID,
		"account":  user.Account,
		"name":     user.Name,
		"avatar":   user.Avatar,
		"email":    user.Email,
		"gender":   user.Gender,
		"birthday": user.Birthday,
		"location": user.Location,
	}, "搜索用户成功")
}

// 用户注册
func (h *UserHandler) Register(c *fiber.Ctx) error {
	type RegisterRequest struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil || req.Email == "" || req.Password == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	user, err := h.userService.Register(c.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			return response.Result(c, http.StatusOK, errcode.ErrorUserExist, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	return response.Success(c, fiber.Map{
		"id":      user.ID,
		"account": user.Account,
		"name":    user.Name,
		"email":   user.Email,
	}, "注册成功")
}

// 用户登录
func (h *UserHandler) Login(c *fiber.Ctx) error {
	type LoginRequest struct {
		Account  string `json:"account"`
		Password string `json:"password"`
	}
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil || req.Account == "" || req.Password == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	user, err := h.userService.Login(c.Context(), req.Account, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return response.Result(c, http.StatusUnauthorized, errcode.ErrorPasswordCheck, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	sessionID := generateSessionID()
	refreshID, err := jwt.GenerateTokenID()
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
	}

	accessToken, err := h.jwtManager.GenerateTokenWithSession(user.ID, user.Account, jwt.TokenTypeAccess, sessionID, h.accessTokenExpiresIn)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
	}
	refreshToken, err := h.jwtManager.GenerateTokenWithSessionAndRefreshID(user.ID, user.Account, jwt.TokenTypeRefresh, sessionID, refreshID, h.refreshTokenExpiresIn)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
	}
	_, err = redisPkg.SetUserSession(user.ID, sessionID, h.refreshTokenExpiresIn)
	if err != nil {
		slog.Error("SetUserSession failed", slog.Any("user_id", user.ID), slog.Any("error", err))
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	if err := redisPkg.SetRefreshTokenID(user.ID, sessionID, refreshID, h.refreshTokenExpiresIn); err != nil {
		slog.Error("SetRefreshTokenID failed", slog.Any("user_id", user.ID), slog.Any("error", err))
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	if h.chatService != nil {
		h.chatService.KickUserConnections(user.ID, "账号在其他设备登录")
	}

	return response.Success(c, fiber.Map{
		"token":              accessToken,
		"refresh_token":      refreshToken,
		"expires_in":         int(h.accessTokenExpiresIn.Seconds()),
		"refresh_expires_in": int(h.refreshTokenExpiresIn.Seconds()),
		"session_id":         sessionID,
		"user": fiber.Map{
			"id":       user.ID,
			"account":  user.Account,
			"name":     user.Name,
			"avatar":   user.Avatar,
			"email":    user.Email,
			"gender":   user.Gender,
			"birthday": user.Birthday,
			"location": user.Location,
		},
	}, "登录成功")
}

// 用户位置
func (h *UserHandler) ReportLocation(c *fiber.Ctx) error {
	type LocationRequest struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Province  string  `json:"province"`
		City      string  `json:"city"`
		District  string  `json:"district"`
		Address   string  `json:"address"`
		Timestamp int64   `json:"timestamp"`
	}

	var req LocationRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	location := &model.UserLocation{
		UserID:    userID,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		Province:  req.Province,
		City:      req.City,
		District:  req.District,
		Address:   req.Address,
		Timestamp: req.Timestamp,
	}

	if err := h.userService.UpsertLocation(c.Context(), location); err != nil {
		slog.Error("Failed to save user location", slog.Any("error", err), slog.Any("user_id", userID))
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	slog.Info("Received and saved user location report",
		slog.Any("user_id", userID),
		slog.Float64("latitude", req.Latitude),
		slog.Float64("longitude", req.Longitude),
		slog.String("province", req.Province),
		slog.String("city", req.City),
		slog.String("district", req.District),
		slog.String("address", req.Address),
		slog.Int64("timestamp", req.Timestamp),
	)

	return response.Success(c, nil, "上报成功")
}

// 用户刷新token
func (h *UserHandler) RefreshToken(c *fiber.Ctx) error {
	type RefreshTokenRequest struct {
		RefreshToken string `json:"refresh_token"`
	}

	var req RefreshTokenRequest
	if err := c.BodyParser(&req); err != nil || req.RefreshToken == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}

	claims, err := h.jwtManager.ParseToken(req.RefreshToken)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.ErrorTokenParse, nil)
	}
	if claims.TokenType != jwt.TokenTypeRefresh || claims.SessionID == "" || claims.RefreshID == "" {
		return response.Result(c, http.StatusUnauthorized, errcode.ErrorTokenParse, nil)
	}
	if !redisPkg.IsSessionValid(claims.UserID, claims.SessionID) {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	newRefreshID, err := jwt.GenerateTokenID()
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
	}
	if err := redisPkg.RotateRefreshTokenID(claims.UserID, claims.SessionID, claims.RefreshID, newRefreshID, h.refreshTokenExpiresIn); err != nil {
		slog.Warn("RotateRefreshTokenID failed", slog.Any("user_id", claims.UserID), slog.Any("error", err))
		return response.Result(c, http.StatusUnauthorized, errcode.ErrorTokenParse, nil)
	}
	if err := redisPkg.ExpireUserSession(claims.UserID, h.refreshTokenExpiresIn); err != nil {
		slog.Error("ExpireUserSession failed", slog.Any("user_id", claims.UserID), slog.Any("error", err))
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}

	accessToken, err := h.jwtManager.GenerateTokenWithSession(claims.UserID, claims.Account, jwt.TokenTypeAccess, claims.SessionID, h.accessTokenExpiresIn)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
	}
	refreshToken, err := h.jwtManager.GenerateTokenWithSessionAndRefreshID(claims.UserID, claims.Account, jwt.TokenTypeRefresh, claims.SessionID, newRefreshID, h.refreshTokenExpiresIn)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
	}

	return response.Success(c, fiber.Map{
		"token":              accessToken,
		"refresh_token":      refreshToken,
		"expires_in":         int(h.accessTokenExpiresIn.Seconds()),
		"refresh_expires_in": int(h.refreshTokenExpiresIn.Seconds()),
	}, "刷新token成功")
}

// 用户更新头像
func (h *UserHandler) UpdateAvatar(c *fiber.Ctx) error {
	type UpdateAvatarRequest struct {
		Avatar string `json:"avatar"`
	}

	var req UpdateAvatarRequest
	if err := c.BodyParser(&req); err != nil || req.Avatar == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	user, err := h.userService.UpdateAvatar(c.Context(), userID, req.Avatar)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	return response.Success(c, fiber.Map{"id": user.ID, "object_key": user.Avatar}, "更新头像成功")
}

// 用户更新用户名
func (h *UserHandler) UpdateName(c *fiber.Ctx) error {
	type UpdateNameRequest struct {
		Name string `json:"name"`
	}

	var req UpdateNameRequest
	if err := c.BodyParser(&req); err != nil || req.Name == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	user, err := h.userService.UpdateName(c.Context(), userID, req.Name)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	return response.Success(c, fiber.Map{"id": user.ID, "name": user.Name}, "更新用户名成功")
}

// 用户更新密码
func (h *UserHandler) UpdatePassword(c *fiber.Ctx) error {
	type UpdatePasswordRequest struct {
		Password    string `json:"password"`
		NewPassword string `json:"new_password"`
	}

	var req UpdatePasswordRequest
	if err := c.BodyParser(&req); err != nil || req.Password == "" || req.NewPassword == "" {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	err = h.userService.UpdatePassword(c.Context(), userID, req.Password, req.NewPassword)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return response.Result(c, http.StatusNotFound, errcode.ErrorUserNotExist, nil)
		}
		if errors.Is(err, service.ErrOldPasswordIncorrect) {
			return response.Result(c, http.StatusUnauthorized, errcode.ErrorPasswordCheck, nil)
		}
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	return response.Success(c, nil, "更新密码成功")
}

// 用户更新资料
func (h *UserHandler) UpdateProfile(c *fiber.Ctx) error {
	type UpdateProfileRequest struct {
		Gender   int    `json:"gender"`
		Birthday string `json:"birthday"`
		Location string `json:"location"`
	}

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
	}
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	user, err := h.userService.UpdateProfile(c.Context(), userID, req.Gender, req.Birthday, req.Location)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	return response.Success(c, fiber.Map{
		"id":       user.ID,
		"gender":   user.Gender,
		"birthday": user.Birthday,
		"location": user.Location,
	}, "更新资料成功")
}

// 用户删除
func (h *UserHandler) Delete(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	err = h.userService.Delete(c.Context(), userID)
	if err != nil {
		return response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
	}
	return response.Success(c, nil, "删除用户成功")
}
