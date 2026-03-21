package handler

import (
	"net/http"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gin-gonic/gin"
)

// ----------用户handler 实现----------
type UserHandler struct {
	userService service.UserService
}

// ----------用户handler 构造函数----------
func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
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
