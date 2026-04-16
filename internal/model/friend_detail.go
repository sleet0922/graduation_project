package model

type FriendDetail struct {
	ID       uint   `json:"id"`
	UserID   uint   `json:"user_id"`
	FriendID uint   `json:"friend_id"`
	Account  string `json:"account"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Avatar   string `json:"avatar"`
	Gender   int    `json:"gender"`
	Birthday string `json:"birthday"`
	Location string `json:"location"`
	Remark   string `json:"remark"`
}
