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
	CreateLike(ctx context.Context, like *model.FeedLike) error
	DeleteLike(ctx context.Context, postID uint, userID uint) error
	IsLiked(ctx context.Context, postID uint, userID uint) (bool, error)
	IncrementLikeCount(ctx context.Context, postID uint) error
	DecrementLikeCount(ctx context.Context, postID uint) error

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

func (r *feedRepository) CreateLike(ctx context.Context, like *model.FeedLike) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "post_id"}, {Name: "user_id"}},
		DoNothing: true,
	}).Create(like).Error
}

func (r *feedRepository) DeleteLike(ctx context.Context, postID uint, userID uint) error {
	return r.db.WithContext(ctx).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Delete(&model.FeedLike{}).Error
}

func (r *feedRepository) IsLiked(ctx context.Context, postID uint, userID uint) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.FeedLike{}).
		Where("post_id = ? AND user_id = ?", postID, userID).
		Count(&count).Error
	return count > 0, err
}

func (r *feedRepository) IncrementLikeCount(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Model(&model.FeedPost{}).
		Where("id = ?", postID).
		UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error
}

func (r *feedRepository) DecrementLikeCount(ctx context.Context, postID uint) error {
	return r.db.WithContext(ctx).Model(&model.FeedPost{}).
		Where("id = ? AND like_count > 0", postID).
		UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error
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
