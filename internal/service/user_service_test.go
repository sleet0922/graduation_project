package service

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"sleet0922/graduation_project/pkg/security"
)

func TestUserServiceRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("creates user with hashed password and default name", func(t *testing.T) {
		repo := newFakeUserRepo()
		svc := NewUserService(repo)

		user, err := svc.Register(ctx, "new@example.com", "secret")
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}
		if user.ID == 0 {
			t.Fatal("registered user has no ID")
		}
		if user.Name != "未命名用户" {
			t.Fatalf("Name = %q, want default", user.Name)
		}
		if user.Password == "secret" {
			t.Fatal("password was stored as plaintext")
		}
		if err := security.CheckPassword(user.Password, "secret"); err != nil {
			t.Fatalf("stored password hash does not match original password: %v", err)
		}
		if len(user.Account) != 10 {
			t.Fatalf("account length = %d, want 10", len(user.Account))
		}
	})

	t.Run("rejects duplicate email", func(t *testing.T) {
		user := testUser(1, "1000000001", "exists@example.com")
		user.Password = "hash"
		repo := newFakeUserRepo(user)
		svc := NewUserService(repo)

		_, err := svc.Register(ctx, "exists@example.com", "secret")
		if !errors.Is(err, ErrUserAlreadyExists) {
			t.Fatalf("Register error = %v, want ErrUserAlreadyExists", err)
		}
	})

	t.Run("propagates repository lookup errors", func(t *testing.T) {
		repo := newFakeUserRepo()
		repo.err = errors.New("db down")
		svc := NewUserService(repo)

		_, err := svc.Register(ctx, "new@example.com", "secret")
		if err == nil || errors.Is(err, ErrUserAlreadyExists) {
			t.Fatalf("Register error = %v, want repository error", err)
		}
	})
}

func TestUserServiceLoginAndSearch(t *testing.T) {
	ctx := context.Background()
	hashed, err := security.HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	loginUser := testUser(7, "1000000007", "user@example.com")
	loginUser.Password = hashed
	loginUser.Name = "Tester"
	repo := newFakeUserRepo(loginUser)
	svc := NewUserService(repo)

	user, err := svc.Login(ctx, "user@example.com", "secret")
	if err != nil {
		t.Fatalf("Login by email failed: %v", err)
	}
	if user.ID != 7 {
		t.Fatalf("Login user ID = %d, want 7", user.ID)
	}

	user, err = svc.Login(ctx, "1000000007", "secret")
	if err != nil {
		t.Fatalf("Login by account failed: %v", err)
	}
	if user.Email != "user@example.com" {
		t.Fatalf("Login email = %q, want user@example.com", user.Email)
	}

	_, err = svc.Login(ctx, "1000000007", "wrong")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("wrong password error = %v, want ErrInvalidCredentials", err)
	}

	_, err = svc.SearchUser(ctx, "missing@example.com")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("SearchUser error = %v, want ErrUserNotFound", err)
	}
}

func TestUserServiceUpdatePassword(t *testing.T) {
	ctx := context.Background()
	hashed, err := security.HashPassword("old")
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}
	user := testUser(1, "1000000001", "u@example.com")
	user.Password = hashed
	repo := newFakeUserRepo(user)
	svc := NewUserService(repo)

	if err := svc.UpdatePassword(ctx, 1, "old", "new"); err != nil {
		t.Fatalf("UpdatePassword failed: %v", err)
	}
	updatedUser, err := repo.GetByID(ctx, 1)
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if err := security.CheckPassword(updatedUser.Password, "new"); err != nil {
		t.Fatalf("new password hash does not validate: %v", err)
	}
	if err := security.CheckPassword(updatedUser.Password, "old"); err == nil {
		t.Fatal("old password still validates after update")
	}

	if err := svc.UpdatePassword(ctx, 1, "bad-old", "newer"); !errors.Is(err, ErrOldPasswordIncorrect) {
		t.Fatalf("bad old password error = %v, want ErrOldPasswordIncorrect", err)
	}
	if err := svc.UpdatePassword(ctx, 99, "old", "new"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("missing user error = %v, want ErrUserNotFound", err)
	}
}

func TestUserServiceGetByIDMapsNotFound(t *testing.T) {
	svc := NewUserService(newFakeUserRepo())

	_, err := svc.GetByID(context.Background(), 404)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("GetByID error = %v, want ErrUserNotFound", err)
	}

	repo := newFakeUserRepo()
	repo.err = gorm.ErrInvalidDB
	_, err = NewUserService(repo).GetByID(context.Background(), 1)
	if !errors.Is(err, gorm.ErrInvalidDB) {
		t.Fatalf("GetByID error = %v, want underlying error", err)
	}
}
