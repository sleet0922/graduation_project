package handler

import (
	"net/http"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/response"
	"sleet0922/graduation_project/pkg/security"
	"time"

	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService service.UserService
	jwtManager  *jwt.JWTManager
}

func NewUserHandler(userService service.UserService, jwtManager *jwt.JWTManager) *UserHandler {
	return &UserHandler{
		userService: userService,
		jwtManager:  jwtManager,
	}
}
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
		return 0, nil
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

	user := &model.User{
		Name:     req.Name,
		Account:  req.Account,
		Password: req.Password,
		Phone:    req.Phone,
	}

	err = h.userService.Register(user)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "注册失败")
		return
	}
	response.Success(c, user, "注册成功")
}

func (h *UserHandler) DeleteAll(c *gin.Context) {
	err := h.userService.DeleteAll()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "删除所有用户失败")
		return
	}
	response.Success(c, nil, "删除所有用户成功")
}

func (h *UserHandler) AddTestUser(c *gin.Context) {
	err := h.userService.AddTestUser()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "添加测试用户失败")
		return
	}
	response.Success(c, nil, "添加测试用户成功")
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
	user, err := h.userService.UpdateField(userID, "avatar", req.ObjectKey)
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
	user, err := h.userService.UpdateField(userID, "name", req.Name)
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
	user, err := h.userService.GetByID(userID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "用户不存在")
		return
	}
	if err := security.CheckPassword(user.Password, req.Password); err != nil {
		response.Error(c, http.StatusUnauthorized, "原密码错误")
		return
	}
	updatedUser, err := h.userService.UpdatePassword(userID, req.NewPassword)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "更新密码失败")
		return
	}
	response.Success(c, gin.H{"id": updatedUser.ID}, "更新密码成功")
}
