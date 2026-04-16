package oss

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/url"
	"sleet0922/graduation_project/internal/config"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type CloudflareR2 struct {
	client *s3.Client
	config config.OSSConfig
}

// NewCloudflareR2 创建 Cloudflare R2 客户端实例
func NewCloudflareR2(cfg *config.ViperConfig) *CloudflareR2 {
	ossConfig := cfg.OSS

	// 解析 endpoint
	_, err := url.Parse(ossConfig.Endpoint)
	if err != nil {
		panic(fmt.Sprintf("解析 endpoint 失败: %v", err))
	}

	// 创建 AWS 配置
	awsConfig := aws.Config{
		Credentials: credentials.NewStaticCredentialsProvider(
			ossConfig.AccessKeyID,
			ossConfig.SecretAccessKey,
			"",
		),
		Region: "us-east-1",
	}

	// 创建 S3 客户端
	client := s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(ossConfig.Endpoint)
	})

	return &CloudflareR2{
		client: client,
		config: ossConfig,
	}
}

// UploadFile 上传文件
func (r2 *CloudflareR2) UploadFile(ctx context.Context, file *multipart.FileHeader, destDirectory string) (string, error) {
	openedFile, err := file.Open()
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %v", err)
	}
	defer openedFile.Close()

	// 构建 object key
	fileName := fmt.Sprintf("%d_%s", time.Now().Unix(), strings.ReplaceAll(file.Filename, " ", "_"))
	basePath := strings.Trim(r2.config.BasePath, "/")
	destDirectory = strings.Trim(destDirectory, "/")
	objectKey := fmt.Sprintf("%s/%s/%s", basePath, destDirectory, fileName)
	objectKey = strings.TrimLeft(objectKey, "/")

	// 上传文件
	_, err = r2.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(r2.config.Bucket),
		Key:         aws.String(objectKey),
		Body:        openedFile,
		ContentType: aws.String(file.Header.Get("Content-Type")),
	})
	if err != nil {
		return "", fmt.Errorf("上传文件到 R2 失败: %v", err)
	}

	// 构建访问 URL
	accessURL := fmt.Sprintf("%s/%s/%s", r2.config.Endpoint, r2.config.Bucket, objectKey)
	return accessURL, nil
}

// DeleteFile 删除文件
func (r2 *CloudflareR2) DeleteFile(ctx context.Context, objectKey string) error {
	objectKey = strings.TrimLeft(objectKey, "/")

	_, err := r2.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(r2.config.Bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		return fmt.Errorf("删除文件从 R2 失败: %v", err)
	}

	return nil
}

// GetPresignedUploadURL 生成预签名上传 URL
func (r2 *CloudflareR2) GetPresignedUploadURL(ctx context.Context, objectKey string, expiresIn time.Duration) (string, error) {
	objectKey = strings.TrimLeft(objectKey, "/")

	// 构建完整的 object key
	basePath := strings.Trim(r2.config.BasePath, "/")
	fullObjectKey := fmt.Sprintf("%s/%s", basePath, objectKey)
	fullObjectKey = strings.TrimLeft(fullObjectKey, "/")

	// 生成预签名 URL
	presignClient := s3.NewPresignClient(r2.client)
	presignResult, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(r2.config.Bucket),
		Key:    aws.String(fullObjectKey),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("生成预签名上传 URL 失败: %v", err)
	}

	return presignResult.URL, nil
}

// GetPresignedDownloadURL 生成预签名下载 URL
func (r2 *CloudflareR2) GetPresignedDownloadURL(ctx context.Context, objectKey string, expiresIn time.Duration) (string, error) {
	objectKey = strings.TrimLeft(objectKey, "/")

	// 构建完整的 object key
	basePath := strings.Trim(r2.config.BasePath, "/")
	fullObjectKey := fmt.Sprintf("%s/%s", basePath, objectKey)
	fullObjectKey = strings.TrimLeft(fullObjectKey, "/")

	// 生成预签名 URL
	presignClient := s3.NewPresignClient(r2.client)
	presignResult, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(r2.config.Bucket),
		Key:    aws.String(fullObjectKey),
	}, s3.WithPresignExpires(expiresIn))
	if err != nil {
		return "", fmt.Errorf("生成预签名下载 URL 失败: %v", err)
	}

	return presignResult.URL, nil
}
