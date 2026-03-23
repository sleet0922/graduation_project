package handler

import (
	"fmt"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gin-gonic/gin"
)

type FriendHandler struct {
	friendService service.FriendService
	jwtManager    *jwt.JWTManager
}

func NewFriendHandler(friendService service.FriendService, jwtManager *jwt.JWTManager) *FriendHandler {
	return &FriendHandler{
		friendService: friendService,
		jwtManager:    jwtManager,
	}
}

func (h *FriendHandler) Create(c *gin.Context) {
	type CreateFriendRequest struct {
		FriendID uint `json:"friend_id" binding:"required"`
	}

	var req CreateFriendRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	err = h.friendService.SendFriendRequest(userID, req.FriendID)
	if err != nil {
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

	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	err = h.friendService.RemoveFriend(userID, req.FriendID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "删除好友失败")
		return
	}

	response.Success(c, nil, "删除好友成功")
}

func (h *FriendHandler) GetByUserID(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	friends, err := h.friendService.GetByUserID(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "获取好友列表失败")
		return
	}

	response.Success(c, friends, "获取好友列表成功")
}

func (h *FriendHandler) GetFriendRequests(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	requests, err := h.friendService.GetFriendRequestsByUserID(userID)
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

	err = h.friendService.HandleFriendRequest(req.RequestID, req.Status)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "处理好友申请失败")
		return
	}

	response.Success(c, nil, "处理好友申请成功")
}

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

	userID, err := h.getUserID(c)
	if err != nil || userID == 0 {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	isFriend := h.friendService.CheckFriendship(userID, req.FriendID)

	response.Success(c, gin.H{"is_friend": isFriend}, "检查好友关系成功")
}

func (h *FriendHandler) getUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("user_id not found in context")
	}
	return userID.(uint), nil
}
