package service

import (
	"errors"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/security"
)

// ----------用户 service 接口----------
type UserService interface {
	Register(name, account, password, phone string) (*model.User, error)
	Login(account, password string) (*model.User, error)
	GetByID(id uint) (*model.User, error)
	UpdateAvatar(userID uint, avatar string) (*model.User, error)
	UpdateName(userID uint, name string) (*model.User, error)
	UpdatePassword(userID uint, oldPassword, newPassword string) error
	GetSelf(userID uint) (*model.User, error)
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
func (s *userService) Register(name, account, password, phone string) (*model.User, error) {
	hashedPassword, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Name:     name,
		Account:  account,
		Password: hashedPassword,
		Phone:    phone,
	}
	err = s.userRepo.Add(user)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *userService) Login(account, password string) (*model.User, error) {
	user, err := s.userRepo.GetByAccount(account)
	if err != nil {
		return nil, errors.New("账号或密码错误")
	}

	err = security.CheckPassword(user.Password, password)
	if err != nil {
		return nil, errors.New("账号或密码错误")
	}

	return user, nil
}

func (s *userService) GetByID(id uint) (*model.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *userService) UpdateAvatar(userID uint, avatar string) (*model.User, error) {
	return s.userRepo.UpdateAvatar(userID, avatar)
}

func (s *userService) UpdateName(userID uint, name string) (*model.User, error) {
	return s.userRepo.UpdateName(userID, name)
}

func (s *userService) UpdatePassword(userID uint, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return errors.New("用户不存在")
	}
	if err := security.CheckPassword(user.Password, oldPassword); err != nil {
		return errors.New("原密码错误")
	}
	hashedPassword, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = s.userRepo.UpdatePassword(userID, hashedPassword)
	return err
}

func (s *userService) GetSelf(userID uint) (*model.User, error) {
	return s.userRepo.GetSelf(userID)
}
