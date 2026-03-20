package repo

import (
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
)

// ----------数据库操作层 接口----------
type UserRepository interface {
	Add(user *model.User) error
	GetByID(id int) (*model.User, error)
	GetByAccount(account string) (*model.User, error)
	GetByPhone(phone string) (*model.User, error)
	Update(user *model.User) error
	Delete(id int) error
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

func (r *userRepository) Delete(id int) error {
	return r.db.Delete(&model.User{}, id).Error
}

func (r *userRepository) Update(user *model.User) error {
	return r.db.Save(user).Error
}

func (r *userRepository) GetByID(id int) (*model.User, error) {
	var user model.User
	err := r.db.Where("id = ?", id).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

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
