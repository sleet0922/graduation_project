package repo

import (
	"context"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FeedRepository 动态数据访问接口
type FeedRepository interface {
	// 帖子
	CreatePost(ctx context.Context, post *model.FeedPost) error
	DeletePost(ctx context.Context, postID uint, userID uint) error
	GetPostByID(ctx context.Context, postID uint) (*model.FeedPost, error)
	ListPosts(ctx context.Context, userID uint, offset, limit int) ([]model.FeedPost, int64, error)
	ListMyPosts(ctx context.Context, userID uint, offset, limit int) ([]model.FeedPost, int64, error)

	// 媒体
	CreateMedia(ctx context.Context, media *model.FeedMedia) error
	BatchCreateMedia(ctx context.Context, media []model.FeedMedia) error
	DeleteMediaByPostID(ctx context.Context, postID uint) error

	// 点赞
	CreateLike(ctx context.Context, like *model.FeedLike) (int64, error)  // 返回受影响行数
	DeleteLike(ctx context.Context, postID uint, userID uint) (int64, error) // 返回受影响行数
	IsLiked(ctx context.Context, postID uint, userID uint) (bool, error)
	IncrementLikeCount(ctx context.Context, postID uint) error
	DecrementLikeCount(ctx context.Context, postID uint) error
	BatchIsLiked(ctx context.Context, postIDs []uint, userID uint) (map[uint]bool, error)

	// 评论
	CreateComment(ctx context.Context, comment *model.FeedComment) error
	DeleteComment(ctx context.Context, commentID uint, userID uint) error
	ListComments(ctx context.Context, postID uint, offset, limit int) ([]model.FeedComment, int64, error)
	IncrementCommentCount(ctx context.Context, postID uint) error
	DecrementCommentCount(ctx context.Context, postID uint) error
}

type feedRepository struct {
	db *gorm.DB
}

func NewFeedRepository(db *gorm.DB) FeedRepository {
	return &feedRepository{db: db}
}

// ===================== 帖子 =====================

func (r *feedRepository) CreatePost(ctx context.Context, post *model.FeedPost) error {
	return r.db.WithContext(ctx).Create(post).Error
}

func (r *feedRepository) DeletePost(ctx context.Context, postID uint, userID uint) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", postID, userID).
		Delete(&model.FeedPost{}).Error
}

func (r *feedRepository) GetPostByID(ctx context.Context, postID uint) (*model.FeedPost, error) {
	var post model.FeedPost
	err := r.db.WithContext(ctx).
		Preload("Author").
		Preload("Media", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Preload("Likes.User").
		Preload("Comments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(3)
		}).
		Preload("Comments.User").
		Where("id = ? AND deleted_at IS NULL", postID).
		First(&post).Error
	if err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *feedRepository) ListPosts(ctx context.Context, userID uint, offset, limit int) ([]model.FeedPost, int64, error) {
	var posts []model.FeedPost
	var total int64

	// 只查好友的帖子 + 自己的帖子
	query := r.db.WithContext(ctx).Model(&model.FeedPost{}).
		Where("deleted_at IS NULL").
		Where("user_id = ? OR user_id IN (?)",
			userID,
			r.db.Model(&model.Friend{}).
				Select("friend_id").
				Where("user_id = ? AND deleted_at IS NULL", userID),
		)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Author").
		Preload("Media", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Preload("Likes", func(db *gorm.DB) *gorm.DB {
			return db.Limit(5)
		}).
		Preload("Likes.User").
		Preload("Comments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(3)
		}).
		Preload("Comments.User").
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&posts).Error

	return posts, total, err
}

func (r *feedRepository) ListMyPosts(ctx context.Context, userID uint, offset, limit int) ([]model.FeedPost, int64, error) {
	var posts []model.FeedPost
	var total int64

	query := r.db.WithContext(ctx).Model(&model.FeedPost{}).
		Where("user_id = ? AND deleted_at IS NULL", userID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("Author").
		Preload("Media", func(db *gorm.DB) *gorm.DB {
			return db.Order("sort_order ASC")
		}).
		Preload("Likes", func(db *gorm.DB) *gorm.DB {
			return db.Limit(5)
		}).
		Preload("Likes.User").
		Preload("Comments", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(3)
		}).
		Preload("Comments.User").
		Order("created_at DESC").
		Offset(offset).Limit(limit).
		Find(&posts).Error

	return posts, total, err
}

// ===================== 媒体 =====================

func (r *feedRepository) CreateMedia(ctx context.Context, media *model.FeedMedia) error {
	return r.db.WithContext(ctx).Create(media).Error
}

func (r *feedRepository) BatchCreateMedia(ctx context.Context, media []model.FeedMedia) error {
	return r.db.WithContext(ctx).Create(&media).Error
}

func (r *feedRepository) DeleteMediaByPostID(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Where("post_id = ?", postID).Delete(&model.FeedMedia{}).Error
}

// ===================== 点赞 =====================

// CreateLike 创建点赞（INSERT ON CONFLICT DO NOTHING，返回受影响行数用于判断是否真正插入）
func (r *feedRepository) CreateLike(ctx context.Context, like *model.FeedLike) (int64, error) {
	result := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "post_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).Create(like)
	return result.RowsAffected, result.Error
}

// DeleteLike 硬删除点赞记录（Unscoped 绕过软删除，返回受影响行数用于判断是否真正删除）
func (r *feedRepository) DeleteLike(ctx context.Context, postID uint, userID uint) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Delete(&model.FeedLike{})
	return result.RowsAffected, result.Error
}

// IsLiked 查询当前用户是否已点赞某帖子
func (r *feedRepository) IsLiked(ctx context.Context, postID uint, userID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.FeedLike{}).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Count(&count).Error
	return count > 0, err
}

// IncrementLikeCount 帖子点赞数 +1
func (r *feedRepository) IncrementLikeCount(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Model(&model.FeedPost{}).
		Where("id = ?", postID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

// DecrementLikeCount 帖子点赞数 -1（带 like_count > 0 防护）
func (r *feedRepository) DecrementLikeCount(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Model(&model.FeedPost{}).
		Where("id = ? AND like_count > 0", postID).
		UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error
}

// BatchIsLiked 批量查询当前用户对多个帖子的点赞状态，返回 map[postID]bool
func (r *feedRepository) BatchIsLiked(ctx context.Context, postIDs []uint, userID uint) (map[uint]bool, error) {
	if len(postIDs) == 0 {
		return make(map[uint]bool), nil
	}
	var likes []model.FeedLike
	err := r.db.WithContext(ctx).
		Select("post_id").
		Where("post_id IN ? AND user_id = ?", postIDs, userID).
		Find(&likes).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]bool, len(likes))
	for _, l := range likes {
		result[l.PostID] = true
	}
	return result, nil
}

// ===================== 评论 =====================

func (r *feedRepository) CreateComment(ctx context.Context, comment *model.FeedComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *feedRepository) DeleteComment(ctx context.Context, commentID uint, userID uint) error {
	return r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", commentID, userID).
		Delete(&model.FeedComment{}).Error
}

func (r *feedRepository) ListComments(ctx context.Context, postID uint, offset, limit int) ([]model.FeedComment, int64, error) {
	var comments []model.FeedComment
	var total int64

	query := r.db.WithContext(ctx).Model(&model.FeedComment{}).
		Where("post_id = ? AND deleted_at IS NULL", postID)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.
		Preload("User").
		Preload("ReplyTo.User").
		Order("created_at ASC").
		Offset(offset).Limit(limit).
		Find(&comments).Error

	return comments, total, err
}

func (r *feedRepository) IncrementCommentCount(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Model(&model.FeedPost{}).
		Where("id = ?", postID).
		UpdateColumn("comment_count", gorm.Expr("comment_count + 1")).Error
}

func (r *feedRepository) DecrementCommentCount(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Model(&model.FeedPost{}).
		Where("id = ? AND comment_count > 0", postID).
		UpdateColumn("comment_count", gorm.Expr("comment_count - 1")).Error
}
