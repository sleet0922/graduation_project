package service

import (
	"errors"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/security"
	"strconv"
)

// ----------用户service 接口----------
type UserService interface {
	Register(user *model.User) error
	DeleteAll() error
	AddTestUser() error
	Login(account, password string) (*model.User, error)
	UpdateAvatar(userID uint, avatar string) (*model.User, error)
	UpdateName(userID uint, name string) (*model.User, error)
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
func (s *userService) Register(user *model.User) error {
	hashedPassword, err := security.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashedPassword
	return s.userRepo.Add(user)
}

func (s *userService) DeleteAll() error {
	return s.userRepo.DeleteAll()
}

func (s *userService) AddTestUser() error {

	// 循环10次
	for i := 0; i < 10; i++ {
		user := &model.User{
			Name:       "sleet" + strconv.Itoa(i),
			Account:    "test" + strconv.Itoa(i),
			Password:   "123456",
			Phone:      "13800000000" + strconv.Itoa(i),
			Avatar:     "https://example.com/avatar.jpg",
			Gender:     0,
			Birthday:   "2000-01-01",
			Location:   "北京",
			UserStatus: 0,
		}
		err := s.Register(user)
		if err != nil {
			return err
		}
	}
	return nil
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

func (s *userService) UpdateAvatar(userID uint, avatar string) (*model.User, error) {
	return s.userRepo.UpdateAvatar(userID, avatar)
}

func (s *userService) UpdateName(userID uint, name string) (*model.User, error) {
	return s.userRepo.UpdateName(userID, name)
}
