package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"sleet0922/graduation_project/internal/config"

	"gorm.io/gorm"
)

func TestHealthRouterLivenessAndReadiness(t *testing.T) {
	app := NewHealthRouter(ReadinessCheck{
		Name: "database",
		Check: func(context.Context) error {
			return nil
		},
	})

	for _, path := range []string{"/health", "/health/live", "/livez"} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			t.Fatalf("GET %s status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
		_ = resp.Body.Close()
	}

	for _, path := range []string{"/health/ready", "/readyz"} {
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			t.Fatalf("GET %s status = %d, want %d", path, resp.StatusCode, http.StatusOK)
		}
		_ = resp.Body.Close()
	}
}

func TestHealthRouterReadinessFailure(t *testing.T) {
	app := NewHealthRouter(ReadinessCheck{
		Name: "redis",
		Check: func(context.Context) error {
			return errors.New("redis unavailable")
		},
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if err != nil {
		t.Fatalf("GET /health/ready: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestNewRouterReportsMissingDependencies(t *testing.T) {
	if _, err := NewRouter(Dependencies{}); err == nil {
		t.Fatal("NewRouter accepted missing dependencies")
	}
}

func TestNewDependenciesRequiresExplicitRedis(t *testing.T) {
	if _, err := NewDependencies(&gorm.DB{}, &config.ViperConfig{}, nil); err == nil {
		t.Fatal("NewDependencies accepted a nil Redis dependency")
	}
}
