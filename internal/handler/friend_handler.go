package handler

import (
	"errors"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type FriendHandler struct {
	friendService service.FriendService
}

func NewFriendHandler(friendService service.FriendService) *FriendHandler {
	return &FriendHandler{
		friendService: friendService,
	}
}

// ----------好友 handler 方法----------
// 发送好友申请
func (h *FriendHandler) Create(c *gin.Context) {
	type CreateFriendRequest struct {
		FriendID uint   `json:"friend_id"`
		Account  string `json:"account"`
	}

	var req CreateFriendRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	if req.FriendID != 0 {
		err = h.friendService.SendFriendRequest(c.Request.Context(), userID, req.FriendID)
	} else if req.Account != "" {
		req.FriendID, err = h.friendService.SendFriendRequestByAccount(c.Request.Context(), userID, req.Account)
	} else {
		response.Error(c, http.StatusBadRequest, "缺少有效的好友信息")
		return
	}

	if err != nil {
		if errors.Is(err, service.ErrCannotAddSelf) || errors.Is(err, service.ErrAlreadyFriend) || errors.Is(err, service.ErrRequestExists) {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, service.ErrUserNotFound) {
			response.Error(c, http.StatusNotFound, "未找到该用户")
			return
		}
		response.Error(c, http.StatusInternalServerError, "发送好友申请失败")
		return
	}

	response.Success(c, nil, "好友申请已发送")
}

func (h *FriendHandler) Delete(c *gin.Context) {
	type DeleteFriendRequest struct {
		FriendID uint `json:"friend_id" binding:"required"`
	}

	var req DeleteFriendRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	err = h.friendService.RemoveFriend(c.Request.Context(), userID, req.FriendID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "删除好友失败")
		return
	}

	response.Success(c, nil, "删除好友成功")
}

// 获取好友列表
func (h *FriendHandler) GetByUserID(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	friendDetails, err := h.friendService.GetFriendDetailsByUserID(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "获取好友列表失败")
		return
	}

	response.Success(c, friendDetails, "获取好友列表成功")
}

// 获取好友申请列表
func (h *FriendHandler) GetFriendRequests(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	requests, err := h.friendService.GetFriendRequestsByUserID(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "获取好友申请列表失败")
		return
	}

	response.Success(c, requests, "获取好友申请列表成功")
}

func (h *FriendHandler) HandleFriendRequest(c *gin.Context) {
	type HandleFriendRequest struct {
		RequestID uint `json:"request_id" binding:"required"`
		Status    uint `json:"status" binding:"required"`
	}

	var req HandleFriendRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	err = h.friendService.HandleFriendRequest(c.Request.Context(), userID, req.RequestID, req.Status)
	if err != nil {
		if errors.Is(err, service.ErrFriendRequestPermission) {
			response.Error(c, http.StatusForbidden, err.Error())
			return
		}
		if errors.Is(err, service.ErrInvalidFriendRequestStatus) {
			response.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Error(c, http.StatusNotFound, "好友申请不存在")
			return
		}
		response.Error(c, http.StatusInternalServerError, "处理好友申请失败")
		return
	}

	response.Success(c, nil, "处理好友申请成功")
}

// 检查好友关系
func (h *FriendHandler) CheckFriendship(c *gin.Context) {
	type CheckFriendshipRequest struct {
		FriendID uint `json:"friend_id" binding:"required"`
	}

	var req CheckFriendshipRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	isFriend := h.friendService.CheckFriendship(c.Request.Context(), userID, req.FriendID)

	response.Success(c, gin.H{"is_friend": isFriend}, "检查好友关系成功")
}

// 更新好友备注
func (h *FriendHandler) UpdateRemark(c *gin.Context) {
	type UpdateRemarkRequest struct {
		FriendID uint   `json:"friend_id" binding:"required"`
		Remark   string `json:"remark"`
	}

	var req UpdateRemarkRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	err = h.friendService.UpdateRemark(c.Request.Context(), userID, req.FriendID, req.Remark)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "修改好友备注失败")
		return
	}

	response.Success(c, nil, "修改好友备注成功")
}
