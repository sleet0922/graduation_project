package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRTCServiceInviteAcceptIssueTokenAndHangup(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepo(
		testUser(1, "1", ""),
		testUser(2, "2", ""),
	)
	users.byID[1].Name = "Caller"
	users.byID[2].Name = "Peer"
	friends := newFakeFriendRepo()
	friends.friendships[[2]uint{1, 2}] = true
	chat := NewChatService(friends, newFakeGroupRepo())
	var invitePayloads []any
	chat.RegisterConnection(ctx, 2, nil, func(payload any) error {
		invitePayloads = append(invitePayloads, payload)
		return nil
	}, nil)
	rtcSvc := NewRTCService("http://localhost:7880", "api-key", "api-secret", time.Hour, users, friends, newFakeGroupRepo(), chat)

	invite, err := rtcSvc.Invite(ctx, 1, RTCInviteRequest{PeerID: 2, CallType: "video"})
	if err != nil {
		t.Fatalf("Invite failed: %v", err)
	}
	if invite.CallID == "" || invite.RoomID == "" || invite.CallType != "video" {
		t.Fatalf("invite = %#v, want identifiers and video type", invite)
	}
	if len(invitePayloads) != 1 {
		t.Fatalf("invite payload count = %d, want 1", len(invitePayloads))
	}

	accepted, err := rtcSvc.Accept(ctx, 2, RTCAcceptRequest{CallID: invite.CallID})
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}
	if accepted.RoomID != invite.RoomID {
		t.Fatalf("accepted room = %q, want %q", accepted.RoomID, invite.RoomID)
	}

	token, err := rtcSvc.IssueToken(ctx, 2, RTCIssueTokenRequest{CallID: invite.CallID, RoomID: invite.RoomID, CallType: "video", PeerID: 2})
	if err != nil {
		t.Fatalf("IssueToken failed: %v", err)
	}
	if token.URL != "http://localhost:7880" || token.UID != "2" || token.Token == "" {
		t.Fatalf("token = %#v, want LiveKit URL, uid 2 and serialized token", token)
	}

	if err := rtcSvc.Hangup(ctx, 2, RTCCallIDRequest{CallID: invite.CallID}); err != nil {
		t.Fatalf("Hangup failed: %v", err)
	}
}

func TestRTCServiceInviteValidation(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepo(
		testUser(1, "1", ""),
		testUser(2, "2", ""),
	)
	friends := newFakeFriendRepo()
	chat := NewChatService(friends, newFakeGroupRepo())
	rtcSvc := NewRTCService("http://localhost:7880", "api-key", "api-secret", time.Hour, users, friends, newFakeGroupRepo(), chat)

	tests := []struct {
		name string
		req  RTCInviteRequest
		code int
	}{
		{name: "bad call type", req: RTCInviteRequest{PeerID: 2, CallType: "screen"}, code: 400},
		{name: "no target", req: RTCInviteRequest{CallType: "video"}, code: 400},
		{name: "two targets", req: RTCInviteRequest{PeerID: 2, GroupID: 1, CallType: "video"}, code: 400},
		{name: "self target", req: RTCInviteRequest{PeerID: 1, CallType: "video"}, code: 403},
		{name: "not friends", req: RTCInviteRequest{PeerID: 2, CallType: "video"}, code: 403},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := rtcSvc.Invite(ctx, 1, tt.req)
			var serviceErr *RTCServiceError
			if !errors.As(err, &serviceErr) {
				t.Fatalf("Invite error = %v, want RTCServiceError", err)
			}
			if serviceErr.HTTPCode != tt.code {
				t.Fatalf("HTTPCode = %d, want %d", serviceErr.HTTPCode, tt.code)
			}
		})
	}

	friends.friendships[[2]uint{1, 2}] = true
	_, err := rtcSvc.Invite(ctx, 1, RTCInviteRequest{PeerID: 2, CallType: "video"})
	var serviceErr *RTCServiceError
	if !errors.As(err, &serviceErr) || serviceErr.HTTPCode != 409 {
		t.Fatalf("offline peer error = %v, want 409 RTCServiceError", err)
	}
}

func TestRTCServiceCallActionsValidation(t *testing.T) {
	ctx := context.Background()
	users := newFakeUserRepo(testUser(1, "1", ""), testUser(2, "2", ""), testUser(3, "3", ""))
	friends := newFakeFriendRepo()
	friends.friendships[[2]uint{1, 2}] = true
	chat := NewChatService(friends, newFakeGroupRepo())
	chat.RegisterConnection(ctx, 2, nil, func(payload any) error { return nil }, nil)
	rtcSvc := NewRTCService("http://localhost:7880", "api-key", "api-secret", time.Hour, users, friends, newFakeGroupRepo(), chat)
	invite, err := rtcSvc.Invite(ctx, 1, RTCInviteRequest{PeerID: 2, CallType: "voice"})
	if err != nil {
		t.Fatalf("Invite failed: %v", err)
	}

	if _, err := rtcSvc.Accept(ctx, 3, RTCAcceptRequest{CallID: invite.CallID}); serviceErrorCode(err) != 403 {
		t.Fatalf("unauthorized Accept error = %v, want 403", err)
	}
	if err := rtcSvc.Cancel(ctx, 3, RTCCallIDRequest{CallID: invite.CallID}); serviceErrorCode(err) != 403 {
		t.Fatalf("unauthorized Cancel error = %v, want 403", err)
	}
	if err := rtcSvc.Hangup(ctx, 1, RTCCallIDRequest{CallID: invite.CallID}); serviceErrorCode(err) != 400 {
		t.Fatalf("pending caller Hangup error = %v, want 400", err)
	}
	if err := rtcSvc.Reject(ctx, 2, RTCRejectRequest{CallID: invite.CallID, Reason: "busy"}); err != nil {
		t.Fatalf("Reject failed: %v", err)
	}
	if _, err := rtcSvc.Accept(ctx, 2, RTCAcceptRequest{CallID: invite.CallID}); serviceErrorCode(err) != 400 {
		t.Fatalf("Accept after reject error = %v, want 400", err)
	}
}

func serviceErrorCode(err error) int {
	var serviceErr *RTCServiceError
	if errors.As(err, &serviceErr) {
		return serviceErr.HTTPCode
	}
	return 0
}
