package model

import "gorm.io/gorm"

type User struct {
	gorm.Model
	Name       string `json:"name"`
	Account    string `json:"account"`
	Password   string `json:"-"`
	Phone      string `json:"phone" gorm:"not null"`
	Avatar     string `json:"avatar" gorm:"not null"`
	Gender     int    `json:"gender"`
	Birthday   string `json:"birthday"`
	Location   string `json:"location"`
	UserStatus int    `json:"user_status" gorm:"not null; default:0"`
}

func (User) TableName() string {
	return "user"
}
