package handler

import (
	"fmt"
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	groupService service.GroupService
	chatService  service.ChatService
}

func NewGroupHandler(groupService service.GroupService, chatService service.ChatService) *GroupHandler {
	return &GroupHandler{groupService: groupService, chatService: chatService}
}

func (h *GroupHandler) Create(c *gin.Context) {
	type createGroupRequest struct {
		Name      string `json:"name" binding:"required"`
		Avatar    string `json:"avatar"`
		MemberIDs []uint `json:"member_ids"`
	}

	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	group, err := h.groupService.CreateGroup(userID, req.Name, req.Avatar, req.MemberIDs)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, group, "创建群聊成功")
}

func (h *GroupHandler) AddMembers(c *gin.Context) {
	type addGroupMembersRequest struct {
		GroupID   uint   `json:"group_id" binding:"required"`
		MemberIDs []uint `json:"member_ids"`
	}

	var req addGroupMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	members, err := h.groupService.AddMembers(userID, req.GroupID, req.MemberIDs)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, members, "拉群成功")
}

func (h *GroupHandler) RemoveMember(c *gin.Context) {
	type removeGroupMemberRequest struct {
		GroupID  uint `json:"group_id" binding:"required"`
		MemberID uint `json:"member_id" binding:"required"`
	}

	var req removeGroupMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	if err := h.groupService.RemoveMember(userID, req.GroupID, req.MemberID); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, nil, "踢出群成员成功")
}

func (h *GroupHandler) Leave(c *gin.Context) {
	type leaveGroupRequest struct {
		GroupID uint `json:"group_id" binding:"required"`
	}

	var req leaveGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	if err := h.groupService.LeaveGroup(userID, req.GroupID); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, nil, "退出群聊成功")
}

func (h *GroupHandler) Delete(c *gin.Context) {
	type deleteGroupRequest struct {
		GroupID uint `json:"group_id" binding:"required"`
	}

	var req deleteGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	members, _ := h.groupService.GetMembers(userID, req.GroupID)

	if err := h.groupService.DeleteGroup(userID, req.GroupID); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	if h.chatService != nil && len(members) > 0 {
		var memberIDs []uint
		for _, m := range members {
			memberIDs = append(memberIDs, m.UserID)
		}
		h.chatService.BroadcastGroupDissolved(req.GroupID, memberIDs)
	}

	response.Success(c, nil, "删除群聊成功")
}

func (h *GroupHandler) GetGroups(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	groups, err := h.groupService.GetGroups(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "获取群聊列表失败")
		return
	}
	response.Success(c, groups, "获取群聊列表成功")
}

func (h *GroupHandler) GetMembers(c *gin.Context) {
	userID, err := h.getUserID(c)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, "未获取到用户信息")
		return
	}

	groupID, err := strconv.ParseUint(c.Query("group_id"), 10, 32)
	if err != nil || groupID == 0 {
		response.Error(c, http.StatusBadRequest, "无效的group_id")
		return
	}

	members, err := h.groupService.GetMembers(userID, uint(groupID))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, members, "获取群成员成功")
}

func (h *GroupHandler) getUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, fmt.Errorf("user_id not found in context")
	}
	return userID.(uint), nil
}
