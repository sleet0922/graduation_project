package handler

import (
	"fmt"
	"net/http"
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService           service.UserService
	jwtManager            *jwt.JWTManager
	accessTokenExpiresIn  time.Duration
	refreshTokenExpiresIn time.Duration
}

// ----------用户 handler 构造函数----------
func NewUserHandler(userService service.UserService, jwtManager *jwt.JWTManager, cfg *config.ViperConfig) *UserHandler {
	accessTokenTTL := time.Duration(cfg.JWT.AccessTokenExpireSeconds) * time.Second
	if accessTokenTTL <= 0 {
		accessTokenTTL = 24 * time.Hour
	}
	refreshTokenTTL := time.Duration(cfg.JWT.RefreshTokenExpireSeconds) * time.Second
	if refreshTokenTTL <= 0 {
		refreshTokenTTL = 30 * 24 * time.Hour
	}

	return &UserHandler{
		userService:           userService,
		jwtManager:            jwtManager,
		accessTokenExpiresIn:  accessTokenTTL,
		refreshTokenExpiresIn: refreshTokenTTL,
	}
}

// ----------用户 handler 方法----------
func (h *UserHandler) GetSelf(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	user, err := h.userService.GetSelf(c.Request.Context(), userID)
	if err != nil {
		if err == service.ErrUserNotFound {
			response.Result(c, http.StatusNotFound, errcode.ErrorUserNotExist, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, user, "获取用户信息成功")
}

func (h *UserHandler) SearchUser(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	user, err := h.userService.SearchUser(c.Request.Context(), keyword)
	if err != nil {
		if err == service.ErrUserNotFound {
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

func (h *UserHandler) getUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("user_id not found in context")
	}
	return userID.(uint), nil
}
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
		if err == service.ErrUserAlreadyExists {
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
		if err == service.ErrInvalidCredentials {
			response.Result(c, http.StatusUnauthorized, errcode.ErrorPasswordCheck, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	accessToken, err := h.jwtManager.GenerateToken(user.ID, user.Account, h.accessTokenExpiresIn)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
		return
	}
	refreshToken, err := h.jwtManager.GenerateRefreshToken(user.ID, user.Account, h.refreshTokenExpiresIn)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.ErrorTokenGenerate, nil)
		return
	}

	response.Success(c, gin.H{
		"token":              accessToken,
		"refresh_token":      refreshToken,
		"expires_in":         int(h.accessTokenExpiresIn.Seconds()),
		"refresh_expires_in": int(h.refreshTokenExpiresIn.Seconds()),
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
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
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
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
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
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}
	err = h.userService.UpdatePassword(c.Request.Context(), userID, req.Password, req.NewPassword)
	if err != nil {
		if err == service.ErrUserNotFound {
			response.Result(c, http.StatusNotFound, errcode.ErrorUserNotExist, nil)
			return
		}
		if err == service.ErrOldPasswordIncorrect {
			response.Result(c, http.StatusUnauthorized, errcode.ErrorPasswordCheck, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}
	response.Success(c, nil, "更新密码成功")
}

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
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
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

func (h *UserHandler) Delete(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
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
