package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
)

type fakeRTCService struct {
	inviteFn       func(context.Context, uint, service.RTCInviteRequest) (*service.RTCInviteResponse, error)
	acceptFn       func(context.Context, uint, service.RTCAcceptRequest) (*service.RTCCallActionResponse, error)
	tokenFn        func(context.Context, uint, service.RTCIssueTokenRequest) (*service.RTCTokenPayload, error)
	disconnectedFn func(context.Context, uint) error
}

func (s *fakeRTCService) Invite(ctx context.Context, userID uint, req service.RTCInviteRequest) (*service.RTCInviteResponse, error) {
	if s.inviteFn != nil {
		return s.inviteFn(ctx, userID, req)
	}
	return &service.RTCInviteResponse{CallID: "call-1", RoomID: "room-1", CallType: req.CallType, PeerID: req.PeerID, GroupID: req.GroupID}, nil
}

func (s *fakeRTCService) Accept(ctx context.Context, userID uint, req service.RTCAcceptRequest) (*service.RTCCallActionResponse, error) {
	if s.acceptFn != nil {
		return s.acceptFn(ctx, userID, req)
	}
	return &service.RTCCallActionResponse{CallID: req.CallID, RoomID: "room-1"}, nil
}

func (s *fakeRTCService) Reject(ctx context.Context, userID uint, req service.RTCRejectRequest) error {
	return nil
}

func (s *fakeRTCService) Cancel(ctx context.Context, userID uint, req service.RTCCallIDRequest) error {
	return nil
}

func (s *fakeRTCService) Hangup(ctx context.Context, userID uint, req service.RTCCallIDRequest) error {
	return nil
}

func (s *fakeRTCService) HandleParticipantDisconnected(ctx context.Context, userID uint) error {
	if s.disconnectedFn != nil {
		return s.disconnectedFn(ctx, userID)
	}
	return nil
}

func (s *fakeRTCService) IssueToken(ctx context.Context, userID uint, req service.RTCIssueTokenRequest) (*service.RTCTokenPayload, error) {
	if s.tokenFn != nil {
		return s.tokenFn(ctx, userID, req)
	}
	return &service.RTCTokenPayload{URL: "http://localhost:7880", RoomID: req.RoomID, UID: "7", Token: "token"}, nil
}

func TestRTCHandlerInviteAndToken(t *testing.T) {
	handler := NewRTCHandler(&fakeRTCService{})
	app := fiber.New()
	app.Post("/invite", withUser(7, handler.Invite))
	app.Post("/token", withUser(7, handler.GetToken))

	status, payload := testResponse(t, app, testJSONRequest("POST", "/invite", map[string]any{"peer_id": 2, "call_type": "video"}))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.Success {
		t.Fatalf("invite response = status %d payload %#v, want success", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/invite", map[string]any{"peer_id": 2}))
	if status != http.StatusBadRequest || payload["message"] != "参数错误" {
		t.Fatalf("bad invite response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/token", map[string]any{"call_id": "call-1", "room_id": "room-1", "call_type": "video"}))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.Success {
		t.Fatalf("token response = status %d payload %#v, want success", status, payload)
	}
}

func TestRTCHandlerMapsServiceErrors(t *testing.T) {
	handler := NewRTCHandler(&fakeRTCService{
		acceptFn: func(ctx context.Context, userID uint, req service.RTCAcceptRequest) (*service.RTCCallActionResponse, error) {
			return nil, &service.RTCServiceError{HTTPCode: http.StatusConflict, Message: "busy"}
		},
		tokenFn: func(ctx context.Context, userID uint, req service.RTCIssueTokenRequest) (*service.RTCTokenPayload, error) {
			return nil, assertErr("boom")
		},
	})
	app := fiber.New()
	app.Post("/accept", withUser(7, handler.Accept))
	app.Post("/token", withUser(7, handler.GetToken))

	status, payload := testResponse(t, app, testJSONRequest("POST", "/accept", map[string]any{"call_id": "call-1"}))
	if status != http.StatusConflict || payload["message"] != "busy" {
		t.Fatalf("service error response = status %d payload %#v, want busy conflict", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/token", map[string]any{"call_id": "call-1", "call_type": "video"}))
	if status != http.StatusInternalServerError || payload["message"] != "生成 RTC Token 失败" {
		t.Fatalf("fallback error response = status %d payload %#v", status, payload)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
