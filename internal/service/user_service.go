package service

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/security"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrUserAlreadyExists    = errors.New("用户已存在")
	ErrUserNotFound         = errors.New("用户不存在")
	ErrInvalidCredentials   = errors.New("账号或密码错误")
	ErrOldPasswordIncorrect = errors.New("原密码错误")
)

// ----------用户 service 接口----------
type UserService interface {
	Register(ctx context.Context, email, password string) (*model.User, error)
	Login(ctx context.Context, account, password string) (*model.User, error)
	Delete(ctx context.Context, userID uint) error
	SearchUser(ctx context.Context, keyword string) (*model.User, error)
	GetByID(ctx context.Context, id uint) (*model.User, error)
	UpdateAvatar(ctx context.Context, userID uint, avatar string) (*model.User, error)
	UpdateName(ctx context.Context, userID uint, name string) (*model.User, error)
	UpdatePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error
	UpdateProfile(ctx context.Context, userID uint, gender int, birthday string, location string) (*model.User, error)
	GetSelf(ctx context.Context, userID uint) (*model.User, error)
}

// ----------用户service 实现----------
type userService struct {
	userRepo repo.UserRepository
}

// ----------用户service 构造函数----------
func NewUserService(userRepo repo.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

// ----------用户service 方法----------
func (s *userService) Register(ctx context.Context, email, password string) (*model.User, error) {
	account := s.generateRandomAccount(ctx)
	_, err := s.userRepo.GetByEmail(ctx, email)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hashedPassword, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Name:     "未命名用户",
		Account:  account,
		Password: hashedPassword,
		Email:    email,
	}
	err = s.userRepo.Add(ctx, user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) generateRandomAccount(ctx context.Context) string {
	for i := 0; i < 100; i++ {
		account := fmt.Sprintf("%010d", rand.Intn(10000000000))
		_, err := s.userRepo.GetByAccount(ctx, account)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return account
		}
		if err != nil {
			continue
		}
	}
	// Fallback: use timestamp + random suffix to guarantee uniqueness
	return fmt.Sprintf("%010d", time.Now().UnixNano()%10000000000)
}

func (s *userService) Login(ctx context.Context, account, password string) (*model.User, error) {
	var user *model.User
	var err error

	// 判断是邮箱还是账号登录
	if strings.Contains(account, "@") {
		user, err = s.userRepo.GetByEmail(ctx, account)
	} else {
		user, err = s.userRepo.GetByAccount(ctx, account)
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	err = security.CheckPassword(user.Password, password)
	if err != nil {
		return nil, ErrInvalidCredentials
	}
	return user, nil
}

func (s *userService) SearchUser(ctx context.Context, keyword string) (*model.User, error) {
	if strings.Contains(keyword, "@") {
		user, err := s.userRepo.GetByEmail(ctx, keyword)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return user, err
	}
	user, err := s.userRepo.GetByAccount(ctx, keyword)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return user, err
}

func (s *userService) GetByID(ctx context.Context, id uint) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return user, err
}

func (s *userService) UpdateAvatar(ctx context.Context, userID uint, avatar string) (*model.User, error) {
	return s.userRepo.UpdateAvatar(ctx, userID, avatar)
}

func (s *userService) UpdateName(ctx context.Context, userID uint, name string) (*model.User, error) {
	return s.userRepo.UpdateName(ctx, userID, name)
}

func (s *userService) UpdatePassword(ctx context.Context, userID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	err = security.CheckPassword(user.Password, oldPassword)
	if err != nil {
		return ErrOldPasswordIncorrect
	}
	hashedPassword, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = s.userRepo.UpdatePassword(ctx, userID, hashedPassword)
	return err
}

func (s *userService) UpdateProfile(ctx context.Context, userID uint, gender int, birthday string, location string) (*model.User, error) {
	return s.userRepo.UpdateProfile(ctx, userID, gender, birthday, location)
}

func (s *userService) GetSelf(ctx context.Context, userID uint) (*model.User, error) {
	user, err := s.userRepo.GetSelf(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return user, err
}

func (s *userService) Delete(ctx context.Context, userID uint) error {
	return s.userRepo.Delete(ctx, userID)
}
