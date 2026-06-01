package service

import (
	"context"
	"errors"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"

	"gorm.io/gorm"
)

var (
	ErrPostNotFound   = errors.New("动态不存在")
	ErrNotPostOwner   = errors.New("无权操作此动态")
	ErrAlreadyLiked   = errors.New("已经点赞过了")
	ErrNotLiked       = errors.New("尚未点赞")
	ErrCommentNotFound = errors.New("评论不存在")
)

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
	GetPostDetail(ctx context.Context, postID uint) (*model.FeedPost, error)
	ListFeed(ctx context.Context, userID uint, page, pageSize int) ([]model.FeedPost, int64, error)
	ListMyPosts(ctx context.Context, userID uint, page, pageSize int) ([]model.FeedPost, int64, error)

	// 点赞（toggle模式：已赞则取消，未赞则点赞）
	ToggleLike(ctx context.Context, userID, postID uint) (bool, error) // 返回是否已点赞

	// 评论
	CreateComment(ctx context.Context, userID, postID uint, content string, replyToID *uint) (*model.FeedComment, error)
	DeleteComment(ctx context.Context, userID, commentID uint) error
	ListComments(ctx context.Context, postID uint, page, pageSize int) ([]model.FeedComment, int64, error)
}

type feedService struct {
	feedRepo repo.FeedRepository
	userRepo repo.UserRepository
}

func NewFeedService(feedRepo repo.FeedRepository, userRepo repo.UserRepository) FeedService {
	return &feedService{feedRepo: feedRepo, userRepo: userRepo}
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

func (s *feedService) GetPostDetail(ctx context.Context, postID uint) (*model.FeedPost, error) {
	post, err := s.feedRepo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	return post, nil
}

func (s *feedService) ListFeed(ctx context.Context, userID uint, page, pageSize int) ([]model.FeedPost, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.feedRepo.ListPosts(ctx, userID, offset, pageSize)
}

func (s *feedService) ListMyPosts(ctx context.Context, userID uint, page, pageSize int) ([]model.FeedPost, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.feedRepo.ListMyPosts(ctx, userID, offset, pageSize)
}

// ===================== 点赞 =====================

func (s *feedService) ToggleLike(ctx context.Context, userID, postID uint) (bool, error) {
	// 检查帖子是否存在
	_, err := s.feedRepo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrPostNotFound
		}
		return false, err
	}

	liked, err := s.feedRepo.IsLiked(ctx, postID, userID)
	if err != nil {
		return false, err
	}

	if liked {
		// 取消点赞
		if err := s.feedRepo.DeleteLike(ctx, postID, userID); err != nil {
			return false, err
		}
		_ = s.feedRepo.DecrementLikeCount(ctx, postID)
		return false, nil
	}

	// 点赞
	like := &model.FeedLike{
		PostID: postID,
		UserID: userID,
	}
	if err := s.feedRepo.CreateLike(ctx, like); err != nil {
		return false, err
	}
	_ = s.feedRepo.IncrementLikeCount(ctx, postID)
	return true, nil
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
