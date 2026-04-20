package model

import "time"

type E2EEUserPublicKey struct {
	UserID    uint      `json:"user_id" gorm:"primaryKey"`
	KeyType   string    `json:"key_type" gorm:"type:varchar(32);not null;default:x25519"`
	PublicKey string    `json:"public_key" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (E2EEUserPublicKey) TableName() string {
	return "e2ee_user_public_keys"
}
