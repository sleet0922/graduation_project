package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name       string `json:"name" gorm:"not null"`
	Account    string `json:"account" gorm:"uniqueIndex;not null"`
	Password   string `json:"-" gorm:"not null"`
	Phone      string `json:"phone" gorm:"uniqueIndex;not null"`
	Avatar     string `json:"avatar" gorm:"default:''"`
	Gender     int    `json:"gender" gorm:"default:0"`
	Birthday   string `json:"birthday" gorm:"default:''"`
	Location   string `json:"location" gorm:"default:''"`
	UserStatus int    `json:"user_status" gorm:"default:0"`
}

func (User) TableName() string {
	return "user"
}
