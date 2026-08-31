package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"gorm.io/gorm"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/security"
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
	UpsertLocation(ctx context.Context, location *model.UserLocation) error
}

// ----------用户service 实现----------
type userService struct {
	userRepo repo.UserRepository
}

// ----------用户service 构造函数----------
func NewUserService(userRepo repo.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

// 生成随机账号（最多重试 5 次确保唯一性）
func (s *userService) generateRandomAccount(ctx context.Context) (string, error) {
	for i := 0; i < 5; i++ {
		prefix, err := rand.Int(rand.Reader, big.NewInt(9))
		if err != nil {
			return "", fmt.Errorf("generate account prefix: %w", err)
		}
		suffix, err := rand.Int(rand.Reader, big.NewInt(1000000000))
		if err != nil {
			return "", fmt.Errorf("generate account suffix: %w", err)
		}
		account := fmt.Sprintf("%d%09d", prefix.Int64()+1, suffix.Int64())
		_, err = s.userRepo.GetByAccount(ctx, account)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 账号不存在，可以使用
			return account, nil
		}
		if err != nil {
			return "", err
		}
		// 账号已存在，继续重试
	}
	return "", fmt.Errorf("无法生成唯一账号，请重试")
}

// 用户注册
func (s *userService) Register(ctx context.Context, email, password string) (*model.User, error) {
	account, err := s.generateRandomAccount(ctx)
	if err != nil {
		return nil, err
	}
	_, err = s.userRepo.GetByEmail(ctx, email)
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

// 用户登录
func (s *userService) Login(ctx context.Context, account, password string) (*model.User, error) {
	var user *model.User
	var err error
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

// 用户搜索
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

// 用户ID查询
func (s *userService) GetByID(ctx context.Context, id uint) (*model.User, error) {
	user, err := s.userRepo.GetByID(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return user, err
}

// 更新头像、昵称、密码、个人资料等信息
func (s *userService) UpdateAvatar(ctx context.Context, userID uint, avatar string) (*model.User, error) {
	return s.userRepo.UpdateAvatar(ctx, userID, avatar)
}

// 更新name
func (s *userService) UpdateName(ctx context.Context, userID uint, name string) (*model.User, error) {
	return s.userRepo.UpdateName(ctx, userID, name)
}

// 更新密码
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

// 更新个人资料
func (s *userService) UpdateProfile(ctx context.Context, userID uint, gender int, birthday string, location string) (*model.User, error) {
	return s.userRepo.UpdateProfile(ctx, userID, gender, birthday, location)
}

// 获取用户自己信息
func (s *userService) GetSelf(ctx context.Context, userID uint) (*model.User, error) {
	user, err := s.userRepo.GetSelf(ctx, userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return user, err
}

// 用户删除
func (s *userService) Delete(ctx context.Context, userID uint) error {
	return s.userRepo.Delete(ctx, userID)
}

// 更新/插入用户位置
func (s *userService) UpsertLocation(ctx context.Context, location *model.UserLocation) error {
	return s.userRepo.UpsertLocation(ctx, location)
}
