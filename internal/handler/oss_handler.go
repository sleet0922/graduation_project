package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"sleet0922/graduation_project/pkg/errcode"
	"sleet0922/graduation_project/pkg/logger"
	"sleet0922/graduation_project/pkg/oss"
	"sleet0922/graduation_project/pkg/response"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type OssHandler struct {
	kodoClient *oss.QiniuKodo
}

func NewOssHandler(kodoClient *oss.QiniuKodo) *OssHandler {
	return &OssHandler{kodoClient: kodoClient}
}

// GetUploadURL 获取文件上传URL
func (h *OssHandler) GetUploadURL(c *fiber.Ctx) error {
	objectKey := c.Query("key")
	if objectKey == "" {
		return response.Error(c, http.StatusBadRequest, "缺少key参数")
	}
	fileType := c.Query("type")
	if fileType == "" {
		fileType = "chat"
	}
	var fullObjectKey string
	switch fileType {
	case "avatar":
		fullObjectKey = "avatar/" + objectKey
	case "chat", "video":
		fullObjectKey = "chat/" + objectKey
	default:
		fullObjectKey = objectKey
	}
	presignedURL, err := h.kodoClient.GetPresignedUploadURL(c.Context(), fullObjectKey, time.Hour)
	if err != nil {
		logger.Error("生成上传URL失败", slog.Any("error", err), slog.String("key", fullObjectKey))
		return response.Error(c, http.StatusInternalServerError, "生成上传URL失败")
	}

	accessURL := h.kodoClient.GetPublicURL(fullObjectKey)

	return response.Success(c, fiber.Map{
		"upload_url": presignedURL,
		"access_url": accessURL,
		"expires_in": "1小时",
	}, "获取上传URL成功")
}

// GetDownloadURL 获取文件下载URL
func (h *OssHandler) GetDownloadURL(c *fiber.Ctx) error {
	objectKey := c.Query("key")
	if objectKey == "" {
		return response.Error(c, http.StatusBadRequest, "缺少key参数")
	}

	var fullObjectKey string
	if strings.HasPrefix(objectKey, "avatar_") {
		fullObjectKey = "avatar/" + objectKey
	} else if strings.HasPrefix(objectKey, "chat_") {
		fullObjectKey = "chat/" + objectKey
	} else {
		fullObjectKey = objectKey
	}

	url, err := h.kodoClient.GetPresignedDownloadURL(c.Context(), fullObjectKey, time.Hour)
	if err != nil {
		logger.Error("生成下载URL失败", slog.Any("error", err), slog.String("key", fullObjectKey))
		return response.Error(c, http.StatusInternalServerError, "生成下载URL失败")
	}

	return response.Success(c, fiber.Map{
		"download_url": url,
		"expires_in":   "1小时",
	}, "获取下载URL成功")
}

func (h *OssHandler) uploadChatFile(c *fiber.Ctx, maxSize int64, mimePrefix, fileLabel, successLabel string) error {
	userID, err := GetUserID(c)
	if err != nil {
		return response.Result(c, http.StatusUnauthorized, errcode.Unauthorized, nil)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return response.Error(c, http.StatusBadRequest, "请选择"+fileLabel+"文件")
	}
	if file.Size == 0 {
		return response.Error(c, http.StatusBadRequest, fileLabel+"不能为空")
	}
	if file.Size > maxSize {
		return response.Error(c, http.StatusBadRequest, fileLabel+"大小不能超过"+formatSize(maxSize))
	}
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, mimePrefix) && contentType != "application/octet-stream" {
		return response.Error(c, http.StatusBadRequest, "仅支持"+fileLabel+"或二进制流上传")
	}

	fileURL, err := h.kodoClient.UploadFile(c.Context(), file, fmt.Sprintf("chat/%d", userID))
	if err != nil {
		return response.Error(c, http.StatusInternalServerError, "上传聊天"+fileLabel+"失败")
	}

	return response.Success(c, fiber.Map{
		"url":         fileURL,
		"content":     fileURL,
		"filename":    file.Filename,
		"contentType": contentType,
	}, successLabel)
}

func formatSize(bytes int64) string {
	if bytes >= 1024*1024*1024 {
		return fmt.Sprintf("%dGB", bytes/(1024*1024*1024))
	}
	return fmt.Sprintf("%dMB", bytes/(1024*1024))
}

func (h *OssHandler) UploadChatImage(c *fiber.Ctx) error {
	return h.uploadChatFile(c, 10*1024*1024, "image/", "图片", "上传聊天图片成功")
}

func (h *OssHandler) UploadChatVideo(c *fiber.Ctx) error {
	return h.uploadChatFile(c, 100*1024*1024, "video/", "视频", "上传聊天视频成功")
}
