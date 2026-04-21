package oss

import (
	"context"
	"fmt"
	"mime/multipart"
	"sleet0922/graduation_project/internal/config"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type QiniuKodo struct {
	client    *s3.Client
	config    config.OSSConfig
	cdnDomain string
}

// 创建Kodo客户端实例
func NewQiniuKodo(cfg *config.ViperConfig) *QiniuKodo {
	ossConfig := cfg.OSS
	endpoint := fmt.Sprintf("https://%s.s3.cn-east-1.qiniucs.com", ossConfig.Bucket)
	if ossConfig.Endpoint != "" {
		endpoint = ossConfig.Endpoint
	}
	awsConfig := aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(
			ossConfig.AccessKeyID,
			ossConfig.SecretAccessKey,
			"",
		),
		Region: "cn-east-1",
	}
	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})
	cdnDomain := ossConfig.CDNDomain
	if cdnDomain == "" {
		cdnDomain = endpoint
	}

	return &QiniuKodo{
		client:    client,
		config:    ossConfig,
		cdnDomain: cdnDomain,
	}
}

func (k *QiniuKodo) UploadFile(ctx context.Context, file *multipart.FileHeader, destDirectory string) (string, error) {
	openedFile, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer openedFile.Close()

	fileName := fmt.Sprintf("%d_%s", time.Now().Unix(), strings.ReplaceAll(file.Filename, " ", "_"))
	basePath := strings.Trim(k.config.BasePath, "/")
	destDirectory = strings.Trim(destDirectory, "/")
	objectKey := fmt.Sprintf("%s/%s/%s", basePath, destDirectory, fileName)
	objectKey = strings.TrimLeft(objectKey, "/")

	_, err = k.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(k.config.Bucket),
		Key:         aws.String(objectKey),
		Body:        openedFile,
		ContentType: aws.String(file.Header.Get("Content-Type")),
	})
	if err != nil {
		return "", fmt.Errorf("上传文件到 Kodo 失败: %v", err)
	}

	accessURL := fmt.Sprintf("%s/%s", strings.TrimRight(k.cdnDomain, "/"), objectKey)
	return accessURL, nil
}

func (k *QiniuKodo) DeleteFile(ctx context.Context, objectKey string) error {
	objectKey = strings.TrimLeft(objectKey, "/")

	_, err := k.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(k.config.Bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("删除文件从 Kodo 失败: %v", err)
	}

	return nil
}

// GetPresignedUploadURL 生成预签名上传 URL
// 前端可以直接使用 PUT 方法上传文件到七牛云，不经过服务器
func (k *QiniuKodo) GetPresignedUploadURL(ctx context.Context, objectKey string, expiresIn time.Duration) (string, error) {
	objectKey = strings.TrimLeft(objectKey, "/")
	basePath := strings.Trim(k.config.BasePath, "/")
	fullObjectKey := fmt.Sprintf("%s/%s", basePath, objectKey)
	fullObjectKey = strings.TrimLeft(fullObjectKey, "/")
	presignClient := s3.NewPresignClient(k.client)
	presignResult, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(k.config.Bucket),
		Key:    aws.String(fullObjectKey),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("生成预签名上传 URL 失败: %v", err)
	}

	return presignResult.URL, nil
}

func (k *QiniuKodo) GetPresignedDownloadURL(ctx context.Context, objectKey string, expiresIn time.Duration) (string, error) {
	objectKey = strings.TrimLeft(objectKey, "/")
	basePath := strings.Trim(k.config.BasePath, "/")
	fullObjectKey := fmt.Sprintf("%s/%s", basePath, objectKey)
	fullObjectKey = strings.TrimLeft(fullObjectKey, "/")
	presignClient := s3.NewPresignClient(k.client)
	presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(k.config.Bucket),
		Key:    aws.String(fullObjectKey),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("生成预签名下载 URL 失败: %v", err)
	}

	return presignResult.URL, nil
}

// GetPublicURL 获取公开访问 URL（七牛云 CDN 链接，无需签名）
// 适用于 bucket 设置为公开读的情况，直接返回 CDN 链接
func (k *QiniuKodo) GetPublicURL(objectKey string) string {
	objectKey = strings.TrimLeft(objectKey, "/")
	return fmt.Sprintf("%s/%s", strings.TrimRight(k.cdnDomain, "/"), objectKey)
}
