package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name       string `json:"name" gorm:"not null"`
	Account    string `json:"account" gorm:"uniqueIndex;not null"`
	Password   string `json:"-" gorm:"not null"`
	Email      string `json:"email" gorm:"not null"`
	Avatar     string `json:"avatar" gorm:"default:''"`
	Gender     int    `json:"gender" gorm:"default:0"`
	Birthday   string `json:"birthday" gorm:"default:''"`
	Location   string `json:"location" gorm:"default:''"`
	UserStatus int    `json:"user_status" gorm:"default:0"`
	PublicKey  string `json:"public_key" gorm:"type:text"` // 用于端到端加密的公钥
}

func (User) TableName() string {
	return "user"
}
