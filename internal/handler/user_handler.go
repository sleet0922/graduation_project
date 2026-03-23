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

func (h *UserHandler) getUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("user_id not found in context")
	}
	return userID.(uint), nil
}
func (h *UserHandler) Register(c *gin.Context) {
	type RegisterRequest struct {
		Name     string `json:"name" binding:"required"`
		Account  string `json:"account" binding:"required"`
		Password string `json:"password" binding:"required"`
		Phone    string `json:"phone" binding:"required"`
	}

	var req RegisterRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	user, err := h.userService.Register(req.Name, req.Account, req.Password, req.Phone)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "注册失败")
		return
	}
	response.Success(c, gin.H{
		"id":      user.ID,
		"account": user.Account,
		"name":    user.Name,
		"phone":   user.Phone,
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

	token, err := h.jwtManager.GenerateToken(user.ID, user.Account, time.Hour*24)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "生成token失败")
		return
	}

	response.Success(c, gin.H{
		"token": token,
		"user": gin.H{
			"id":       user.ID,
			"account":  user.Account,
			"name":     user.Name,
			"avatar":   user.Avatar,
			"phone":    user.Phone,
			"gender":   user.Gender,
			"birthday": user.Birthday,
			"location": user.Location,
		},
	}, "登录成功")
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
