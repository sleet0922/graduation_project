package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
)

type fakeFriendService struct {
	sendFn          func(context.Context, uint, uint) error
	sendByAccountFn func(context.Context, uint, string) (uint, error)
	handleFn        func(context.Context, uint, uint, uint) error
	checkFn         func(context.Context, uint, uint) bool
}

func (s *fakeFriendService) SendFriendRequest(ctx context.Context, senderID, receiverID uint) error {
	if s.sendFn != nil {
		return s.sendFn(ctx, senderID, receiverID)
	}
	return nil
}

func (s *fakeFriendService) SendFriendRequestByAccount(ctx context.Context, senderID uint, account string) (uint, error) {
	if s.sendByAccountFn != nil {
		return s.sendByAccountFn(ctx, senderID, account)
	}
	return 2, nil
}

func (s *fakeFriendService) HandleFriendRequest(ctx context.Context, userID, requestID uint, status uint) error {
	if s.handleFn != nil {
		return s.handleFn(ctx, userID, requestID, status)
	}
	return nil
}

func (s *fakeFriendService) GetFriendRequestsByUserID(ctx context.Context, userID uint) ([]*model.FriendRequest, error) {
	return []*model.FriendRequest{{Model: gorm.Model{ID: 1}, SenderID: 2, ReceiverID: userID}}, nil
}

func (s *fakeFriendService) RemoveFriend(ctx context.Context, userID, friendID uint) error {
	return nil
}

func (s *fakeFriendService) GetByUserID(ctx context.Context, userID uint) ([]*model.Friend, error) {
	return nil, nil
}

func (s *fakeFriendService) GetFriendDetailsByUserID(ctx context.Context, userID uint) ([]*model.FriendDetail, error) {
	return []*model.FriendDetail{{UserID: userID, FriendID: 2, Account: "1000000002"}}, nil
}

func (s *fakeFriendService) CheckFriendship(ctx context.Context, userID uint, friendID uint) bool {
	if s.checkFn != nil {
		return s.checkFn(ctx, userID, friendID)
	}
	return true
}

func (s *fakeFriendService) UpdateRemark(ctx context.Context, userID, friendID uint, remark string) error {
	return nil
}

func TestFriendHandlerCreate(t *testing.T) {
	app := fiber.New()
	handler := NewFriendHandler(&fakeFriendService{
		sendFn: func(ctx context.Context, senderID, receiverID uint) error {
			if receiverID == 1 {
				return service.ErrCannotAddSelf
			}
			return nil
		},
		sendByAccountFn: func(ctx context.Context, senderID uint, account string) (uint, error) {
			return 0, service.ErrUserNotFound
		},
	})
	app.Post("/friend", withUser(7, handler.Create))

	status, payload := testResponse(t, app, testJSONRequest("POST", "/friend", map[string]any{"friend_id": 2}))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.Success {
		t.Fatalf("create friend response = status %d payload %#v, want success", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/friend", map[string]any{}))
	if status != http.StatusBadRequest || payload["message"] != "缺少有效的好友信息" {
		t.Fatalf("missing friend response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/friend", map[string]any{"friend_id": 1}))
	if status != http.StatusBadRequest || payload["message"] != service.ErrCannotAddSelf.Error() {
		t.Fatalf("self friend response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/friend", map[string]any{"account": "missing"}))
	if status != http.StatusNotFound || payload["message"] != "未找到该用户" {
		t.Fatalf("missing account response = status %d payload %#v", status, payload)
	}
}

func TestFriendHandlerHandleAndCheck(t *testing.T) {
	handler := NewFriendHandler(&fakeFriendService{
		handleFn: func(ctx context.Context, userID, requestID, status uint) error {
			if requestID == 9 {
				return service.ErrFriendRequestPermission
			}
			if requestID == 10 {
				return gorm.ErrRecordNotFound
			}
			return nil
		},
		checkFn: func(ctx context.Context, userID, friendID uint) bool {
			return friendID == 2
		},
	})
	app := fiber.New()
	app.Post("/handle", withUser(7, handler.HandleFriendRequest))
	app.Post("/check", withUser(7, handler.CheckFriendship))

	status, payload := testResponse(t, app, testJSONRequest("POST", "/handle", map[string]any{"request_id": 1, "status": 1}))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.Success {
		t.Fatalf("handle response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/handle", map[string]any{"request_id": 9, "status": 1}))
	if status != http.StatusForbidden {
		t.Fatalf("permission response = status %d payload %#v, want 403", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/handle", map[string]any{"request_id": 10, "status": 1}))
	if status != http.StatusNotFound {
		t.Fatalf("not found response = status %d payload %#v, want 404", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/check", map[string]any{"friend_id": 2}))
	data := payload["data"].(map[string]any)
	if status != http.StatusOK || data["is_friend"] != true {
		t.Fatalf("check response = status %d payload %#v, want is_friend true", status, payload)
	}
}
