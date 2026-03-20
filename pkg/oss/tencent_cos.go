package oss

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"sleet0922/graduation_project/internal/config"
	"strings"
	"time"

	"github.com/tencentyun/cos-go-sdk-v5"
)

type TencentCOS struct {
	client *cos.Client
	config config.COSConfig
}

// ----------初始化 对象存储----------
func NewTencentCOS(cfg *config.ViperConfig) *TencentCOS {
	cos_config := cfg.Cos
	bucketURL, _ := url.Parse(fmt.Sprintf("https://%s.cos.%s.myqcloud.com", cos_config.Bucket, cos_config.Region))
	baseURL := &cos.BaseURL{BucketURL: bucketURL}
	client := cos.NewClient(baseURL, &http.Client{
		Transport: &cos.AuthorizationTransport{
			SecretID:  cos_config.SecretID,
			SecretKey: cos_config.SecretKey,
		},
	})
	return &TencentCOS{
		client: client,
		config: cos_config,
	}
}

// ----------对象存储 上传文件----------
func (cosClient *TencentCOS) UploadFile(ctx context.Context, file *multipart.FileHeader, destDirectory string) (string, error) {
	openedFile, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer openedFile.Close()
	fileName := fmt.Sprintf("%d_%s", time.Now().Unix(), strings.ReplaceAll(file.Filename, " ", "_"))
	basePath := strings.Trim(cosClient.config.BasePath, "/")
	destDirectory = strings.Trim(destDirectory, "/")
	objectKey := fmt.Sprintf("%s/%s/%s", basePath, destDirectory, fileName)
	objectKey = strings.TrimLeft(objectKey, "/")
	_, err = cosClient.client.Object.Put(ctx, objectKey, openedFile, nil)
	if err != nil {
		return "", fmt.Errorf("上传文件到COS失败: %v", err)
	}
	return cosClient.client.Object.GetObjectURL(objectKey).String(), nil
}

// ----------对象存储 删除文件----------
func (cosClient *TencentCOS) DeleteFile(ctx context.Context, objectKey string) error {
	objectKey = strings.TrimLeft(objectKey, "/")
	_, err := cosClient.client.Object.Delete(ctx, objectKey, nil)
	if err != nil {
		return fmt.Errorf("删除文件从COS失败: %v", err)
	}
	return nil
}
