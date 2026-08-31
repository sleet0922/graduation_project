package service

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/repo"
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
	ListComments(ctx context.Context, userID, postID uint, page, pageSize int) ([]model.FeedComment, int64, error)
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

func (s *feedService) GetPostDetail(ctx context.Context, userID, postID uint) (*FeedPostWithLiked, error) {
	canView, err := s.feedRepo.CanViewPost(ctx, userID, postID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, ErrPostNotFound
	}
	post, err := s.feedRepo.GetPostByID(ctx, postID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}
	liked, err := s.feedRepo.IsLiked(ctx, postID, userID)
	if err != nil {
		return nil, fmt.Errorf("查询点赞状态失败: %w", err)
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

	items, err := s.fillLikedStatus(ctx, posts, userID)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
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

	items, err := s.fillLikedStatus(ctx, posts, userID)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// fillLikedStatus 批量填充帖子的 is_liked 状态
func (s *feedService) fillLikedStatus(ctx context.Context, posts []model.FeedPost, userID uint) ([]FeedPostWithLiked, error) {
	postIDs := make([]uint, len(posts))
	for i, p := range posts {
		postIDs[i] = p.ID
	}

	likedMap, err := s.feedRepo.BatchIsLiked(ctx, postIDs, userID)
	if err != nil {
		return nil, fmt.Errorf("批量查询点赞状态失败: %w", err)
	}

	result := make([]FeedPostWithLiked, len(posts))
	for i, p := range posts {
		result[i] = FeedPostWithLiked{
			Post:    &posts[i],
			IsLiked: likedMap[p.ID],
		}
	}
	return result, nil
}

// ===================== 点赞 =====================

// ToggleLike 点赞/取消点赞。事务边界由 repository 管理，确保点赞记录和
// 冗余计数始终一起提交或回滚。
func (s *feedService) ToggleLike(ctx context.Context, userID, postID uint) (bool, error) {
	isLiked, err := s.feedRepo.ToggleLike(ctx, userID, postID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrPostNotFound
		}
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
	// 权限 + 帖子存在性合并为一次 CanViewPost 检查
	canView, err := s.feedRepo.CanViewPost(ctx, userID, postID)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, ErrPostNotFound
	}

	comment := &model.FeedComment{
		PostID:    postID,
		UserID:    userID,
		Content:   content,
		ReplyToID: replyToID,
	}

	if err := s.feedRepo.CreateCommentWithCount(ctx, comment); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPostNotFound
		}
		return nil, err
	}

	return comment, nil
}

func (s *feedService) DeleteComment(ctx context.Context, userID, commentID uint) error {
	// 先取到评论，判断操作者权限：评论作者 或 帖子作者 均可删除
	comment, err := s.feedRepo.GetCommentByID(ctx, commentID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrCommentNotFound
		}
		return err
	}

	if comment.UserID == userID {
		// 评论作者删自己的评论
		_, _, err = s.feedRepo.DeleteCommentWithCount(ctx, commentID, userID)
	} else {
		// 检查操作者是否为帖子作者
		post, postErr := s.feedRepo.GetPostByID(ctx, comment.PostID)
		if postErr != nil {
			if errors.Is(postErr, gorm.ErrRecordNotFound) {
				return ErrPostNotFound
			}
			return postErr
		}
		if post.UserID != userID {
			return ErrNotPostOwner
		}
		// 帖子作者强制删除
		_, _, err = s.feedRepo.ForceDeleteCommentWithCount(ctx, commentID)
	}

	if err != nil {
		return err
	}
	// The repository has already committed the comment deletion and counter
	// update atomically; no second write is needed here.
	return nil
}

func (s *feedService) ListComments(ctx context.Context, userID, postID uint, page, pageSize int) ([]model.FeedComment, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	canView, err := s.feedRepo.CanViewPost(ctx, userID, postID)
	if err != nil {
		return nil, 0, err
	}
	if !canView {
		return nil, 0, ErrPostNotFound
	}
	return s.feedRepo.ListComments(ctx, postID, offset, pageSize)
}
