package repo

import (
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
)

// ----------数据库操作层 接口----------
type UserRepository interface {
	Add(user *model.User) error
	Delete(id uint) error
	Update(user *model.User) error
	DeleteAll() error
	AddTestUser() error
	GetByID(id uint) (*model.User, error)
	GetByAccount(account string) (*model.User, error)
	GetByPhone(phone string) (*model.User, error)
	UpdateAvatar(userID uint, avatar string) (*model.User, error)
	UpdateName(userID uint, name string) (*model.User, error)
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

// 更新用户
func (r *userRepository) Update(user *model.User) error {
	return r.db.Save(user).Error
}

// 删除所有用户
func (r *userRepository) DeleteAll() error {
	return r.db.Unscoped().Where("1=1").Delete(&model.User{}).Error
}

// 添加测试用户
func (r *userRepository) AddTestUser() error {
	return r.db.Create(&model.User{
		Name:       "sleet",
		Account:    "943781228",
		Password:   "Zyz20050922!",
		Phone:      "13915181300",
		Avatar:     "https://example.com/avatar.jpg",
		Gender:     1,
		Birthday:   "2006-05-28",
		Location:   "江苏",
		UserStatus: 0,
	}).Error
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

// 根据手机号获取用户
func (r *userRepository) GetByPhone(phone string) (*model.User, error) {
	var user model.User
	err := r.db.Where("phone = ?", phone).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 更新用户指定字段
func (r *userRepository) UpdateField(userID uint, field string, value interface{}) (*model.User, error) {
	var user model.User
	err := r.db.Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	err = r.db.Model(&user).Update(field, value).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// 更新用户头像
func (r *userRepository) UpdateAvatar(userID uint, avatar string) (*model.User, error) {
	return r.UpdateField(userID, "avatar", avatar)
}

// 更新用户名
func (r *userRepository) UpdateName(userID uint, name string) (*model.User, error) {
	return r.UpdateField(userID, "name", name)
}
