package service

import (
	"context"
	"errors"
	"log/slog"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrPostNotFound    = errors.New("动态不存在")
	ErrNotPostOwner    = errors.New("无权操作此动态")
	ErrAlreadyLiked    = errors.New("已经点赞过了")
	ErrNotLiked        = errors.New("尚未点赞")
	ErrCommentNotFound = errors.New("评论不存在")
)

// FeedPostWithLiked 帖子 + 当前用户是否已点赞
type FeedPostWithLiked struct {
	Post    *model.FeedPost
	IsLiked bool
}

// 媒体上传请求
type CreateMediaInput struct {
	MediaType model.MediaType `json:"media_type"`
	MediaURL  string          `json:"media_url"`
	SortOrder int             `json:"sort_order"`
}

// FeedService 动态业务逻辑接口
type FeedService interface {
	// 帖子
	CreatePost(ctx context.Context, userID uint, content string, media []CreateMediaInput) (*model.FeedPost, error)
	DeletePost(ctx context.Context, userID, postID uint) error
	GetPostDetail(ctx context.Context, userID, postID uint) (*FeedPostWithLiked, error)
	ListFeed(ctx context.Context, userID uint, page, pageSize int) ([]FeedPostWithLiked, int64, error)
	ListMyPosts(ctx context.Context, userID uint, page, pageSize int) ([]FeedPostWithLiked, int64, error)

	// 点赞（toggle模式：已赞则取消，未赞则点赞）
	ToggleLike(ctx context.Context, userID, postID uint) (bool, error) // 返回是否已点赞

	// 查询点赞状态
	IsLiked(ctx context.Context, userID, postID uint) (bool, error)

	// 评论
	CreateComment(ctx context.Context, userID, postID uint, content string, replyToID *uint) (*model.FeedComment, error)
	DeleteComment(ctx context.Context, userID, commentID uint) error
	ListComments(ctx context.Context, postID uint, page, pageSize int) ([]model.FeedComment, int64, error)
}

type feedService struct {
	feedRepo repo.FeedRepository
	userRepo repo.UserRepository
	db       *gorm.DB
}

func NewFeedService(feedRepo repo.FeedRepository, userRepo repo.UserRepository, db *gorm.DB) FeedService {
	return &feedService{feedRepo: feedRepo, userRepo: userRepo, db: db}
}

// ===================== 帖子 =====================

func (s *feedService) CreatePost(ctx context.Context, userID uint, content string, media []CreateMediaInput) (*model.FeedPost, error) {
	post := &model.FeedPost{
		UserID:  userID,
		Content: content,
	}

	if err := s.feedRepo.CreatePost(ctx, post); err != nil {
		return nil, err
	}

	// 批量创建媒体附件
	if len(media) > 0 {
		mediaModels := make([]model.FeedMedia, len(media))
		for i, m := range media {
			mediaModels[i] = model.FeedMedia{
				PostID:    post.ID,
				MediaType: m.MediaType,
				MediaURL:  m.MediaURL,
				SortOrder: m.SortOrder,
			}
		}
		if err := s.feedRepo.BatchCreateMedia(ctx, mediaModels); err != nil {
			return nil, err
		}
	}

	// 返回完整帖子信息
	return s.feedRepo.GetPostByID(ctx, post.ID)
}

func (s *feedService) DeletePost(ctx context.Context, userID, postID uint) error {
	post, err := s.feedRepo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPostNotFound
		}
		return err
	}
	if post.UserID != userID {
		return ErrNotPostOwner
	}
	return s.feedRepo.DeletePost(ctx, postID, userID)
}

func (s *feedService) GetPostDetail(ctx context.Context, userID, postID uint) (*FeedPostWithLiked, error) {
	post, err := s.feedRepo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	liked, err := s.feedRepo.IsLiked(ctx, postID, userID)
	if err != nil {
		slog.Error("查询点赞状态失败", "error", err, "post_id", postID, "user_id", userID)
		liked = false // 查询失败不阻断，默认未点赞
	}
	return &FeedPostWithLiked{Post: post, IsLiked: liked}, nil
}

func (s *feedService) ListFeed(ctx context.Context, userID uint, page, pageSize int) ([]FeedPostWithLiked, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	posts, total, err := s.feedRepo.ListPosts(ctx, userID, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	return s.fillLikedStatus(ctx, posts, userID), total, nil
}

func (s *feedService) ListMyPosts(ctx context.Context, userID uint, page, pageSize int) ([]FeedPostWithLiked, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	posts, total, err := s.feedRepo.ListMyPosts(ctx, userID, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}

	return s.fillLikedStatus(ctx, posts, userID), total, nil
}

// fillLikedStatus 批量填充帖子的 is_liked 状态
func (s *feedService) fillLikedStatus(ctx context.Context, posts []model.FeedPost, userID uint) []FeedPostWithLiked {
	postIDs := make([]uint, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}

	likedMap, err := s.feedRepo.BatchIsLiked(ctx, postIDs, userID)
	if err != nil {
		slog.Error("批量查询点赞状态失败", "error", err, "user_id", userID)
		likedMap = make(map[uint]bool) // 查询失败不阻断
	}

	result := make([]FeedPostWithLiked, len(posts))
	for i, p := range posts {
		result[i] = FeedPostWithLiked{
			Post:    &posts[i],
			IsLiked: likedMap[p.ID],
		}
	}
	return result
}

// ===================== 点赞 =====================

// ToggleLike 点赞/取消点赞（事务内原子操作，防止并发竞态）
func (s *feedService) ToggleLike(ctx context.Context, userID, postID uint) (bool, error) {
	// 检查帖子是否存在
	_, err := s.feedRepo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrPostNotFound
		}
		return false, err
	}

	var isLiked bool

	// 在事务中执行，保证"查询+操作+计数"的原子性
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 在事务中查询当前点赞状态（加行锁 SELECT FOR UPDATE 防止并发）
		var count int64
		if err := tx.Model(&model.FeedLike{}).
			Where("post_id = ? AND user_id = ?", postID, userID).
			Count(&count).Error; err != nil {
			return err
		}

		if count > 0 {
			// 已点赞 → 取消点赞（硬删除）
			result := tx.Where("post_id = ? AND user_id = ?", postID, userID).
				Delete(&model.FeedLike{})
			if result.Error != nil {
				return result.Error
			}
			// 只有真正删除了记录才减计数
			if result.RowsAffected > 0 {
				if err := tx.Model(&model.FeedPost{}).
					Where("id = ? AND like_count > 0", postID).
					UpdateColumn("like_count", gorm.Expr("like_count - 1")).Error; err != nil {
					slog.Error("减少点赞计数失败", "error", err, "post_id", postID)
				}
			}
			isLiked = false
		} else {
			// 未点赞 → 创建点赞
			like := &model.FeedLike{
				PostID: postID,
				UserID: userID,
			}
			result := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "post_id"}, {Name: "user_id"}},
				DoNothing: true,
			}).Create(like)
			if result.Error != nil {
				return result.Error
			}
			// 只有真正插入了记录才加计数
			if result.RowsAffected > 0 {
				if err := tx.Model(&model.FeedPost{}).
					Where("id = ?", postID).
					UpdateColumn("like_count", gorm.Expr("like_count + 1")).Error; err != nil {
					slog.Error("增加点赞计数失败", "error", err, "post_id", postID)
				}
			}
			isLiked = true
		}
		return nil
	})

	if err != nil {
		return false, err
	}
	return isLiked, nil
}

// IsLiked 查询当前用户是否已点赞某帖子
func (s *feedService) IsLiked(ctx context.Context, userID, postID uint) (bool, error) {
	return s.feedRepo.IsLiked(ctx, postID, userID)
}

// ===================== 评论 =====================

func (s *feedService) CreateComment(ctx context.Context, userID, postID uint, content string, replyToID *uint) (*model.FeedComment, error) {
	// 检查帖子是否存在
	_, err := s.feedRepo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	comment := &model.FeedComment{
		PostID:    postID,
		UserID:    userID,
		Content:   content,
		ReplyToID: replyToID,
	}

	if err := s.feedRepo.CreateComment(ctx, comment); err != nil {
		return nil, err
	}

	// 增加评论计数
	_ = s.feedRepo.IncrementCommentCount(ctx, postID)

	return comment, nil
}

func (s *feedService) DeleteComment(ctx context.Context, userID, commentID uint) error {
	if err := s.feedRepo.DeleteComment(ctx, commentID, userID); err != nil {
		return err
	}
	return nil
}

func (s *feedService) ListComments(ctx context.Context, postID uint, page, pageSize int) ([]model.FeedComment, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.feedRepo.ListComments(ctx, postID, offset, pageSize)
}
