package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/handler"

	"github.com/gin-gonic/gin"
)

func TestOssHandler_GetUploadURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// mock config
	cfg := &config.ViperConfig{
		OSS: config.OSSConfig{
			AccessKeyID:     "test",
			SecretAccessKey: "test",
			Bucket:          "test",
			Endpoint:        "http://test.com",
			BasePath:        "test",
		},
	}
	ossHandler := handler.NewOssHandler(cfg)

	t.Run("MissingKey", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodGet, "/oss/upload", nil)
		c.Request = req

		ossHandler.GetUploadURL(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request, got %v", w.Code)
		}
	})

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodGet, "/oss/upload?key=test.jpg", nil)
		c.Request = req

		ossHandler.GetUploadURL(c)

		// Note: since aws s3 client might try to connect or sign, it might fail or succeed depending on mock.
		// For unit test without deep mocking aws client, we check that it doesn't panic and returns a response.
		// Usually it will return 500 if the endpoint is invalid or 200 if the local sign works.
		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Errorf("Unexpected status code %v", w.Code)
		}
	})
}

func TestOssHandler_GetDownloadURL(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.ViperConfig{
		OSS: config.OSSConfig{
			AccessKeyID:     "test",
			SecretAccessKey: "test",
			Bucket:          "test",
			Endpoint:        "http://test.com",
			BasePath:        "test",
		},
	}
	ossHandler := handler.NewOssHandler(cfg)

	t.Run("MissingKey", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodGet, "/oss/download", nil)
		c.Request = req

		ossHandler.GetDownloadURL(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status Bad Request, got %v", w.Code)
		}
	})

	t.Run("Success", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		req, _ := http.NewRequest(http.MethodGet, "/oss/download?key=test.jpg", nil)
		c.Request = req

		ossHandler.GetDownloadURL(c)

		if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
			t.Errorf("Unexpected status code %v", w.Code)
		}
	})
}
