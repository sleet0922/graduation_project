package model

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel 替代 gorm.Model，JSON 序列化使用小写 key
type BaseModel struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}
