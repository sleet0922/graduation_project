package service

import (
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"

	"golang.org/x/crypto/bcrypt"
)

// ----------用户service 接口----------
type UserService interface {
	Add(user *model.User) error
	Delete(id int) error
	Update(user *model.User) error
	GetByID(id int) (*model.User, error)
	GetByAccount(account string) (*model.User, error)
	GetByPhone(phone string) (*model.User, error)
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
func (s *userService) Add(user *model.User) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)
	return s.userRepo.Add(user)
}

func (s *userService) Delete(id int) error {
	return s.userRepo.Delete(id)
}

func (s *userService) Update(user *model.User) error {
	if user.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		user.Password = string(hashedPassword)
	}
	return s.userRepo.Update(user)
}

func (s *userService) GetByID(id int) (*model.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *userService) GetByAccount(account string) (*model.User, error) {
	return s.userRepo.GetByAccount(account)
}

func (s *userService) GetByPhone(phone string) (*model.User, error) {
	return s.userRepo.GetByPhone(phone)
}
