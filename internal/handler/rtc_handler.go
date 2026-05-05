package handler

import (
	"net/http"

	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gin-gonic/gin"
)

type RTCHandler struct {
	rtcService service.RTCService
}

func NewRTCHandler(rtcService service.RTCService) *RTCHandler {
	return &RTCHandler{rtcService: rtcService}
}

// ----------RTC handler 私有方法----------
func (h *RTCHandler) handleServiceError(c *gin.Context, err error, fallback string) {
	if serviceErr, ok := err.(*service.RTCServiceError); ok {
		response.Error(c, serviceErr.HTTPCode, serviceErr.Message)
		return
	}
	response.Error(c, http.StatusInternalServerError, fallback)
}

// ----------RTC handler 方法----------

func (h *RTCHandler) Invite(c *gin.Context) {
	type inviteRequest struct {
		PeerID   uint   `json:"peer_id"`
		GroupID  uint   `json:"group_id"`
		CallType string `json:"call_type" binding:"required"`
	}
	var req inviteRequest
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

	data, err := h.rtcService.Invite(c.Request.Context(), userID, service.RTCInviteRequest{
		PeerID:   req.PeerID,
		GroupID:  req.GroupID,
		CallType: req.CallType,
	})
	if err != nil {
		h.handleServiceError(c, err, "发起呼叫失败")
		return
	}

	response.Success(c, data, "发起呼叫成功")
}

func (h *RTCHandler) Accept(c *gin.Context) {
	type acceptRequest struct {
		CallID string `json:"call_id" binding:"required"`
	}

	var req acceptRequest
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

	data, err := h.rtcService.Accept(c.Request.Context(), userID, service.RTCAcceptRequest{CallID: req.CallID})
	if err != nil {
		h.handleServiceError(c, err, "接听失败")
		return
	}

	response.Success(c, data, "接听成功")
}

func (h *RTCHandler) Reject(c *gin.Context) {
	type rejectRequest struct {
		CallID string `json:"call_id" binding:"required"`
		Reason string `json:"reason"`
	}

	var req rejectRequest
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

	err = h.rtcService.Reject(c.Request.Context(), userID, service.RTCRejectRequest{CallID: req.CallID, Reason: req.Reason})
	if err != nil {
		h.handleServiceError(c, err, "拒绝失败")
		return
	}

	response.Success(c, nil, "拒绝成功")
}

func (h *RTCHandler) Cancel(c *gin.Context) {
	type cancelRequest struct {
		CallID string `json:"call_id" binding:"required"`
	}

	var req cancelRequest
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

	err = h.rtcService.Cancel(c.Request.Context(), userID, service.RTCCallIDRequest{CallID: req.CallID})
	if err != nil {
		h.handleServiceError(c, err, "取消失败")
		return
	}

	response.Success(c, nil, "取消成功")
}

func (h *RTCHandler) Hangup(c *gin.Context) {
	type hangupRequest struct {
		CallID string `json:"call_id" binding:"required"`
	}

	var req hangupRequest
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

	err = h.rtcService.Hangup(c.Request.Context(), userID, service.RTCCallIDRequest{CallID: req.CallID})
	if err != nil {
		h.handleServiceError(c, err, "挂断失败")
		return
	}

	response.Success(c, nil, "挂断成功")
}

func (h *RTCHandler) GetToken(c *gin.Context) {
	type rtcTokenRequest struct {
		CallID   string `json:"call_id" binding:"required"`
		RoomID   string `json:"room_id"`
		CallType string `json:"call_type" binding:"required"`
		PeerID   uint   `json:"peer_id"`
		GroupID  uint   `json:"group_id"`
	}

	var req rtcTokenRequest
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

	data, err := h.rtcService.IssueToken(c.Request.Context(), userID, service.RTCIssueTokenRequest{
		CallID:   req.CallID,
		RoomID:   req.RoomID,
		CallType: req.CallType,
		PeerID:   req.PeerID,
		GroupID:  req.GroupID,
	})
	if err != nil {
		h.handleServiceError(c, err, "生成 RTC Token 失败")
		return
	}

	response.Success(c, data, "获取 RTC Token 成功")
}
