package handler

import (
	"net/http"

	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gofiber/fiber/v2"
)

type RTCHandler struct {
	rtcService service.RTCService
}

// ----------RTC handler 构造函数----------
func NewRTCHandler(rtcService service.RTCService) *RTCHandler {
	return &RTCHandler{rtcService: rtcService}
}

// ----------RTC handler 私有方法----------
// 处理RTC服务错误
func (h *RTCHandler) handleServiceError(c *fiber.Ctx, err error, fallback string) error {
	if serviceErr, ok := err.(*service.RTCServiceError); ok {
		return response.Error(c, serviceErr.HTTPCode, serviceErr.Message)
	}
	return response.Error(c, http.StatusInternalServerError, fallback)
}

// ----------RTC handler 方法----------
// 发起呼叫
func (h *RTCHandler) Invite(c *fiber.Ctx) error {
	type inviteRequest struct {
		PeerID   uint   `json:"peer_id"`
		GroupID  uint   `json:"group_id"`
		CallType string `json:"call_type"`
	}
	var req inviteRequest
	if err := c.BodyParser(&req); err != nil || req.CallType == "" {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}
	data, err := h.rtcService.Invite(c.Context(), userID, service.RTCInviteRequest{
		PeerID:   req.PeerID,
		GroupID:  req.GroupID,
		CallType: req.CallType,
	})
	if err != nil {
		return h.handleServiceError(c, err, "发起呼叫失败")
	}

	return response.Success(c, data, "发起呼叫成功")
}

// 接听呼叫
func (h *RTCHandler) Accept(c *fiber.Ctx) error {
	type acceptRequest struct {
		CallID string `json:"call_id"`
	}

	var req acceptRequest
	if err := c.BodyParser(&req); err != nil || req.CallID == "" {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	data, err := h.rtcService.Accept(c.Context(), userID, service.RTCAcceptRequest{CallID: req.CallID})
	if err != nil {
		return h.handleServiceError(c, err, "接听失败")
	}

	return response.Success(c, data, "接听成功")
}

// 拒绝呼叫
func (h *RTCHandler) Reject(c *fiber.Ctx) error {
	type rejectRequest struct {
		CallID string `json:"call_id"`
		Reason string `json:"reason"`
	}

	var req rejectRequest
	if err := c.BodyParser(&req); err != nil || req.CallID == "" {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.rtcService.Reject(c.Context(), userID, service.RTCRejectRequest{CallID: req.CallID, Reason: req.Reason})
	if err != nil {
		return h.handleServiceError(c, err, "拒绝失败")
	}

	return response.Success(c, nil, "拒绝成功")
}

// 取消呼叫
func (h *RTCHandler) Cancel(c *fiber.Ctx) error {
	type cancelRequest struct {
		CallID string `json:"call_id"`
	}

	var req cancelRequest
	if err := c.BodyParser(&req); err != nil || req.CallID == "" {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.rtcService.Cancel(c.Context(), userID, service.RTCCallIDRequest{CallID: req.CallID})
	if err != nil {
		return h.handleServiceError(c, err, "取消失败")
	}

	return response.Success(c, nil, "取消成功")
}

// 挂断呼叫
func (h *RTCHandler) Hangup(c *fiber.Ctx) error {
	type hangupRequest struct {
		CallID string `json:"call_id"`
	}

	var req hangupRequest
	if err := c.BodyParser(&req); err != nil || req.CallID == "" {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	err = h.rtcService.Hangup(c.Context(), userID, service.RTCCallIDRequest{CallID: req.CallID})
	if err != nil {
		return h.handleServiceError(c, err, "挂断失败")
	}

	return response.Success(c, nil, "挂断成功")
}

// 获取RTC Token
func (h *RTCHandler) GetToken(c *fiber.Ctx) error {
	type rtcTokenRequest struct {
		CallID   string `json:"call_id"`
		RoomID   string `json:"room_id"`
		CallType string `json:"call_type"`
		PeerID   uint   `json:"peer_id"`
		GroupID  uint   `json:"group_id"`
	}

	var req rtcTokenRequest
	if err := c.BodyParser(&req); err != nil || req.CallID == "" || req.CallType == "" {
		return response.Error(c, http.StatusBadRequest, "参数错误")
	}

	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	data, err := h.rtcService.IssueToken(c.Context(), userID, service.RTCIssueTokenRequest{
		CallID:   req.CallID,
		RoomID:   req.RoomID,
		CallType: req.CallType,
		PeerID:   req.PeerID,
		GroupID:  req.GroupID,
	})
	if err != nil {
		return h.handleServiceError(c, err, "生成 RTC Token 失败")
	}

	return response.Success(c, data, "获取 RTC Token 成功")
}
