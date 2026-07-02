package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v2"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
)

type fakeE2EEService struct {
	publishKeyFn   func(context.Context, uint, string, string) (*model.E2EEUserPublicKey, error)
	getKeyFn       func(context.Context, uint) (*model.E2EEUserPublicKey, error)
	currentBoxFn   func(context.Context, uint, uint) (*model.E2EEGroupKeyBox, error)
	versionFn      func(context.Context, uint) (int, error)
	publishBoxesFn func(context.Context, uint, uint, int, []service.GroupKeyBoxUpload, string) error
}

func (s *fakeE2EEService) PublishUserPublicKey(ctx context.Context, userID uint, keyType, publicKey string) (*model.E2EEUserPublicKey, error) {
	if s.publishKeyFn != nil {
		return s.publishKeyFn(ctx, userID, keyType, publicKey)
	}
	return &model.E2EEUserPublicKey{UserID: userID, KeyType: keyType, PublicKey: publicKey}, nil
}

func (s *fakeE2EEService) GetUserPublicKey(ctx context.Context, userID uint) (*model.E2EEUserPublicKey, error) {
	if s.getKeyFn != nil {
		return s.getKeyFn(ctx, userID)
	}
	return &model.E2EEUserPublicKey{UserID: userID, KeyType: "x25519", PublicKey: "key"}, nil
}

func (s *fakeE2EEService) GetGroupCurrentKeyBox(ctx context.Context, currentUserID, groupID uint) (*model.E2EEGroupKeyBox, error) {
	if s.currentBoxFn != nil {
		return s.currentBoxFn(ctx, currentUserID, groupID)
	}
	return &model.E2EEGroupKeyBox{GroupID: groupID, KeyVersion: 1, UserID: currentUserID, WrappedGroupKey: "wrapped", WrapNonce: "nonce"}, nil
}

func (s *fakeE2EEService) GetGroupKeyBoxByVersion(ctx context.Context, currentUserID, groupID uint, keyVersion int) (*model.E2EEGroupKeyBox, error) {
	return &model.E2EEGroupKeyBox{GroupID: groupID, KeyVersion: keyVersion, UserID: currentUserID, WrappedGroupKey: "wrapped", WrapNonce: "nonce"}, nil
}

func (s *fakeE2EEService) GetGroupCurrentVersion(ctx context.Context, groupID uint) (int, error) {
	if s.versionFn != nil {
		return s.versionFn(ctx, groupID)
	}
	return 3, nil
}

func (s *fakeE2EEService) RotateGroupKey(ctx context.Context, groupID, currentUserID uint) error {
	return nil
}

func (s *fakeE2EEService) PublishGroupKeyBoxes(ctx context.Context, currentUserID, groupID uint, keyVersion int, boxes []service.GroupKeyBoxUpload, keyWrapAlg string) error {
	if s.publishBoxesFn != nil {
		return s.publishBoxesFn(ctx, currentUserID, groupID, keyVersion, boxes, keyWrapAlg)
	}
	return nil
}

func TestE2EEHandlerPublicKey(t *testing.T) {
	handler := NewE2EEHandler(&fakeE2EEService{
		publishKeyFn: func(ctx context.Context, userID uint, keyType, publicKey string) (*model.E2EEUserPublicKey, error) {
			if keyType == "bad" {
				return nil, service.ErrUnsupportedE2EEKeyType
			}
			return &model.E2EEUserPublicKey{UserID: userID, KeyType: keyType, PublicKey: publicKey}, nil
		},
		getKeyFn: func(ctx context.Context, userID uint) (*model.E2EEUserPublicKey, error) {
			if userID == 404 {
				return nil, service.ErrE2EEPublicKeyNotFound
			}
			return &model.E2EEUserPublicKey{UserID: userID, KeyType: "x25519", PublicKey: "pub"}, nil
		},
	})
	app := fiber.New()
	app.Post("/key", withUser(7, handler.PublishPublicKey))
	app.Get("/key", handler.GetPublicKey)

	status, _ := testResponse(t, app, testJSONRequest("POST", "/key", map[string]any{"key_type": "x25519", "public_key": "pub"}))
	if status != http.StatusOK {
		t.Fatalf("publish key status = %d, want 200", status)
	}
	status, _ = testResponse(t, app, testJSONRequest("POST", "/key", map[string]any{"key_type": "bad", "public_key": "pub"}))
	if status != http.StatusBadRequest {
		t.Fatalf("bad publish key status = %d, want 400", status)
	}
	status, _ = testResponse(t, app, testJSONRequest("GET", "/key?user_id=404", nil))
	if status != http.StatusNotFound {
		t.Fatalf("missing key status = %d, want 404", status)
	}
}

func TestE2EEHandlerGroupKeyResponses(t *testing.T) {
	handler := NewE2EEHandler(&fakeE2EEService{
		currentBoxFn: func(ctx context.Context, currentUserID, groupID uint) (*model.E2EEGroupKeyBox, error) {
			return nil, service.ErrE2EEGroupKeyBoxMissing
		},
		publishBoxesFn: func(ctx context.Context, currentUserID, groupID uint, keyVersion int, boxes []service.GroupKeyBoxUpload, keyWrapAlg string) error {
			return service.ErrE2EEGroupVersionLock
		},
	})
	app := fiber.New()
	app.Get("/current", withUser(7, handler.GetGroupCurrentKey))
	app.Post("/boxes", withUser(7, handler.PublishGroupKeyBoxes))

	status, payload := testResponse(t, app, testJSONRequest("GET", "/current?group_id=1", nil))
	if status != 428 || payload["message"] != "e2ee group key box not found, please upload key boxes" {
		t.Fatalf("missing box response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/boxes", map[string]any{
		"group_id":    1,
		"key_version": 1,
		"boxes":       []map[string]any{{"user_id": 2, "wrapped_group_key": "k", "wrap_nonce": "n"}},
	}))
	if status != http.StatusConflict || payload["message"] != "e2ee group key version conflict" {
		t.Fatalf("version lock response = status %d payload %#v", status, payload)
	}
}
