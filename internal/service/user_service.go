package service

import (
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
	"sleet0922/graduation_project/pkg/security"
	"strconv"
)

// ----------用户service 接口----------
type UserService interface {
	Add(user *model.User) error
	Delete(id int) error
	Update(user *model.User) error
	DeleteAll() error
	GetByID(id int) (*model.User, error)
	GetByAccount(account string) (*model.User, error)
	GetByPhone(phone string) (*model.User, error)
	AddTestUser() error
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
	hashedPassword, err := security.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashedPassword
	return s.userRepo.Add(user)
}

func (s *userService) Delete(id int) error {
	return s.userRepo.Delete(id)
}

func (s *userService) Update(user *model.User) error {
	if user.Password != "" {
		hashedPassword, err := security.HashPassword(user.Password)
		if err != nil {
			return err
		}
		user.Password = hashedPassword
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
		err := s.Add(user)
		if err != nil {
			return err
		}
	}
	return nil
}
