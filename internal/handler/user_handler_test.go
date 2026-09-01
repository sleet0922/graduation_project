package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/jwt"
)

type fakeUserService struct {
	registerFn       func(context.Context, string, string) (*model.User, error)
	loginFn          func(context.Context, string, string) (*model.User, error)
	deleteFn         func(context.Context, uint) error
	searchFn         func(context.Context, string) (*model.User, error)
	getByIDFn        func(context.Context, uint) (*model.User, error)
	updateAvatarFn   func(context.Context, uint, string) (*model.User, error)
	updateNameFn     func(context.Context, uint, string) (*model.User, error)
	updatePasswordFn func(context.Context, uint, string, string) error
	updateProfileFn  func(context.Context, uint, int, string, string) (*model.User, error)
	getSelfFn        func(context.Context, uint) (*model.User, error)
	upsertLocationFn func(context.Context, *model.UserLocation) error
}

func (s *fakeUserService) Register(ctx context.Context, email, password string) (*model.User, error) {
	if s.registerFn != nil {
		return s.registerFn(ctx, email, password)
	}
	return &model.User{Email: email, Account: "1000000001", Name: "Tester"}, nil
}

func (s *fakeUserService) Login(ctx context.Context, account, password string) (*model.User, error) {
	if s.loginFn != nil {
		return s.loginFn(ctx, account, password)
	}
	return &model.User{Model: gorm.Model{ID: 1}, Account: account, Email: "u@example.com", Name: "Tester"}, nil
}

func (s *fakeUserService) Delete(ctx context.Context, userID uint) error {
	if s.deleteFn != nil {
		return s.deleteFn(ctx, userID)
	}
	return nil
}

func (s *fakeUserService) SearchUser(ctx context.Context, keyword string) (*model.User, error) {
	if s.searchFn != nil {
		return s.searchFn(ctx, keyword)
	}
	return &model.User{Model: gorm.Model{ID: 2}, Account: "1000000002", Email: keyword, Name: "Found"}, nil
}

func (s *fakeUserService) GetByID(ctx context.Context, id uint) (*model.User, error) {
	if s.getByIDFn != nil {
		return s.getByIDFn(ctx, id)
	}
	return &model.User{Model: gorm.Model{ID: id}}, nil
}

func (s *fakeUserService) UpdateAvatar(ctx context.Context, userID uint, avatar string) (*model.User, error) {
	if s.updateAvatarFn != nil {
		return s.updateAvatarFn(ctx, userID, avatar)
	}
	return &model.User{Model: gorm.Model{ID: userID}, Avatar: avatar}, nil
}

func (s *fakeUserService) UpdateName(ctx context.Context, userID uint, name string) (*model.User, error) {
	if s.updateNameFn != nil {
		return s.updateNameFn(ctx, userID, name)
	}
	return &model.User{Model: gorm.Model{ID: userID}, Name: name}, nil
}

func (s *fakeUserService) UpdatePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	if s.updatePasswordFn != nil {
		return s.updatePasswordFn(ctx, userID, oldPassword, newPassword)
	}
	return nil
}

func (s *fakeUserService) UpdateProfile(ctx context.Context, userID uint, gender int, birthday string, location string) (*model.User, error) {
	if s.updateProfileFn != nil {
		return s.updateProfileFn(ctx, userID, gender, birthday, location)
	}
	return &model.User{Model: gorm.Model{ID: userID}, Gender: gender, Birthday: birthday, Location: location}, nil
}

func (s *fakeUserService) GetSelf(ctx context.Context, userID uint) (*model.User, error) {
	if s.getSelfFn != nil {
		return s.getSelfFn(ctx, userID)
	}
	return &model.User{Model: gorm.Model{ID: userID}, Account: "1000000001", Email: "u@example.com"}, nil
}

func (s *fakeUserService) UpsertLocation(ctx context.Context, location *model.UserLocation) error {
	if s.upsertLocationFn != nil {
		return s.upsertLocationFn(ctx, location)
	}
	return nil
}

func TestUserHandlerRegister(t *testing.T) {
	handler, err := NewUserHandler(&fakeUserService{}, jwt.NewJWTManager("secret"), time.Hour, time.Hour, nil, WithSessionStore(testSessionStore{}))
	if err != nil {
		t.Fatalf("NewUserHandler failed: %v", err)
	}
	app := fiber.New()
	app.Post("/register", handler.Register)

	status, payload := testResponse(t, app, testJSONRequest("POST", "/register", map[string]any{"email": "a@example.com", "password": "secret123"}))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.Success {
		t.Fatalf("register response = status %d payload %#v, want success", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/register", map[string]any{"email": ""}))
	if status != http.StatusBadRequest || int(payload["code"].(float64)) != errcode.InvalidParams {
		t.Fatalf("bad register response = status %d payload %#v, want invalid params", status, payload)
	}

	duplicate, err := NewUserHandler(&fakeUserService{
		registerFn: func(ctx context.Context, email, password string) (*model.User, error) {
			return nil, service.ErrUserAlreadyExists
		},
	}, jwt.NewJWTManager("secret"), time.Hour, time.Hour, nil, WithSessionStore(testSessionStore{}))
	if err != nil {
		t.Fatalf("NewUserHandler duplicate failed: %v", err)
	}
	app = fiber.New()
	app.Post("/register", duplicate.Register)
	status, payload = testResponse(t, app, testJSONRequest("POST", "/register", map[string]any{"email": "a@example.com", "password": "secret123"}))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.ErrorUserExist {
		t.Fatalf("duplicate response = status %d payload %#v, want ErrorUserExist", status, payload)
	}
}

func TestUserHandlerAuthenticatedEndpoints(t *testing.T) {
	svc := &fakeUserService{
		getSelfFn: func(ctx context.Context, userID uint) (*model.User, error) {
			return nil, service.ErrUserNotFound
		},
		updatePasswordFn: func(ctx context.Context, userID uint, oldPassword, newPassword string) error {
			return service.ErrOldPasswordIncorrect
		},
		upsertLocationFn: func(ctx context.Context, location *model.UserLocation) error {
			if location.UserID != 7 || location.City != "Shanghai" {
				return errors.New("unexpected location")
			}
			return nil
		},
	}
	handler, err := NewUserHandler(svc, jwt.NewJWTManager("secret"), time.Hour, time.Hour, nil, WithSessionStore(testSessionStore{}))
	if err != nil {
		t.Fatalf("NewUserHandler failed: %v", err)
	}
	app := fiber.New()
	app.Post("/self", handler.GetSelf)
	app.Post("/password", withUser(7, handler.UpdatePassword))
	app.Post("/location", withUser(7, handler.ReportLocation))

	status, payload := testResponse(t, app, testJSONRequest("POST", "/self", nil))
	if status != http.StatusUnauthorized || int(payload["code"].(float64)) != errcode.Unauthorized {
		t.Fatalf("unauth self response = status %d payload %#v", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/password", map[string]any{"password": "OldPass123", "new_password": "NewPass456"}))
	if status != http.StatusUnauthorized || int(payload["code"].(float64)) != errcode.ErrorPasswordCheck {
		t.Fatalf("password response = status %d payload %#v, want old password error", status, payload)
	}

	status, payload = testResponse(t, app, testJSONRequest("POST", "/location", map[string]any{"latitude": 1.2, "longitude": 3.4, "city": "Shanghai"}))
	if status != http.StatusOK || int(payload["code"].(float64)) != errcode.Success {
		t.Fatalf("location response = status %d payload %#v, want success", status, payload)
	}
}

func TestNewUserHandlerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewUserHandler(&fakeUserService{}, jwt.NewJWTManager("secret"), time.Hour, time.Hour, nil); err == nil {
		t.Fatal("NewUserHandler accepted a missing session store")
	}
	if _, err := NewUserHandler(nil, jwt.NewJWTManager("secret"), time.Hour, time.Hour, nil, WithSessionStore(testSessionStore{})); err == nil {
		t.Fatal("NewUserHandler accepted a nil user service")
	}
	if _, err := NewUserHandler(&fakeUserService{}, nil, time.Hour, time.Hour, nil, WithSessionStore(testSessionStore{})); err == nil {
		t.Fatal("NewUserHandler accepted a nil JWT manager")
	}
}
