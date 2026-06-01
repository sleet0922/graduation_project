package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"sleet0922/graduation_project/internal/model"
	"sleet0922/graduation_project/internal/service"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/response"

	"github.com/gin-gonic/gin"
)

type FeedHandler struct {
	feedService service.FeedService
}

func NewFeedHandler(feedService service.FeedService) *FeedHandler {
	return &FeedHandler{feedService: feedService}
}

// ===================== 创建动态 ====================
// POST /api/feed/create
func (h *FeedHandler) CreatePost(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	type MediaInput struct {
		MediaType int    `json:"media_type" binding:"required"` // 1=图片 2=视频
		MediaURL  string `json:"media_url" binding:"required"`
		SortOrder int    `json:"sort_order"`
	}

	var req struct {
		Content string       `json:"content"`
		Media   []MediaInput `json:"media"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	// 纯文字动态也需要至少有一个内容
	if req.Content == "" && len(req.Media) == 0 {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	mediaInputs := make([]service.CreateMediaInput, len(req.Media))
	for i, m := range req.Media {
		mediaInputs[i] = service.CreateMediaInput{
			MediaType: model.MediaType(m.MediaType),
			MediaURL:  m.MediaURL,
			SortOrder: m.SortOrder,
		}
	}

	post, err := h.feedService.CreatePost(c.Request.Context(), userID, req.Content, mediaInputs)
	if err != nil {
		slog.Error("创建动态失败", "error", err, "user_id", userID)
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	response.Success(c, post, "发布成功")
}

// ===================== 删除动态 ====================
// DELETE /api/feed/delete
func (h *FeedHandler) DeletePost(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	var req struct {
		PostID uint `json:"post_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	if err := h.feedService.DeletePost(c.Request.Context(), userID, req.PostID); err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
			return
		}
		if errors.Is(err, service.ErrNotPostOwner) {
			response.Result(c, http.StatusForbidden, errcode.Forbidden, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	response.Success(c, nil, "删除成功")
}

// ===================== 动态详情 ====================
// GET /api/feed/detail
func (h *FeedHandler) GetDetail(c *gin.Context) {
	postIDStr := c.Query("post_id")
	if postIDStr == "" {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	postID, err := strconv.ParseUint(postIDStr, 10, 64)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	post, err := h.feedService.GetPostDetail(c.Request.Context(), uint(postID))
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	response.Success(c, post, "获取动态详情成功")
}

// ===================== 动态列表（朋友圈） ====================
// GET /api/feed/list?page=1&page_size=20
func (h *FeedHandler) ListFeed(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	posts, total, err := h.feedService.ListFeed(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	response.Success(c, gin.H{
		"list":      posts,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}, "获取动态列表成功")
}

// ===================== 我的动态 ====================
// GET /api/feed/my_posts?page=1&page_size=20
func (h *FeedHandler) ListMyPosts(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	posts, total, err := h.feedService.ListMyPosts(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	response.Success(c, gin.H{
		"list":      posts,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}, "获取我的动态成功")
}

// ===================== 点赞/取消点赞（Toggle） ====================
// POST /api/feed/like
func (h *FeedHandler) ToggleLike(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	var req struct {
		PostID uint `json:"post_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error("like bind error", "error", err)
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	liked, err := h.feedService.ToggleLike(c.Request.Context(), userID, req.PostID)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	msg := "取消点赞"
	if liked {
		msg = "点赞成功"
	}
	response.Success(c, gin.H{
		"is_liked": liked,
	}, msg)
}

// ===================== 发表评论 ====================
// POST /api/feed/comment
func (h *FeedHandler) CreateComment(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	var req struct {
		PostID    uint   `json:"post_id" binding:"required"`
		Content   string `json:"content" binding:"required"`
		ReplyToID *uint  `json:"reply_to_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	comment, err := h.feedService.CreateComment(c.Request.Context(), userID, req.PostID, req.Content, req.ReplyToID)
	if err != nil {
		if errors.Is(err, service.ErrPostNotFound) {
			response.Result(c, http.StatusNotFound, errcode.NotFound, nil)
			return
		}
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	response.Success(c, comment, "评论成功")
}

// ===================== 删除评论 ====================
// DELETE /api/feed/comment
func (h *FeedHandler) DeleteComment(c *gin.Context) {
	userID, err := GetUserID(c)
	if err != nil {
		response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
		return
	}

	var req struct {
		CommentID uint `json:"comment_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	if err := h.feedService.DeleteComment(c.Request.Context(), userID, req.CommentID); err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	response.Success(c, nil, "删除评论成功")
}

// ===================== 评论列表 ====================
// GET /api/feed/comments?post_id=1&page=1&page_size=20
func (h *FeedHandler) ListComments(c *gin.Context) {
	postIDStr := c.Query("post_id")
	if postIDStr == "" {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}
	postID, err := strconv.ParseUint(postIDStr, 10, 64)
	if err != nil {
		response.Result(c, http.StatusBadRequest, errcode.InvalidParams, nil)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	comments, total, err := h.feedService.ListComments(c.Request.Context(), uint(postID), page, pageSize)
	if err != nil {
		response.Result(c, http.StatusInternalServerError, errcode.InternalServerError, nil)
		return
	}

	response.Success(c, gin.H{
		"list":      comments,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	}, "获取评论列表成功")
}
