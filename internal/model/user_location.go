package model

type UserLocation struct {
	UserID    uint    `json:"user_id" gorm:"primaryKey"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Province  string  `json:"province"`
	City      string  `json:"city"`
	District  string  `json:"district"`
	Address   string  `json:"address"`
	Timestamp int64   `json:"timestamp"`
}

func (UserLocation) TableName() string {
	return "user_location"
}
