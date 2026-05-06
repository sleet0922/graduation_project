package handler

import (
	"net/http"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

type GroupHandler struct {
	groupService service.GroupService
}

func NewGroupHandler(groupService service.GroupService) *GroupHandler {
	return &GroupHandler{groupService: groupService}
}

// ----------GroupHandler 方法----------
// 创建群聊
func (h *GroupHandler) Create(c *gin.Context) {
	type createGroupRequest struct {
		Name      string `json:"name" binding:"required"`
		Avatar    string `json:"avatar"`
		MemberIDs []uint `json:"member_ids"`
	}

	var req createGroupRequest
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

	group, err := h.groupService.CreateGroup(c.Request.Context(), userID, req.Name, req.Avatar, req.MemberIDs)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, group, "创建群聊成功")
}

// 邀请加入群聊
func (h *GroupHandler) AddMembers(c *gin.Context) {
	type addGroupMembersRequest struct {
		GroupID   uint   `json:"group_id" binding:"required"`
		MemberIDs []uint `json:"member_ids"`
	}

	var req addGroupMembersRequest
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

	members, err := h.groupService.AddMembers(c.Request.Context(), userID, req.GroupID, req.MemberIDs)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, members, "拉群成功")
}

// 移除群成员
func (h *GroupHandler) RemoveMember(c *gin.Context) {
	type removeGroupMemberRequest struct {
		GroupID  uint `json:"group_id" binding:"required"`
		MemberID uint `json:"member_id" binding:"required"`
	}

	var req removeGroupMemberRequest
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

	err = h.groupService.RemoveMember(c.Request.Context(), userID, req.GroupID, req.MemberID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, nil, "踢出群成员成功")
}

// 退出群聊
func (h *GroupHandler) Leave(c *gin.Context) {
	type leaveGroupRequest struct {
		GroupID uint `json:"group_id" binding:"required"`
	}

	var req leaveGroupRequest
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

	err = h.groupService.LeaveGroup(c.Request.Context(), userID, req.GroupID)
	if err != nil {
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

	err = h.groupService.DeleteGroup(c.Request.Context(), userID, req.GroupID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.Success(c, nil, "删除群聊成功")
}

// 获取群聊列表
func (h *GroupHandler) GetGroups(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	groups, err := h.groupService.GetGroups(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "获取群聊列表失败")
		return
	}
	response.Success(c, groups, "获取群聊列表成功")
}

// 获取群成员列表
func (h *GroupHandler) GetMembers(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	groupID, err := strconv.ParseUint(c.Query("group_id"), 10, 64)
	if err != nil || groupID == 0 {
		response.Error(c, http.StatusBadRequest, "无效的group_id")
		return
	}

	members, err := h.groupService.GetMembers(c.Request.Context(), userID, uint(groupID))
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, members, "获取群成员成功")
}
