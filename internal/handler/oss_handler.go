package handler

import (
	"fmt"
	"net/http"
	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/pkg/oss"
	"sleet0922/graduation_project/pkg/response"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type OssHandler struct {
	kodoClient *oss.QiniuKodo
}

func NewOssHandler(cfg *config.ViperConfig) *OssHandler {
	return &OssHandler{
		kodoClient: oss.NewQiniuKodo(cfg),
	}
}

// GetUploadURL 获取文件上传URL
// 生成一个临时的、带签名的上传URL，前端可以直接使用PUT方法上传到七牛云
// 请求参数:
//   - key: 文件在存储中的名称（如：avatar_3_1776731300657.jpg）
//   - type: 文件类型，可选值为 "avatar"(头像) 或 "chat"(聊天图片)，默认为 "chat"
//
// 返回:
//   - upload_url: 预签名上传URL（有效期1小时）
//   - access_url: 文件访问URL（上传成功后可直接使用）
//   - expires_in: URL有效期说明
func (h *OssHandler) GetUploadURL(c *gin.Context) {
	// 获取文件key参数
	objectKey := c.Query("key")
	if objectKey == "" {
		response.Error(c, http.StatusBadRequest, "缺少key参数")
		return
	}
	// 获取文件类型，决定存储路径
	fileType := c.Query("type")
	if fileType == "" {
		fileType = "chat" // 默认聊天图片
	}
	// 根据类型添加路径前缀
	var fullObjectKey string
	switch fileType {
	case "avatar":
		fullObjectKey = "avatar/" + objectKey
	case "chat":
		fullObjectKey = "chat/" + objectKey
	default:
		fullObjectKey = objectKey
	}
	// 生成预签名上传URL（有效期1小时）
	presignedURL, err := h.kodoClient.GetPresignedUploadURL(c.Request.Context(), fullObjectKey, time.Hour)
	if err != nil {
		fmt.Printf("生成上传URL失败: %v\n", err)
		response.Error(c, http.StatusInternalServerError, "生成上传URL失败")
		return
	}

	// 生成访问URL
	accessURL := h.kodoClient.GetPublicURL(fullObjectKey)

	response.Success(c, gin.H{
		"upload_url": presignedURL,
		"access_url": accessURL,
		"expires_in": "1小时",
	}, "获取上传URL成功")
}

// GetDownloadURL 获取文件下载URL
// 生成一个临时的、带签名的下载URL，前端可以直接使用GET方法下载文件
// 请求参数:
//   - key: 文件在R2中的存储名称（如：user123.jpg）
//
// 返回:
//   - download_url: 预签名下载URL（有效期1小时）
//   - expires_in: URL有效期说明
func (h *OssHandler) GetDownloadURL(c *gin.Context) {
	// 获取文件key参数
	objectKey := c.Query("key")
	if objectKey == "" {
		response.Error(c, http.StatusBadRequest, "缺少key参数")
		return
	}

	// 生成预签名下载URL（有效期1小时）
	url, err := h.kodoClient.GetPresignedDownloadURL(c.Request.Context(), objectKey, time.Hour)
	if err != nil {
		fmt.Printf("生成下载URL失败: %v\n", err)
		response.Error(c, http.StatusInternalServerError, "生成下载URL失败")
		return
	}

	response.Success(c, gin.H{
		"download_url": url,
		"expires_in":   "1小时",
	}, "获取下载URL成功")
}

func (h *OssHandler) UploadChatImage(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		response.Error(c, http.StatusUnauthorized, "未找到用户信息")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "请选择图片文件")
		return
	}
	if file.Size == 0 {
		response.Error(c, http.StatusBadRequest, "图片不能为空")
		return
	}
	if file.Size > 10*1024*1024 {
		response.Error(c, http.StatusBadRequest, "图片大小不能超过10MB")
		return
	}
	contentType := file.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") && contentType != "application/octet-stream" {
		response.Error(c, http.StatusBadRequest, "仅支持图片或二进制流上传")
		return
	}

	userID := userIDVal.(uint)
	fileURL, err := h.kodoClient.UploadFile(c.Request.Context(), file, fmt.Sprintf("chat/%d", userID))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "上传聊天图片失败")
		return
	}

	response.Success(c, gin.H{
		"url":         fileURL,
		"content":     fileURL,
		"filename":    file.Filename,
		"contentType": contentType,
	}, "上传聊天图片成功")
}
