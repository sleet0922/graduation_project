package handler

import (
	"net/http"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/response"
	"time"

	"github.com/gin-gonic/gin"
)

// ----------用户handler 实现----------
type UserHandler struct {
	userService service.UserService
	jwtManager  *jwt.JWTManager
}

// ----------用户handler 构造函数----------
func NewUserHandler(userService service.UserService, jwtManager *jwt.JWTManager) *UserHandler {
	return &UserHandler{
		userService: userService,
		jwtManager:  jwtManager,
	}
}

// ----------用户handler 方法----------
func (h *UserHandler) Add(c *gin.Context) {
	var user model.User
	err := c.ShouldBindJSON(&user)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	err = h.userService.Add(&user)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "添加用户失败")
		return
	}
	response.Success(c, user, "添加用户成功")
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

	userID, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	user, err := h.userService.UpdateAvatar(userID.(uint), req.ObjectKey)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "更新头像失败")
		return
	}
	response.Success(c, gin.H{
		"id":         user.ID,
		"object_key": user.Avatar,
	}, "更新头像成功")
}
