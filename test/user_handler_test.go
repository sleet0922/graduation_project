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
	"sleet0922/graduation_project/pkg/jwt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// mockUserService 实现了 service.UserService 接口
type mockUserService struct{}

func (m *mockUserService) Register(email, password string) (*model.User, error) {
	if email == "test@example.com" {
		return &model.User{
			Model:   gorm.Model{ID: 1},
			Account: "testuser",
			Name:    "Test User",
			Email:   email,
		}, nil
	}
	return nil, errors.New("register failed")
}
func (m *mockUserService) Login(account, password string) (*model.User, error) {
	if account == "testuser" && password == "password" {
		return &model.User{
			Model:   gorm.Model{ID: 1},
			Account: "testuser",
			Name:    "Test User",
			Email:   "test@example.com",
		}, nil
	}
	return nil, errors.New("invalid credentials")
}
func (m *mockUserService) SearchUser(keyword string) (*model.User, error) {
	if keyword == "testuser" {
		return &model.User{
			Model:   gorm.Model{ID: 1},
			Account: "testuser",
			Name:    "Test User",
			Email:   "test@example.com",
		}, nil
	}
	return nil, errors.New("not found")
}
func (m *mockUserService) GetByID(id uint) (*model.User, error) {
	if id == 1 {
		return &model.User{Model: gorm.Model{ID: 1}, Account: "testuser", Name: "Test User"}, nil
	}
	return nil, errors.New("not found")
}
func (m *mockUserService) UpdateAvatar(userID uint, avatar string) (*model.User, error) {
	if userID == 1 {
		return &model.User{Model: gorm.Model{ID: 1}, Avatar: avatar}, nil
	}
	return nil, errors.New("update avatar failed")
}
func (m *mockUserService) UpdateName(userID uint, name string) (*model.User, error) {
	if userID == 1 {
		return &model.User{Model: gorm.Model{ID: 1}, Name: name}, nil
	}
	return nil, errors.New("update name failed")
}
func (m *mockUserService) UpdatePassword(userID uint, oldPassword, newPassword string) error {
	if userID == 1 && oldPassword == "oldpass" {
		return nil
	}
	return errors.New("update password failed")
}
func (m *mockUserService) GetSelf(userID uint) (*model.User, error) {
	if userID == 1 {
		return &model.User{
			Model:   gorm.Model{ID: 1},
			Account: "testuser",
			Name:    "Test User",
		}, nil
	}
	return nil, errors.New("not found")
}

func TestUserHandler_SearchUser(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	userHandler := handler.NewUserHandler(mockService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/user/search?keyword=testuser", nil)
		c.Request = req

		userHandler.SearchUser(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)
		if response["message"] != "搜索用户成功" {
			t.Errorf("Expected message '搜索用户成功', got %v", response["message"])
		}
	})

	t.Run("MissingKeyword", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/user/search", nil)
		c.Request = req

		userHandler.SearchUser(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request, got %v", w.Code)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/user/search?keyword=unknown", nil)
		c.Request = req

		userHandler.SearchUser(c)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status Not Found, got %v", w.Code)
		}
	})
}

func TestUserHandler_GetSelf(t *testing.T) {
	gin.SetMode(gin.TestMode)

	mockService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	userHandler := handler.NewUserHandler(mockService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Set("user_id", uint(1))

		req, _ := http.NewRequest(http.MethodGet, "/user/self", nil)
		c.Request = req

		userHandler.GetSelf(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})

	t.Run("Unauthorized", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		req, _ := http.NewRequest(http.MethodGet, "/user/self", nil)
		c.Request = req

		userHandler.GetSelf(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status Unauthorized, got %v", w.Code)
		}
	})
}

func TestUserHandler_Register(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	userHandler := handler.NewUserHandler(mockService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := `{"email":"test@example.com","password":"password123"}`
		req, _ := http.NewRequest(http.MethodPost, "/user/register", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		userHandler.Register(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})

	t.Run("BadRequest", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := `{"email":"test@example.com"}` // missing password
		req, _ := http.NewRequest(http.MethodPost, "/user/register", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		userHandler.Register(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request, got %v", w.Code)
		}
	})
}

func TestUserHandler_Login(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	userHandler := handler.NewUserHandler(mockService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := `{"account":"testuser","password":"password"}`
		req, _ := http.NewRequest(http.MethodPost, "/user/login", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		userHandler.Login(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})

	t.Run("InvalidCredentials", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := `{"account":"testuser","password":"wrongpassword"}`
		req, _ := http.NewRequest(http.MethodPost, "/user/login", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		userHandler.Login(c)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status Unauthorized, got %v", w.Code)
		}
	})
}

func TestUserHandler_RefreshToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	userHandler := handler.NewUserHandler(mockService, jwtManager)

	// generate a valid token first
	token, _ := jwtManager.GenerateRefreshToken(1, "testuser", 3600*1000000000)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		reqBody := `{"refresh_token":"` + token + `"}`
		req, _ := http.NewRequest(http.MethodPost, "/user/refresh", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		userHandler.RefreshToken(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}

func TestUserHandler_UpdateAvatar(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	userHandler := handler.NewUserHandler(mockService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Set("user_id", uint(1))

		reqBody := `{"object_key":"new_avatar.png"}`
		req, _ := http.NewRequest(http.MethodPost, "/user/avatar", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		userHandler.UpdateAvatar(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}

func TestUserHandler_UpdateName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	userHandler := handler.NewUserHandler(mockService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Set("user_id", uint(1))

		reqBody := `{"name":"New Test Name"}`
		req, _ := http.NewRequest(http.MethodPost, "/user/name", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		userHandler.UpdateName(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}

func TestUserHandler_UpdatePassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mockService := &mockUserService{}
	jwtManager := jwt.NewJWTManager("testsecret")
	userHandler := handler.NewUserHandler(mockService, jwtManager)

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Set("user_id", uint(1))

		reqBody := `{"password":"oldpass","new_password":"newpass"}`
		req, _ := http.NewRequest(http.MethodPost, "/user/password", bytes.NewBufferString(reqBody))
		req.Header.Set("Content-Type", "application/json")
		c.Request = req

		userHandler.UpdatePassword(c)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status OK, got %v", w.Code)
		}
	})
}
