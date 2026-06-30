package model

import "time"

// MediaType 媒体类型
type MediaType int

const (
	MediaTypeImage MediaType = 1 // 图片
	MediaTypeVideo MediaType = 2 // 视频
)

// FeedPost 动态帖子
type FeedPost struct {
	BaseModel
	UserID  uint   `json:"user_id" gorm:"index;not null"`
	Content string `json:"content" gorm:"type:text"`
	// 关联
	Author   User         `json:"author" gorm:"foreignKey:UserID"`
	Media    []FeedMedia  `json:"media" gorm:"foreignKey:PostID"`
	Likes    []FeedLike   `json:"-" gorm:"foreignKey:PostID"`
	Comments []FeedComment `json:"-" gorm:"foreignKey:PostID"`
	// 计数（数据库冗余字段，避免频繁 count 查询）
	LikeCount    int `json:"like_count" gorm:"default:0"`
	CommentCount int `json:"comment_count" gorm:"default:0"`
}

func (FeedPost) TableName() string {
	return "feed_post"
}

// FeedMedia 动态媒体附件（图片/视频）
type FeedMedia struct {
	BaseModel
	PostID    uint      `json:"post_id" gorm:"index;not null"`
	MediaType MediaType `json:"media_type" gorm:"not null"` // 1=图片 2=视频
	MediaURL  string    `json:"media_url" gorm:"type:varchar(512);not null"`
	SortOrder int       `json:"sort_order" gorm:"default:0"`
}

func (FeedMedia) TableName() string {
	return "feed_media"
}

// FeedLike 动态点赞（不使用软删除，避免取消点赞后重新点赞时唯一索引冲突）
type FeedLike struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	PostID    uint      `json:"post_id" gorm:"uniqueIndex:idx_post_user;not null"`
	UserID    uint      `json:"user_id" gorm:"uniqueIndex:idx_post_user;not null"`
	// 关联
	User User `json:"user" gorm:"foreignKey:UserID"`
}

func (FeedLike) TableName() string {
	return "feed_like"
}

// FeedComment 动态评论
type FeedComment struct {
	BaseModel
	PostID    uint   `json:"post_id" gorm:"index;not null"`
	UserID    uint   `json:"user_id" gorm:"not null"`
	Content   string `json:"content" gorm:"type:text;not null"`
	ReplyToID *uint  `json:"reply_to_id" gorm:"default:null"` // 回复某条评论的ID，nil表示直接评论帖子
	// 关联
	User    User          `json:"user" gorm:"foreignKey:UserID"`
	ReplyTo *FeedComment  `json:"reply_to,omitempty" gorm:"foreignKey:ReplyToID"`
}

func (FeedComment) TableName() string {
	return "feed_comment"
}
