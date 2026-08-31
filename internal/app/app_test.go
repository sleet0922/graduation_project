package app

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/router"

	"github.com/gofiber/fiber/v2"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func testConfig(port string) *config.ViperConfig {
	return &config.ViperConfig{
		Server: config.ServerConfig{Port: port, Mode: "test", AllowedOrigins: "*"},
		Database: config.DatabaseConfig{
			Username: "test_user", Password: "test-password", Host: "127.0.0.1", Port: 5432,
			Dbname: "test_db", Charset: "utf8", AutoMigrate: false,
		},
		OSS:     config.OSSConfig{AccessKeyID: "test-access", SecretAccessKey: "test-secret", Bucket: "test", Endpoint: "https://example.com"},
		JWT:     config.JWTConfig{SecretKey: "test-jwt-secret", AccessTokenExpireSeconds: 60, RefreshTokenExpireSeconds: 120},
		LiveKit: config.LiveKitConfig{URL: "ws://127.0.0.1:7880", APIKey: "test-key", APISecret: "test-secret", TokenExpireSeconds: 60},
		Log:     config.LogConfig{Level: "info", Filename: "logs/test-app.log"},
		Redis:   config.RedisConfig{Addr: "127.0.0.1", Port: 6379, DB: 0},
	}
}

func noopLogger(*config.ViperConfig) (io.Closer, error) {
	return io.NopCloser(nil), nil
}

func TestBootstrapPropagatesFactoryErrors(t *testing.T) {
	want := errors.New("database unavailable")
	_, err := Bootstrap(context.Background(), Options{
		Config:   testConfig("127.0.0.1:18081"),
		SetupLog: noopLogger,
		OpenDB: func(context.Context, *config.ViperConfig) (*gorm.DB, error) {
			return nil, want
		},
	})
	if !errors.Is(err, want) {
		t.Fatalf("Bootstrap error = %v, want wrapped %v", err, want)
	}
}

func TestBootstrapHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Bootstrap(ctx, Options{Config: testConfig("127.0.0.1:18082")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Bootstrap error = %v, want context.Canceled", err)
	}
}

func TestBootstrapValidatesInjectedConfig(t *testing.T) {
	cfg := testConfig("127.0.0.1:18083")
	cfg.Server.Port = "8081"
	_, err := Bootstrap(context.Background(), Options{Config: cfg, SetupLog: noopLogger})
	if err == nil {
		t.Fatal("Bootstrap accepted an invalid injected config")
	}
}

func TestRunGracefulShutdown(t *testing.T) {
	port := freeTCPPort(t)
	application, err := Bootstrap(context.Background(), Options{
		Config:   testConfig(port),
		DB:       &gorm.DB{},
		Redis:    goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"}),
		SetupLog: noopLogger,
		BuildRouter: func(deps router.Dependencies) (*fiber.App, error) {
			return router.NewHealthRouter(), nil
		},
		ShutdownTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- application.Run(ctx) }()
	waitForListener(t, port)
	cancel()
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run returned error after cancellation: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not stop after context cancellation")
	}
	if err := application.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
	_ = application.Redis.Close()
}

func TestRunCancellationBeforeFiberServeDoesNotLeaveListener(t *testing.T) {
	port := freeTCPPort(t)
	listenerHookEntered := make(chan struct{})
	releaseListenerHook := make(chan struct{})
	application, err := Bootstrap(context.Background(), Options{
		Config:   testConfig(port),
		DB:       &gorm.DB{},
		Redis:    goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:1"}),
		SetupLog: noopLogger,
		BuildRouter: func(deps router.Dependencies) (*fiber.App, error) {
			app := router.NewHealthRouter()
			app.Hooks().OnListen(func(fiber.ListenData) error {
				close(listenerHookEntered)
				<-releaseListenerHook
				return nil
			})
			return app, nil
		},
		ShutdownTimeout: time.Second,
	})
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	runErr := make(chan error, 1)
	go func() { runErr <- application.Run(ctx) }()
	select {
	case <-listenerHookEntered:
	case <-time.After(time.Second):
		t.Fatal("Fiber listener hook was not reached")
	}
	cancel()
	close(releaseListenerHook)
	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("Run returned error after early cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after early cancellation")
	}

	conn, err := net.DialTimeout("tcp", port, 100*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		t.Fatal("listener remained reachable after Run returned")
	}
	if err := application.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
	_ = application.Redis.Close()
}

func freeTCPPort(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve test port: %v", err)
	}
	port := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("release test port: %v", err)
	}
	return port
}

func waitForListener(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server did not start listening on %s", address)
}
