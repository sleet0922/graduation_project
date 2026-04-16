package service

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/rtc"

	"gorm.io/gorm"
)

const (
	defaultRTCTokenExpire = 2 * time.Hour
	defaultRTCInviteTTL   = 45 * time.Second

	rtcCallStatusPending  = "pending"
	rtcCallStatusOngoing  = "ongoing"
	rtcCallStatusRejected = "rejected"
	rtcCallStatusCanceled = "canceled"
	rtcCallStatusEnded    = "ended"
	rtcCallStatusTimeout  = "timeout"
)

type RTCService interface {
	Invite(userID uint, req RTCInviteRequest) (*RTCInviteResponse, error)
	Accept(userID uint, req RTCAcceptRequest) (*RTCCallActionResponse, error)
	Reject(userID uint, req RTCRejectRequest) error
	Cancel(userID uint, req RTCCallIDRequest) error
	Hangup(userID uint, req RTCCallIDRequest) error
	IssueToken(userID uint, req RTCIssueTokenRequest) (*RTCTokenPayload, error)
}

type RTCInviteRequest struct {
	PeerID   uint
	GroupID  uint
	CallType string
}

type RTCAcceptRequest struct {
	CallID string
}

type RTCRejectRequest struct {
	CallID string
	Reason string
}

type RTCCallIDRequest struct {
	CallID string
}

type RTCIssueTokenRequest struct {
	CallID   string
	RoomID   string
	CallType string
	PeerID   uint
	GroupID  uint
}

type RTCInviteResponse struct {
	CallID   string `json:"call_id"`
	RoomID   string `json:"room_id"`
	CallType string `json:"call_type"`
	PeerID   uint   `json:"peer_id"`
	GroupID  uint   `json:"group_id"`
}

type RTCCallActionResponse struct {
	CallID string `json:"call_id"`
	RoomID string `json:"room_id"`
}

type RTCTokenPayload struct {
	AppID  string `json:"app_id"`
	RoomID string `json:"room_id"`
	UID    string `json:"uid"`
	Token  string `json:"token"`
}

type rtcCall struct {
	CallID      string
	RoomID      string
	CallType    string
	InitiatorID uint
	PeerID      uint
	GroupID     uint
	InviteeIDs  []uint
	AcceptedIDs map[uint]bool
	RejectedIDs map[uint]string
	Status      string
	CreatedAt   time.Time
}

type rtcService struct {
	userRepo       repo.UserRepository
	friendRepo     repo.FriendRepository
	groupRepo      repo.GroupRepository
	chatService    ChatService
	appID          string
	appKey         string
	tokenLifetime  time.Duration
	inviteTTL      time.Duration
	mu             sync.RWMutex
	sequence       uint64
	calls          map[string]*rtcCall
	activeCallByID map[uint]string
}

type RTCServiceError struct {
	HTTPCode int
	Message  string
}

func (e *RTCServiceError) Error() string {
	return e.Message
}

func NewRTCService(cfg *config.ViperConfig, userRepo repo.UserRepository, friendRepo repo.FriendRepository, groupRepo repo.GroupRepository, chatService ChatService) RTCService {
	return &rtcService{
		userRepo:       userRepo,
		friendRepo:     friendRepo,
		groupRepo:      groupRepo,
		chatService:    chatService,
		appID:          strings.TrimSpace(cfg.RTC.AppID),
		appKey:         strings.TrimSpace(cfg.RTC.AppKey),
		tokenLifetime:  loadRTCTokenLifetime(cfg.RTC.TokenExpireSeconds),
		inviteTTL:      defaultRTCInviteTTL,
		calls:          make(map[string]*rtcCall),
		activeCallByID: make(map[uint]string),
	}
}

func (s *rtcService) Invite(userID uint, req RTCInviteRequest) (*RTCInviteResponse, error) {
	callType, err := normalizeCallType(req.CallType)
	if err != nil {
		return nil, err
	}
	if (req.PeerID == 0 && req.GroupID == 0) || (req.PeerID != 0 && req.GroupID != 0) {
		return nil, &RTCServiceError{HTTPCode: 400, Message: "peer_id 和 group_id 必须二选一"}
	}

	inviter, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, s.mapRecordError(err, "用户不存在", "获取用户信息失败")
	}

	var inviteeIDs []uint
	if req.PeerID != 0 {
		if req.PeerID == userID {
			return nil, &RTCServiceError{HTTPCode: 403, Message: "无权限发起该通话"}
		}
		if _, err := s.userRepo.GetByID(req.PeerID); err != nil {
			return nil, s.mapRecordError(err, "呼叫对象不存在", "校验呼叫对象失败")
		}
		if !s.friendRepo.CheckFriendship(userID, req.PeerID) {
			return nil, &RTCServiceError{HTTPCode: 403, Message: "无权限发起该通话"}
		}
		targetConnIDs := s.connectionIDs(req.PeerID)
		logger.Info("rtc invite target status", "target_user_id", req.PeerID, "online", len(targetConnIDs) > 0, "connection_ids", targetConnIDs)
		if len(targetConnIDs) == 0 {
			return nil, &RTCServiceError{HTTPCode: 409, Message: "对方当前不在线"}
		}
		inviteeIDs = []uint{req.PeerID}
	} else {
		group, err := s.groupRepo.GetByID(req.GroupID)
		if err != nil {
			return nil, s.mapRecordError(err, "群聊不存在", "校验群聊失败")
		}
		if group == nil {
			return nil, &RTCServiceError{HTTPCode: 404, Message: "群聊不存在"}
		}
		if !s.groupRepo.IsMember(req.GroupID, userID) {
			return nil, &RTCServiceError{HTTPCode: 403, Message: "无权限发起该通话"}
		}
		members, err := s.groupRepo.GetMembersByGroupID(req.GroupID)
		if err != nil {
			return nil, &RTCServiceError{HTTPCode: 500, Message: "获取群成员失败"}
		}
		inviteeIDs = make([]uint, 0, len(members))
		for _, member := range members {
			if member.UserID == userID {
				continue
			}
			memberConnIDs := s.connectionIDs(member.UserID)
			logger.Info("rtc invite group member status", "group_id", req.GroupID, "member_user_id", member.UserID, "online", len(memberConnIDs) > 0, "connection_ids", memberConnIDs)
			if len(memberConnIDs) == 0 {
				continue
			}
			inviteeIDs = append(inviteeIDs, member.UserID)
		}
		if len(inviteeIDs) == 0 {
			return nil, &RTCServiceError{HTTPCode: 409, Message: "群聊当前没有在线成员"}
		}
	}

	err = s.ensureUsersAvailable(append([]uint{userID}, inviteeIDs...))
	if err != nil {
		return nil, err
	}

	callID, roomID := s.nextCallIdentifiers()
	call := &rtcCall{
		CallID:      callID,
		RoomID:      roomID,
		CallType:    callType,
		InitiatorID: userID,
		PeerID:      req.PeerID,
		GroupID:     req.GroupID,
		InviteeIDs:  append([]uint(nil), inviteeIDs...),
		AcceptedIDs: map[uint]bool{userID: true},
		RejectedIDs: make(map[uint]string),
		Status:      rtcCallStatusPending,
		CreatedAt:   time.Now(),
	}

	s.mu.Lock()
	s.calls[callID] = call
	for _, busyUserID := range append([]uint{userID}, inviteeIDs...) {
		s.activeCallByID[busyUserID] = callID
	}
	s.mu.Unlock()

	invitePayload := &RTCInviteResponse{
		CallID:   callID,
		RoomID:   roomID,
		CallType: callType,
		PeerID:   req.PeerID,
		GroupID:  req.GroupID,
	}
	logger.Info("rtc invite created", "call_id", callID, "room_id", roomID, "from_user_id", userID, "peer_id", req.PeerID, "group_id", req.GroupID, "call_type", callType)
	successCount := s.pushInviteEvents(call, inviter)
	if successCount == 0 {
		s.releaseCall(callID)
		if req.PeerID != 0 {
			return nil, &RTCServiceError{HTTPCode: 409, Message: "对方当前不在线"}
		}
		return nil, &RTCServiceError{HTTPCode: 409, Message: "群聊当前没有在线成员"}
	}
	go s.scheduleTimeout(callID)

	return invitePayload, nil
}

func (s *rtcService) Accept(userID uint, req RTCAcceptRequest) (*RTCCallActionResponse, error) {
	callID := strings.TrimSpace(req.CallID)
	if callID == "" {
		return nil, &RTCServiceError{HTTPCode: 400, Message: "call_id 不能为空"}
	}

	s.mu.Lock()
	call, ok := s.calls[callID]
	if !ok {
		s.mu.Unlock()
		return nil, &RTCServiceError{HTTPCode: 404, Message: "通话不存在"}
	}
	if !containsUint(call.InviteeIDs, userID) {
		s.mu.Unlock()
		return nil, &RTCServiceError{HTTPCode: 403, Message: "无权限操作该通话"}
	}
	if isTerminalStatus(call.Status) {
		s.mu.Unlock()
		return nil, &RTCServiceError{HTTPCode: 400, Message: "通话已结束"}
	}
	if call.AcceptedIDs[userID] {
		roomID := call.RoomID
		s.mu.Unlock()
		return &RTCCallActionResponse{CallID: call.CallID, RoomID: roomID}, nil
	}
	call.AcceptedIDs[userID] = true
	s.activeCallByID[userID] = callID
	delete(call.RejectedIDs, userID)
	call.Status = rtcCallStatusOngoing
	roomID := call.RoomID
	notifyIDs := s.otherParticipantIDs(call, userID)
	s.mu.Unlock()

	logger.Info("rtc call accepted", "call_id", callID, "user_id", userID, "room_id", roomID)
	s.pushSystemEvent(notifyIDs, map[string]any{
		"type":    "rtc_accept",
		"call_id": callID,
		"room_id": roomID,
	})

	return &RTCCallActionResponse{CallID: callID, RoomID: roomID}, nil
}

func (s *rtcService) Reject(userID uint, req RTCRejectRequest) error {
	callID := strings.TrimSpace(req.CallID)
	if callID == "" {
		return &RTCServiceError{HTTPCode: 400, Message: "call_id 不能为空"}
	}
	reason := normalizeRejectReason(req.Reason)

	s.mu.Lock()
	call, ok := s.calls[callID]
	if !ok {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 404, Message: "通话不存在"}
	}
	if !containsUint(call.InviteeIDs, userID) {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 403, Message: "无权限操作该通话"}
	}
	if isTerminalStatus(call.Status) {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 400, Message: "通话已结束"}
	}
	if call.AcceptedIDs[userID] {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 400, Message: "通话已接听，请使用挂断接口"}
	}

	call.RejectedIDs[userID] = reason
	delete(s.activeCallByID, userID)

	notifyInitiator := []uint{call.InitiatorID}
	shouldReleaseAll := len(call.RejectedIDs) == len(call.InviteeIDs)
	if shouldReleaseAll {
		call.Status = rtcCallStatusRejected
		for _, participantID := range s.participantIDs(call) {
			delete(s.activeCallByID, participantID)
		}
	}
	s.mu.Unlock()

	eventType := "rtc_reject"
	if reason == "busy" {
		eventType = "rtc_busy"
	}
	logger.Info("rtc call rejected", "call_id", callID, "user_id", userID, "reason", reason, "release_all", shouldReleaseAll)
	s.pushSystemEvent(notifyInitiator, map[string]any{
		"type":    eventType,
		"call_id": callID,
		"reason":  reason,
	})
	return nil
}

func (s *rtcService) Cancel(userID uint, req RTCCallIDRequest) error {
	callID := strings.TrimSpace(req.CallID)
	if callID == "" {
		return &RTCServiceError{HTTPCode: 400, Message: "call_id 不能为空"}
	}

	s.mu.Lock()
	call, ok := s.calls[callID]
	if !ok {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 404, Message: "通话不存在"}
	}
	if call.InitiatorID != userID {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 403, Message: "无权限操作该通话"}
	}
	if call.Status != rtcCallStatusPending {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 400, Message: "当前通话状态不允许取消"}
	}
	call.Status = rtcCallStatusCanceled
	notifyIDs := append([]uint(nil), call.InviteeIDs...)
	for _, participantID := range s.participantIDs(call) {
		delete(s.activeCallByID, participantID)
	}
	s.mu.Unlock()

	logger.Info("rtc call canceled", "call_id", callID, "user_id", userID)
	s.pushSystemEvent(notifyIDs, map[string]any{
		"type":    "rtc_cancel",
		"call_id": callID,
	})
	return nil
}

func (s *rtcService) Hangup(userID uint, req RTCCallIDRequest) error {
	callID := strings.TrimSpace(req.CallID)
	if callID == "" {
		return &RTCServiceError{HTTPCode: 400, Message: "call_id 不能为空"}
	}

	s.mu.Lock()
	call, ok := s.calls[callID]
	if !ok {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 404, Message: "通话不存在"}
	}
	if !s.canHangup(call, userID) {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 403, Message: "无权限操作该通话"}
	}
	if isTerminalStatus(call.Status) {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 400, Message: "通话已结束"}
	}
	if call.Status == rtcCallStatusPending && call.InitiatorID == userID {
		s.mu.Unlock()
		return &RTCServiceError{HTTPCode: 400, Message: "未接通前请使用取消接口"}
	}
	call.Status = rtcCallStatusEnded
	notifyIDs := s.otherParticipantIDs(call, userID)
	for _, participantID := range s.participantIDs(call) {
		delete(s.activeCallByID, participantID)
	}
	s.mu.Unlock()

	logger.Info("rtc call hangup", "call_id", callID, "user_id", userID)
	s.pushSystemEvent(notifyIDs, map[string]any{
		"type":    "rtc_hangup",
		"call_id": callID,
	})
	return nil
}

func (s *rtcService) IssueToken(userID uint, req RTCIssueTokenRequest) (*RTCTokenPayload, error) {
	callType, err := normalizeCallType(req.CallType)
	if err != nil {
		return nil, err
	}
	callID := strings.TrimSpace(req.CallID)
	if callID == "" {
		return nil, &RTCServiceError{HTTPCode: 400, Message: "call_id 不能为空"}
	}
	if s.appID == "" || s.appKey == "" {
		return nil, &RTCServiceError{HTTPCode: 500, Message: "RTC 服务端未配置 AppId 或 AppKey"}
	}

	s.mu.RLock()
	call, ok := s.calls[callID]
	if !ok {
		s.mu.RUnlock()
		return nil, &RTCServiceError{HTTPCode: 404, Message: "通话不存在"}
	}
	if isTerminalStatus(call.Status) {
		s.mu.RUnlock()
		return nil, &RTCServiceError{HTTPCode: 400, Message: "通话已结束"}
	}
	if !containsUint(s.participantIDs(call), userID) {
		s.mu.RUnlock()
		return nil, &RTCServiceError{HTTPCode: 403, Message: "无权限发起该通话"}
	}
	if !call.AcceptedIDs[userID] {
		s.mu.RUnlock()
		return nil, &RTCServiceError{HTTPCode: 403, Message: "当前用户尚未加入该通话"}
	}
	roomID := call.RoomID
	storedType := call.CallType
	storedPeerID := call.PeerID
	storedGroupID := call.GroupID
	status := call.Status
	s.mu.RUnlock()

	if callType != storedType {
		return nil, &RTCServiceError{HTTPCode: 400, Message: "call_type 与当前通话不一致"}
	}
	if req.RoomID != "" && strings.TrimSpace(req.RoomID) != roomID {
		return nil, &RTCServiceError{HTTPCode: 400, Message: "room_id 与当前通话不一致"}
	}
	if storedPeerID != 0 && req.PeerID != 0 && storedPeerID != req.PeerID {
		return nil, &RTCServiceError{HTTPCode: 400, Message: "peer_id 与当前通话不一致"}
	}
	if storedGroupID != 0 && req.GroupID != 0 && storedGroupID != req.GroupID {
		return nil, &RTCServiceError{HTTPCode: 400, Message: "group_id 与当前通话不一致"}
	}

	uid := strconv.FormatUint(uint64(userID), 10)
	expireAt := time.Now().Add(s.tokenLifetime)
	token := rtc.NewAccessToken(s.appID, s.appKey, roomID, uid)
	token.ExpireTime(expireAt)
	token.AddPrivilege(rtc.PrivSubscribeStream, expireAt)
	token.AddPrivilege(rtc.PrivPublishStream, expireAt)
	logger.Info("rtc token issued", "call_id", callID, "user_id", userID, "room_id", roomID, "call_status", status)

	return &RTCTokenPayload{
		AppID:  s.appID,
		RoomID: roomID,
		UID:    uid,
		Token:  token.Serialize(),
	}, nil
}

func (s *rtcService) scheduleTimeout(callID string) {
	timer := time.NewTimer(s.inviteTTL)
	defer timer.Stop()
	<-timer.C

	s.mu.Lock()
	call, ok := s.calls[callID]
	if !ok || call.Status != rtcCallStatusPending {
		s.mu.Unlock()
		return
	}
	call.Status = rtcCallStatusTimeout
	notifyIDs := s.participantIDs(call)
	for _, participantID := range notifyIDs {
		delete(s.activeCallByID, participantID)
	}
	s.mu.Unlock()

	logger.Info("rtc call timeout", "call_id", callID, "participant_ids", notifyIDs)
	s.pushSystemEvent(notifyIDs, map[string]any{
		"type":    "rtc_timeout",
		"call_id": callID,
	})
}

func (s *rtcService) pushInviteEvents(call *rtcCall, inviter *model.User) int {
	successCount := 0
	for _, inviteeID := range call.InviteeIDs {
		payload := map[string]any{
			"type":         "rtc_invite",
			"call_id":      call.CallID,
			"room_id":      call.RoomID,
			"call_type":    call.CallType,
			"from_user_id": call.InitiatorID,
			"to_user_id":   inviteeID,
			"from_name":    inviter.Name,
			"avatar":       inviter.Avatar,
		}
		if call.GroupID != 0 {
			payload["group_id"] = call.GroupID
		}
		payloadJSON := mustJSON(payload)
		logger.Info("rtc invite payload", "call_id", call.CallID, "target_user_id", inviteeID, "payload", payloadJSON)
		results := s.pushSystemEvent([]uint{inviteeID}, payload)
		for _, result := range results {
			logger.Info("rtc invite push result",
				"call_id", call.CallID,
				"target_user_id", result.UserID,
				"online", result.Online,
				"connection_ids", result.ConnectionIDs,
				"successful_connection_ids", result.SuccessfulConnIDs,
				"failed_connection_ids", result.FailedConnIDs,
				"success", result.SuccessfulPushCount > 0,
				"errors", result.ErrorMessages,
			)
			if result.SuccessfulPushCount > 0 {
				successCount++
			}
		}
	}
	return successCount
}

func (s *rtcService) pushSystemEvent(userIDs []uint, payload any) []SystemPushResult {
	if s.chatService == nil || len(userIDs) == 0 {
		return nil
	}
	return s.chatService.PushSystemEvent(userIDs, payload)
}

func (s *rtcService) ensureUsersAvailable(userIDs []uint) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, userID := range userIDs {
		if activeCallID, busy := s.activeCallByID[userID]; busy {
			logger.Info("rtc invite conflict", "user_id", userID, "active_call_id", activeCallID)
			return &RTCServiceError{HTTPCode: 409, Message: "目标用户忙线中"}
		}
	}
	return nil
}

func (s *rtcService) connectionIDs(userID uint) []string {
	if s.chatService == nil {
		return nil
	}
	return s.chatService.GetConnectionIDs(userID)
}

func (s *rtcService) canHangup(call *rtcCall, userID uint) bool {
	if call.InitiatorID == userID {
		return true
	}
	return call.AcceptedIDs[userID]
}

func (s *rtcService) participantIDs(call *rtcCall) []uint {
	userIDs := make([]uint, 0, len(call.InviteeIDs)+1)
	userIDs = append(userIDs, call.InitiatorID)
	userIDs = append(userIDs, call.InviteeIDs...)
	return userIDs
}

func (s *rtcService) otherParticipantIDs(call *rtcCall, excludeID uint) []uint {
	participants := s.participantIDs(call)
	result := make([]uint, 0, len(participants))
	for _, userID := range participants {
		if userID == excludeID {
			continue
		}
		result = append(result, userID)
	}
	return result
}

func (s *rtcService) nextCallIdentifiers() (string, string) {
	seq := atomic.AddUint64(&s.sequence, 1) % 1000
	stamp := time.Now().Format("20060102150405")
	suffix := fmt.Sprintf("%s%03d", stamp, seq)
	return "call_" + suffix, "rtc_room_" + suffix
}

func (s *rtcService) mapRecordError(err error, notFoundMsg, internalMsg string) error {
	if err == nil {
		return nil
	}
	if err == gorm.ErrRecordNotFound {
		return &RTCServiceError{HTTPCode: 404, Message: notFoundMsg}
	}
	return &RTCServiceError{HTTPCode: 500, Message: internalMsg}
}

func normalizeCallType(callType string) (string, error) {
	value := strings.ToLower(strings.TrimSpace(callType))
	if value != "video" && value != "voice" {
		return "", &RTCServiceError{HTTPCode: 400, Message: "call_type 仅支持 video 或 voice"}
	}
	return value, nil
}

func normalizeRejectReason(reason string) string {
	value := strings.ToLower(strings.TrimSpace(reason))
	if value == "busy" {
		return "busy"
	}
	return "rejected"
}

func containsUint(values []uint, target uint) bool {
	return slices.Contains(values, target)
}

func isTerminalStatus(status string) bool {
	switch status {
	case rtcCallStatusRejected, rtcCallStatusCanceled, rtcCallStatusEnded, rtcCallStatusTimeout:
		return true
	default:
		return false
	}
}

func loadRTCTokenLifetime(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultRTCTokenExpire
	}
	return time.Duration(seconds) * time.Second
}

func (s *rtcService) releaseCall(callID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	call, ok := s.calls[callID]
	if !ok {
		return
	}
	for _, participantID := range s.participantIDs(call) {
		delete(s.activeCallByID, participantID)
	}
	delete(s.calls, callID)
	logger.Info("rtc call released", "call_id", callID)
}

func mustJSON(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("marshal payload failed: %v", err)
	}
	return string(data)
}
