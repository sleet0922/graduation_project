package repo

import (
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
)

type UserRepository interface {
	Add(user *model.User) error
	Delete(id uint) error
	Update(user *model.User) error
	GetByID(id uint) (*model.User, error)
	GetByAccount(account string) (*model.User, error)
	GetByPhone(phone string) (*model.User, error)
	UpdateAvatar(userID uint, avatar string) (*model.User, error)
	UpdateName(userID uint, name string) (*model.User, error)
	UpdatePassword(userID uint, password string) (*model.User, error)
	GetSelf(userID uint) (*model.User, error)
}

// ----------数据库操作层 实现----------
type userRepository struct {
	db *gorm.DB
}

// ----------数据库操作层 构造函数----------
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// ----------数据库操作层 方法----------
func (r *userRepository) Add(user *model.User) error {
	return r.db.Create(user).Error
}

func (r *userRepository) Delete(id uint) error {
	return r.db.Delete(&model.User{}, id).Error
}

func (r *userRepository) Update(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) GetByID(id uint) (*model.User, error) {
	var user model.User
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 根据账号获取用户
func (r *userRepository) GetByAccount(account string) (*model.User, error) {
	var user model.User
	err := r.db.Where("account = ?", account).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) GetByPhone(phone string) (*model.User, error) {
	var user model.User
	err := r.db.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepository) UpdateAvatar(userID uint, avatar string) (*model.User, error) {
	err := r.db.Model(&model.User{}).Where("id = ?", userID).Update("avatar", avatar).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(userID)
}

func (r *userRepository) UpdateName(userID uint, name string) (*model.User, error) {
	err := r.db.Model(&model.User{}).Where("id = ?", userID).Update("name", name).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(userID)
}

func (r *userRepository) UpdatePassword(userID uint, password string) (*model.User, error) {
	err := r.db.Model(&model.User{}).Where("id = ?", userID).Update("password", password).Error
	if err != nil {
		return nil, err
	}
	return r.GetByID(userID)
}

func (r *userRepository) GetSelf(userID uint) (*model.User, error) {
	var user model.User
	err := r.db.Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
