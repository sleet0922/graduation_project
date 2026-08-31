package handler

import (
	"errors"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/response"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type GroupHandler struct {
	groupService service.GroupService
}

func NewGroupHandler(groupService service.GroupService) *GroupHandler {
	return &GroupHandler{groupService: groupService}
}

// groupServiceError keeps domain failures meaningful at the HTTP boundary.
// Unknown failures are intentionally reduced to a generic 500 response so
// database or infrastructure details do not leak to clients.
func groupServiceError(c *fiber.Ctx, err error) error {
	if err == nil {
		return nil
	}
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, service.ErrGroupNotFound), errors.Is(err, service.ErrGroupMemberNotFound):
		status = http.StatusNotFound
	case errors.Is(err, service.ErrGroupPermission),
		errors.Is(err, service.ErrGroupDeleteDenied),
		errors.Is(err, service.ErrGroupKickDenied),
		errors.Is(err, service.ErrGroupLeaveDenied):
		status = http.StatusForbidden
	case errors.Is(err, service.ErrGroupNameEmpty),
		errors.Is(err, service.ErrGroupMembersEmpty),
		errors.Is(err, service.ErrGroupFriendOnly),
		errors.Is(err, service.ErrGroupOwnerProtected):
		status = http.StatusBadRequest
	}
	if status == http.StatusInternalServerError {
		return response.Result(c, status, errcode.InternalServerError, nil)
	}
	return response.Error(c, status, err.Error())
}

// ----------GroupHandler 方法----------
// 创建群聊
func (h *GroupHandler) Create(c *fiber.Ctx) error {
	type createGroupRequest struct {
		Name      string `json:"name"`
		Avatar    string `json:"avatar"`
		MemberIDs []uint `json:"member_ids"`
	}

	var req createGroupRequest
	if err := c.BodyParser(&req); err != nil || req.Name == "" {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	group, err := h.groupService.CreateGroup(c.Context(), userID, req.Name, req.Avatar, req.MemberIDs)
	if err != nil {
		return groupServiceError(c, err)
	}
	return response.Success(c, group, "创建群聊成功")
}

// 邀请加入群聊
func (h *GroupHandler) AddMembers(c *fiber.Ctx) error {
	type addGroupMembersRequest struct {
		GroupID   uint   `json:"group_id"`
		MemberIDs []uint `json:"member_ids"`
	}

	var req addGroupMembersRequest
	if err := c.BodyParser(&req); err != nil || req.GroupID == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	members, err := h.groupService.AddMembers(c.Context(), userID, req.GroupID, req.MemberIDs)
	if err != nil {
		return groupServiceError(c, err)
	}
	return response.Success(c, members, "拉群成功")
}

// 移除群成员
func (h *GroupHandler) RemoveMember(c *fiber.Ctx) error {
	type removeGroupMemberRequest struct {
		GroupID  uint `json:"group_id"`
		MemberID uint `json:"member_id"`
	}

	var req removeGroupMemberRequest
	if err := c.BodyParser(&req); err != nil || req.GroupID == 0 || req.MemberID == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.groupService.RemoveMember(c.Context(), userID, req.GroupID, req.MemberID)
	if err != nil {
		return groupServiceError(c, err)
	}
	return response.Success(c, nil, "踢出群成员成功")
}

// 退出群聊
func (h *GroupHandler) Leave(c *fiber.Ctx) error {
	type leaveGroupRequest struct {
		GroupID uint `json:"group_id"`
	}

	var req leaveGroupRequest
	if err := c.BodyParser(&req); err != nil || req.GroupID == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.groupService.LeaveGroup(c.Context(), userID, req.GroupID)
	if err != nil {
		return groupServiceError(c, err)
	}
	return response.Success(c, nil, "退出群聊成功")
}

func (h *GroupHandler) Delete(c *fiber.Ctx) error {
	type deleteGroupRequest struct {
		GroupID uint `json:"group_id"`
	}

	var req deleteGroupRequest
	if err := c.BodyParser(&req); err != nil || req.GroupID == 0 {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.groupService.DeleteGroup(c.Context(), userID, req.GroupID)
	if err != nil {
		return groupServiceError(c, err)
	}

	return response.Success(c, nil, "删除群聊成功")
}

// 获取群聊列表
func (h *GroupHandler) GetGroups(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	groups, err := h.groupService.GetGroups(c.Context(), userID)
	if err != nil {
		return groupServiceError(c, err)
	}
	return response.Success(c, groups, "获取群聊列表成功")
}

// 获取群成员列表
func (h *GroupHandler) GetMembers(c *fiber.Ctx) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	groupID, err := strconv.ParseUint(c.Query("group_id"), 10, 64)
	if err != nil || groupID == 0 {
		return response.Error(c, http.StatusBadRequest, "无效的group_id")
	}

	members, err := h.groupService.GetMembers(c.Context(), userID, uint(groupID))
	if err != nil {
		return groupServiceError(c, err)
	}
	return response.Success(c, members, "获取群成员成功")
}
