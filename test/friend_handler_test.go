package test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"sleet0922/graduation_project/internal/handler"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/jwt"

	"github.com/gin-gonic/gin"
)

type mockFriendService struct{}

func (m *mockFriendService) SendFriendRequest(senderID, receiverID uint) error {
	if senderID == receiverID {
		return service.ErrCannotAddSelf
	}
	if senderID == 1 && receiverID == 2 {
		return nil
	}
	if senderID == 1 && receiverID == 3 {
		return service.ErrAlreadyFriend
	}
	return errors.New("send failed")
}
func (m *mockFriendService) HandleFriendRequest(requestID uint, status uint) error {
	if requestID == 1 {
		return nil
	}
	return errors.New("handle failed")
}
func (m *mockFriendService) GetFriendRequestsByUserID(userID uint) ([]*model.FriendRequest, error) {
	if userID == 1 {
		return []*model.FriendRequest{{SenderID: 2, ReceiverID: 1}}, nil
	}
	return nil, errors.New("get requests failed")
}
func (m *mockFriendService) RemoveFriend(userID, friendID uint) error {
	if userID == 1 && friendID == 2 {
		return nil
	}
	return errors.New("remove failed")
}
func (m *mockFriendService) GetByUserID(userID uint) ([]*model.Friend, error) {
	return nil, nil
}
func (m *mockFriendService) GetFriendDetailsByUserID(userID uint) ([]*model.FriendDetail, error) {
	if userID == 1 {
		return []*model.FriendDetail{{FriendID: 2, Name: "Friend 2"}}, nil
	}
	return nil, errors.New("get details failed")
}
func (m *mockFriendService) CheckFriendship(userID uint, friendID uint) bool {
	if userID == 1 && friendID == 2 {
		return true
	}
	return false
}

func TestFriendHandler_Create(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockFriendService := &mockFriendService{}
	mockUserService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	friendHandler := handler.NewFriendHandler(mockFriendService, mockUserService, jwtManager)

	t.Run("Success_ByFriendID", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uint(1))
		reqBody := `{"friend_id":2}`
		req, _ := http.NewRequest(http.MethodPost, "/friend/create", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		friendHandler.Create(c)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})

	t.Run("Success_ByAccount", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uint(1))
		reqBody := `{"account":"testuser"}` // mockUserService returns ID=1 for testuser, so sender=1, receiver=1 (CannotAddSelf)
		req, _ := http.NewRequest(http.MethodPost, "/friend/create", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		friendHandler.Create(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request (CannotAddSelf), got %v", w.Code)
		}
	})

	t.Run("AlreadyFriend", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uint(1))
		reqBody := `{"friend_id":3}`
		req, _ := http.NewRequest(http.MethodPost, "/friend/create", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		friendHandler.Create(c)
		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request, got %v", w.Code)
		}
	})
}

func TestFriendHandler_Delete(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockFriendService := &mockFriendService{}
	mockUserService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	friendHandler := handler.NewFriendHandler(mockFriendService, mockUserService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uint(1))
		reqBody := `{"friend_id":2}`
		req, _ := http.NewRequest(http.MethodPost, "/friend/delete", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		friendHandler.Delete(c)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}

func TestFriendHandler_GetByUserID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockFriendService := &mockFriendService{}
	mockUserService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	friendHandler := handler.NewFriendHandler(mockFriendService, mockUserService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uint(1))
		req, _ := http.NewRequest(http.MethodGet, "/friend/list", nil)
		c.Request = req

		friendHandler.GetByUserID(c)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}

func TestFriendHandler_GetFriendRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockFriendService := &mockFriendService{}
	mockUserService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	friendHandler := handler.NewFriendHandler(mockFriendService, mockUserService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uint(1))
		req, _ := http.NewRequest(http.MethodGet, "/friend/requests", nil)
		c.Request = req

		friendHandler.GetFriendRequests(c)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}

func TestFriendHandler_HandleFriendRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockFriendService := &mockFriendService{}
	mockUserService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	friendHandler := handler.NewFriendHandler(mockFriendService, mockUserService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		reqBody := `{"request_id":1, "status":1}`
		req, _ := http.NewRequest(http.MethodPost, "/friend/handle", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		friendHandler.HandleFriendRequest(c)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}

func TestFriendHandler_CheckFriendship(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockFriendService := &mockFriendService{}
	mockUserService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	friendHandler := handler.NewFriendHandler(mockFriendService, mockUserService, jwtManager)

	t.Run("Success_True", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uint(1))
		reqBody := `{"friend_id":2}`
		req, _ := http.NewRequest(http.MethodPost, "/friend/check", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		friendHandler.CheckFriendship(c)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		if data["is_friend"] != true {
			t.Errorf("Expected is_friend true, got false")
		}
	})

	t.Run("Success_False", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("user_id", uint(1))
		reqBody := `{"friend_id":3}`
		req, _ := http.NewRequest(http.MethodPost, "/friend/check", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		friendHandler.CheckFriendship(c)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		data := response["data"].(map[string]interface{})
		if data["is_friend"] != false {
			t.Errorf("Expected is_friend false, got true")
		}
	})
}
