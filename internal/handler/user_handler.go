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

	"github.com/gin-gonic/gin"
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
func (h *UserHandler) GetSelf(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	user, err := h.userService.GetSelf(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Result(c, http.StatusNotFound, errcode.ErrorUserNotExist, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, user, "获取用户信息成功")
}

// 搜索用户
func (h *UserHandler) SearchUser(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	user, err := h.userService.SearchUser(c.Request.Context(), keyword)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Result(c, http.StatusNotFound, errcode.ErrorUserNotExist, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, gin.H{
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
func (h *UserHandler) Register(c *gin.Context) {
	type RegisterRequest struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	var req RegisterRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	user, err := h.userService.Register(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrUserAlreadyExists) {
			response.Result(c, http.StatusOK, errcode.ErrorUserExist, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, gin.H{
		"id":      user.ID,
		"account": user.Account,
		"name":    user.Name,
		"email":   user.Email,
	}, "注册成功")
}

// 用户登录
func (h *UserHandler) Login(c *gin.Context) {
	type LoginRequest struct {
		Account  string `json:"account" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	var req LoginRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	user, err := h.userService.Login(c.Request.Context(), req.Account, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Result(c, http.StatusUnauthorized, errcode.ErrorPasswordCheck, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	sessionID := generateSessionID()
	_, err = redisPkg.SetUserSession(user.ID, sessionID, h.refreshTokenExpiresIn)
	if err != nil {
		slog.Warn("SetUserSession failed", slog.Any("user_id", user.ID), slog.Any("error", err))
	}
	if h.chatService != nil {
		h.chatService.KickUserConnections(user.ID, "账号在其他设备登录")
	}

	accessToken, err := h.jwtManager.GenerateTokenWithSession(user.ID, user.Account, jwt.TokenTypeAccess, sessionID, h.accessTokenExpiresIn)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
		return
	}
	refreshToken, err := h.jwtManager.GenerateTokenWithSession(user.ID, user.Account, jwt.TokenTypeRefresh, sessionID, h.refreshTokenExpiresIn)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
		return
	}

	response.Success(c, gin.H{
		"token":              accessToken,
		"refresh_token":      refreshToken,
		"expires_in":         int(h.accessTokenExpiresIn.Seconds()),
		"refresh_expires_in": int(h.refreshTokenExpiresIn.Seconds()),
		"session_id":         sessionID,
		"user": gin.H{
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
func (h *UserHandler) ReportLocation(c *gin.Context) {
	type LocationRequest struct {
		Latitude  float64 `json:"latitude" binding:"required"`
		Longitude float64 `json:"longitude" binding:"required"`
		Province  string  `json:"province"`
		City      string  `json:"city"`
		District  string  `json:"district"`
		Address   string  `json:"address"`
		Timestamp int64   `json:"timestamp"`
	}

	var req LocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
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

	if err := h.userService.UpsertLocation(c.Request.Context(), location); err != nil {
		slog.Error("Failed to save user location", slog.Any("error", err), slog.Any("user_id", userID))
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
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

	response.Success(c, nil, "上报成功")
}

// 用户刷新token
func (h *UserHandler) RefreshToken(c *gin.Context) {
	type RefreshTokenRequest struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	var req RefreshTokenRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	accessToken, err := h.jwtManager.RefreshAccessToken(req.RefreshToken, h.accessTokenExpiresIn)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.ErrorTokenParse, nil)
		return
	}

	refreshToken, err := h.jwtManager.RotateRefreshToken(req.RefreshToken, h.refreshTokenExpiresIn)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.ErrorTokenParse, nil)
		return
	}

	response.Success(c, gin.H{
		"token":              accessToken,
		"refresh_token":      refreshToken,
		"expires_in":         int(h.accessTokenExpiresIn.Seconds()),
		"refresh_expires_in": int(h.refreshTokenExpiresIn.Seconds()),
	}, "刷新token成功")
}

// 用户更新头像
func (h *UserHandler) UpdateAvatar(c *gin.Context) {
	type UpdateAvatarRequest struct {
		Avatar string `json:"avatar" binding:"required"`
	}

	var req UpdateAvatarRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	user, err := h.userService.UpdateAvatar(c.Request.Context(), userID, req.Avatar)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, gin.H{"id": user.ID, "object_key": user.Avatar}, "更新头像成功")
}

// 用户更新用户名
func (h *UserHandler) UpdateName(c *gin.Context) {
	type UpdateNameRequest struct {
		Name string `json:"name" binding:"required"`
	}

	var req UpdateNameRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	user, err := h.userService.UpdateName(c.Request.Context(), userID, req.Name)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, gin.H{"id": user.ID, "name": user.Name}, "更新用户名成功")
}

// 用户更新密码
func (h *UserHandler) UpdatePassword(c *gin.Context) {
	type UpdatePasswordRequest struct {
		Password    string `json:"password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}

	var req UpdatePasswordRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	err = h.userService.UpdatePassword(c.Request.Context(), userID, req.Password, req.NewPassword)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			response.Result(c, http.StatusNotFound, errcode.ErrorUserNotExist, nil)
			return
		}
		if errors.Is(err, service.ErrOldPasswordIncorrect) {
			response.Result(c, http.StatusUnauthorized, errcode.ErrorPasswordCheck, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, nil, "更新密码成功")
}

// 用户更新资料
func (h *UserHandler) UpdateProfile(c *gin.Context) {
	type UpdateProfileRequest struct {
		Gender   int    `json:"gender"`
		Birthday string `json:"birthday"`
		Location string `json:"location"`
	}

	var req UpdateProfileRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	user, err := h.userService.UpdateProfile(c.Request.Context(), userID, req.Gender, req.Birthday, req.Location)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, gin.H{
		"id":       user.ID,
		"gender":   user.Gender,
		"birthday": user.Birthday,
		"location": user.Location,
	}, "更新资料成功")
}

// 用户删除
func (h *UserHandler) Delete(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	err = h.userService.Delete(c.Request.Context(), userID)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, nil, "删除用户成功")
}
