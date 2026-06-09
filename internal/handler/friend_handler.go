package handler

import (
	"errors"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type FriendHandler struct {
	friendService service.FriendService
}

func NewFriendHandler(friendService service.FriendService) *FriendHandler {
	return &FriendHandler{friendService: friendService}
}

// ----------好友 handler 方法----------
// 发送好友申请
func (h *FriendHandler) Create(c *fiber.Ctx) error {
	type CreateFriendRequest struct {
		FriendID uint   `json:"friend_id"`
		Account  string `json:"account"`
	}

	var req CreateFriendRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	if req.FriendID != 0 {
		err = h.friendService.SendFriendRequest(c.Context(), userID, req.FriendID)
	} else if req.Account != "" {
		req.FriendID, err = h.friendService.SendFriendRequestByAccount(c.Context(), userID, req.Account)
	} else {
		return response.Error(c, http.StatusBadRequest, "缺少有效的好友信息")
	}

	if err != nil {
		if errors.Is(err, service.ErrCannotAddSelf) || errors.Is(err, service.ErrAlreadyFriend) || errors.Is(err, service.ErrRequestExists) {
			return response.Error(c, http.StatusBadRequest, err.Error())
		}
		if errors.Is(err, service.ErrUserNotFound) {
			return response.Error(c, http.StatusNotFound, "未找到该用户")
		}
		return response.Error(c, http.StatusInternalServerError, "发送好友申请失败")
	}

	return response.Success(c, nil, "好友申请已发送")
}

func (h *FriendHandler) Delete(c *fiber.Ctx) error {
	type DeleteFriendRequest struct {
		FriendID uint `json:"friend_id"`
	}

	var req DeleteFriendRequest
	if err := c.BodyParser(&req); err != nil || req.FriendID == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.friendService.RemoveFriend(c.Context(), userID, req.FriendID)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "删除好友失败")
	}

	return response.Success(c, nil, "删除好友成功")
}

// 获取好友列表
func (h *FriendHandler) GetByUserID(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	friendDetails, err := h.friendService.GetFriendDetailsByUserID(c.Context(), userID)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "获取好友列表失败")
	}

	return response.Success(c, friendDetails, "获取好友列表成功")
}

// 获取好友申请列表
func (h *FriendHandler) GetFriendRequests(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	requests, err := h.friendService.GetFriendRequestsByUserID(c.Context(), userID)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "获取好友申请列表失败")
	}

	return response.Success(c, requests, "获取好友申请列表成功")
}

func (h *FriendHandler) HandleFriendRequest(c *fiber.Ctx) error {
	type HandleFriendRequest struct {
		RequestID uint `json:"request_id"`
		Status    uint `json:"status"`
	}

	var req HandleFriendRequest
	if err := c.BodyParser(&req); err != nil || req.RequestID == 0 || req.Status == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.friendService.HandleFriendRequest(c.Context(), userID, req.RequestID, req.Status)
	if err != nil {
		if errors.Is(err, service.ErrFriendRequestPermission) {
			return response.Error(c, http.StatusForbidden, err.Error())
		}
		if errors.Is(err, service.ErrInvalidFriendRequestStatus) {
			return response.Error(c, http.StatusBadRequest, err.Error())
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return response.Error(c, http.StatusNotFound, "好友申请不存在")
		}
		return response.Error(c, http.StatusInternalServerError, "处理好友申请失败")
	}

	return response.Success(c, nil, "处理好友申请成功")
}

// 检查好友关系
func (h *FriendHandler) CheckFriendship(c *fiber.Ctx) error {
	type CheckFriendshipRequest struct {
		FriendID uint `json:"friend_id"`
	}

	var req CheckFriendshipRequest
	if err := c.BodyParser(&req); err != nil || req.FriendID == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	isFriend := h.friendService.CheckFriendship(c.Context(), userID, req.FriendID)

	return response.Success(c, fiber.Map{"is_friend": isFriend}, "检查好友关系成功")
}

// 更新好友备注
func (h *FriendHandler) UpdateRemark(c *fiber.Ctx) error {
	type UpdateRemarkRequest struct {
		FriendID uint   `json:"friend_id"`
		Remark   string `json:"remark"`
	}

	var req UpdateRemarkRequest
	if err := c.BodyParser(&req); err != nil || req.FriendID == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.friendService.UpdateRemark(c.Context(), userID, req.FriendID, req.Remark)
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "修改好友备注失败")
	}

	return response.Success(c, nil, "修改好友备注成功")
}
