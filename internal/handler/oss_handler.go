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

// OssHandler 对象存储处理器
// 负责处理与Cloudflare R2对象存储相关的HTTP请求
type OssHandler struct {
	r2Client *oss.CloudflareR2 // R2客户端
}

// NewOssHandler 创建OSS处理器实例
// cfg: 应用配置，用于初始化R2客户端
func NewOssHandler(cfg *config.ViperConfig) *OssHandler {
	return &OssHandler{
		r2Client: oss.NewCloudflareR2(cfg),
	}
}

// GetUploadURL 获取文件上传URL
// 生成一个临时的、带签名的上传URL，前端可以直接使用PUT方法上传文件到R2
// 请求参数:
//   - key: 文件在R2中的存储名称（如：user123.jpg）
//
// 返回:
//   - upload_url: 预签名上传URL（有效期1小时）
//   - expires_in: URL有效期说明
func (h *OssHandler) GetUploadURL(c *gin.Context) {
	// 获取文件key参数
	objectKey := c.Query("key")
	if objectKey == "" {
		response.Error(c, http.StatusBadRequest, "缺少key参数")
		return
	}

	// 生成预签名上传URL（有效期1小时）
	url, err := h.r2Client.GetPresignedUploadURL(c.Request.Context(), objectKey, time.Hour)
	if err != nil {
		fmt.Printf("生成上传URL失败: %v\n", err)
		response.Error(c, http.StatusInternalServerError, "生成上传URL失败")
		return
	}

	response.Success(c, gin.H{
		"upload_url": url,
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
	url, err := h.r2Client.GetPresignedDownloadURL(c.Request.Context(), objectKey, time.Hour)
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
	if !strings.HasPrefix(contentType, "image/") {
		response.Error(c, http.StatusBadRequest, "仅支持图片上传")
		return
	}

	userID := userIDVal.(uint)
	fileURL, err := h.r2Client.UploadFile(c.Request.Context(), file, fmt.Sprintf("chat/%d", userID))
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
