package handler

import (
	"fmt"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	accessTokenExpiresIn  = time.Hour * 24
	refreshTokenExpiresIn = time.Hour * 24 * 30
)

type UserHandler struct {
	userService service.UserService
	jwtManager  *jwt.JWTManager
}

// ----------用户 handler 构造函数----------
func NewUserHandler(userService service.UserService, jwtManager *jwt.JWTManager) *UserHandler {
	return &UserHandler{
		userService: userService,
		jwtManager:  jwtManager,
	}
}

// ----------用户 handler 方法----------
func (h *UserHandler) GetSelf(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}
	user, err := h.userService.GetSelf(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "获取用户信息失败")
		return
	}
	response.Success(c, user, "获取用户信息成功")
}

func (h *UserHandler) SearchUser(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		response.Error(c, http.StatusBadRequest, "缺少搜索关键字")
		return
	}

	user, err := h.userService.SearchUser(keyword)
	if err != nil {
		response.Error(c, http.StatusNotFound, "未找到该用户")
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
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	user, err := h.userService.Register(req.Email, req.Password)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "注册失败")
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
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	user, err := h.userService.Login(req.Account, req.Password)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
		return
	}

	accessToken, err := h.jwtManager.GenerateToken(user.ID, user.Account, accessTokenExpiresIn)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "生成token失败")
		return
	}
	refreshToken, err := h.jwtManager.GenerateRefreshToken(user.ID, user.Account, refreshTokenExpiresIn)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "生成refresh token失败")
		return
	}

	response.Success(c, gin.H{
		"token":              accessToken,
		"refresh_token":      refreshToken,
		"expires_in":         int(accessTokenExpiresIn.Seconds()),
		"refresh_expires_in": int(refreshTokenExpiresIn.Seconds()),
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
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	accessToken, err := h.jwtManager.RefreshAccessToken(req.RefreshToken, accessTokenExpiresIn)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "refresh token无效")
		return
	}

	refreshToken, err := h.jwtManager.RotateRefreshToken(req.RefreshToken, refreshTokenExpiresIn)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "refresh token无效")
		return
	}

	response.Success(c, gin.H{
		"token":              accessToken,
		"refresh_token":      refreshToken,
		"expires_in":         int(accessTokenExpiresIn.Seconds()),
		"refresh_expires_in": int(refreshTokenExpiresIn.Seconds()),
	}, "刷新token成功")
}

func (h *UserHandler) UpdateAvatar(c *gin.Context) {
	type UpdateAvatarRequest struct {
		ObjectKey string `json:"object_key" binding:"required"`
	}
	var req UpdateAvatarRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}
	user, err := h.userService.UpdateAvatar(userID, req.ObjectKey)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "更新头像失败")
		return
	}
	response.Success(c, gin.H{"id": user.ID, "object_key": user.Avatar}, "更新头像成功")
}

func (h *UserHandler) UpdateName(c *gin.Context) {
	type UpdateNameRequest struct {
		Name string `json:"name" binding:"required"`
	}
	var req UpdateNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}
	user, err := h.userService.UpdateName(userID, req.Name)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "更新用户名失败")
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
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}
	err = h.userService.UpdatePassword(userID, req.Password, req.NewPassword)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, err.Error())
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
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}
	user, err := h.userService.UpdateProfile(userID, req.Gender, req.Birthday, req.Location)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "更新资料失败")
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
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	err = h.userService.Delete(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "删除用户失败")
		return
	}

	response.Success(c, nil, "删除用户成功")
}
