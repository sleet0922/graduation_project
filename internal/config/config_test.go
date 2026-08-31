package config

import (
	"os"
	"path/filepath"
	"testing"
)

const testConfig = `
server: {port: ":8081", mode: release}
database: {username: zat, password: change-me, host: 127.0.0.1, port: 5432, dbname: zat, charset: utf8, auto_migrate: false}
oss: {access_key_id: change-me, secret_access_key: change-me, bucket: test, endpoint: "https://example.com", base_path: "", cdn_domain: "https://example.com"}
jwt: {secret_key: SET_ZAT_JWT_SECRET_IN_ENVIRONMENT, access_token_expire_seconds: 3600, refresh_token_expire_seconds: 7200}
livekit: {url: http://localhost:7880, api_key: devkey, api_secret: secret, token_expire_seconds: 3600}
log: {level: info, filename: app.log}
redis: {addr: 127.0.0.1, port: 6379, password: "", db: 0}
`

func writeTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(testConfig), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return path
}

func TestLoadConfigRejectsPlaceholderSecrets(t *testing.T) {
	_, err := LoadConfig(writeTestConfig(t))
	if err == nil {
		t.Fatal("LoadConfig accepted placeholder credentials")
	}
}

func TestLoadConfigUsesEnvironmentSecrets(t *testing.T) {
	t.Setenv("ZAT_DATABASE_PASSWORD", "database-test-password")
	t.Setenv("ZAT_JWT_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("ZAT_OSS_ACCESS_KEY_ID", "test-access")
	t.Setenv("ZAT_OSS_SECRET_ACCESS_KEY", "test-secret")
	t.Setenv("ZAT_LIVEKIT_URL", "http://test")
	t.Setenv("ZAT_LIVEKIT_API_KEY", "test-key")
	t.Setenv("ZAT_LIVEKIT_API_SECRET", "test-secret")
	t.Setenv("ZAT_DATABASE_AUTO_MIGRATE", "true")
	t.Setenv("ZAT_SERVER_ALLOWED_ORIGINS", "https://client.example")

	config, err := LoadConfig(writeTestConfig(t))
	if err != nil {
		t.Fatalf("LoadConfig returned an error: %v", err)
	}
	if config.Database.Password != "database-test-password" || !config.Database.AutoMigrate {
		t.Fatalf("database environment overrides were not applied: %#v", config.Database)
	}
	if config.JWT.SecretKey != "0123456789abcdef0123456789abcdef" {
		t.Fatal("JWT environment override was not applied")
	}
	if config.Server.AllowedOrigins != "https://client.example" {
		t.Fatalf("server allowed origins override was not applied: %q", config.Server.AllowedOrigins)
	}
}

func TestLoadConfigRejectsInvalidInfrastructureValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*ViperConfig)
	}{
		{name: "server port", mutate: func(cfg *ViperConfig) { cfg.Server.Port = "8081" }},
		{name: "database host", mutate: func(cfg *ViperConfig) { cfg.Database.Host = " " }},
		{name: "database port", mutate: func(cfg *ViperConfig) { cfg.Database.Port = 70000 }},
		{name: "database username", mutate: func(cfg *ViperConfig) { cfg.Database.Username = "user-name" }},
		{name: "database name", mutate: func(cfg *ViperConfig) { cfg.Database.Dbname = "123database" }},
		{name: "redis port", mutate: func(cfg *ViperConfig) { cfg.Redis.Port = 0 }},
		{name: "log filename", mutate: func(cfg *ViperConfig) { cfg.Log.Filename = "" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &ViperConfig{
				Server:   ServerConfig{Port: ":8081", Mode: "release"},
				Database: DatabaseConfig{Host: "127.0.0.1", Username: "user", Dbname: "db", Port: 5432, Password: "password"},
				Redis:    RedisConfig{Addr: "127.0.0.1", Port: 6379},
				Log:      LogConfig{Filename: "app.log", Level: "info"},
				JWT:      JWTConfig{SecretKey: "jwt-secret", AccessTokenExpireSeconds: 60, RefreshTokenExpireSeconds: 120},
				OSS:      OSSConfig{AccessKeyID: "access", SecretAccessKey: "secret"},
				LiveKit:  LiveKitConfig{URL: "http://localhost:7880", APIKey: "key", APISecret: "secret", TokenExpireSeconds: 1},
			}
			test.mutate(cfg)
			if err := validateConfig(cfg); err == nil {
				t.Fatalf("validateConfig accepted invalid %s", test.name)
			}
		})
	}
}

func TestLoadConfigRejectsInsecureDefaultsOutsideDevelopment(t *testing.T) {
	t.Setenv("ZAT_ALLOW_INSECURE_DEFAULTS", "")
	cfg := &ViperConfig{
		Server:   ServerConfig{Port: ":8081", Mode: "release"},
		Database: DatabaseConfig{Host: "127.0.0.1", Username: "user", Dbname: "db", Port: 5432, Password: "local-development-password"},
		Redis:    RedisConfig{Addr: "127.0.0.1", Port: 6379},
		Log:      LogConfig{Filename: "app.log", Level: "info"},
		JWT:      JWTConfig{SecretKey: "jwt-secret", AccessTokenExpireSeconds: 60, RefreshTokenExpireSeconds: 120},
		OSS:      OSSConfig{AccessKeyID: "access", SecretAccessKey: "secret"},
		LiveKit:  LiveKitConfig{URL: "http://localhost:7880", APIKey: "key", APISecret: "secret", TokenExpireSeconds: 1},
	}
	if err := validateConfig(cfg); err == nil {
		t.Fatal("validateConfig accepted insecure development default")
	}
}

func TestLoadConfigDevelopmentDefaultsRequireExplicitDeploymentGate(t *testing.T) {
	cfg := &ViperConfig{
		Server:   ServerConfig{Port: ":8081", Mode: "release"},
		Database: DatabaseConfig{Host: "127.0.0.1", Username: "user", Dbname: "db", Port: 5432, Password: "local-development-password"},
		Redis:    RedisConfig{Addr: "127.0.0.1", Port: 6379},
		Log:      LogConfig{Filename: filepath.Join(t.TempDir(), "app.log")},
		JWT:      JWTConfig{SecretKey: "local-development-jwt-secret", AccessTokenExpireSeconds: 60, RefreshTokenExpireSeconds: 120},
		OSS:      OSSConfig{AccessKeyID: "local-development-access", SecretAccessKey: "local-development-secret"},
		LiveKit:  LiveKitConfig{URL: "ws://localhost:7880", APIKey: "devkey", APISecret: "local-development-livekit-secret", TokenExpireSeconds: 60},
	}
	t.Setenv("ZAT_ALLOW_INSECURE_DEFAULTS", "1")
	t.Setenv("BOCKER_DEPLOYMENT_MODE", "production")
	if err := validateConfig(cfg); err == nil {
		t.Fatal("production deployment accepted development defaults")
	}
	t.Setenv("BOCKER_DEPLOYMENT_MODE", "development")
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("development deployment rejected explicitly gated defaults: %v", err)
	}
}

func TestLoadConfigRejectsInvalidModesAndOrigins(t *testing.T) {
	base := &ViperConfig{
		Server:   ServerConfig{Port: ":8081", Mode: "release"},
		Database: DatabaseConfig{Host: "127.0.0.1", Username: "user", Dbname: "db", Port: 5432, Password: "password"},
		Redis:    RedisConfig{Addr: "127.0.0.1", Port: 6379},
		Log:      LogConfig{Filename: filepath.Join(t.TempDir(), "app.log"), Level: "info"},
		JWT:      JWTConfig{SecretKey: "a-secure-jwt-secret", AccessTokenExpireSeconds: 60, RefreshTokenExpireSeconds: 120},
		OSS:      OSSConfig{AccessKeyID: "access", SecretAccessKey: "secret"},
		LiveKit:  LiveKitConfig{URL: "ws://localhost:7880", APIKey: "key", APISecret: "secret", TokenExpireSeconds: 60},
	}
	for name, mutate := range map[string]func(*ViperConfig){
		"mode":          func(cfg *ViperConfig) { cfg.Server.Mode = "unknown" },
		"origin":        func(cfg *ViperConfig) { cfg.Server.AllowedOrigins = "https://client.example/path" },
		"log level":     func(cfg *ViperConfig) { cfg.Log.Level = "verbose" },
		"redis db":      func(cfg *ViperConfig) { cfg.Redis.DB = 16 },
		"database host": func(cfg *ViperConfig) { cfg.Database.Host = "[::1" },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := *base
			mutate(&cfg)
			if err := validateConfig(&cfg); err == nil {
				t.Fatalf("validateConfig accepted invalid %s", name)
			}
		})
	}
}

func TestLoadConfigRejectsUnknownDeploymentMode(t *testing.T) {
	cfg := &ViperConfig{
		Server:   ServerConfig{Port: ":8081", Mode: "release"},
		Database: DatabaseConfig{Host: "127.0.0.1", Username: "user", Dbname: "db", Port: 5432, Password: "password"},
		Redis:    RedisConfig{Addr: "127.0.0.1", Port: 6379},
		Log:      LogConfig{Filename: filepath.Join(t.TempDir(), "app.log"), Level: "info"},
		JWT:      JWTConfig{SecretKey: "a-secure-jwt-secret", AccessTokenExpireSeconds: 60, RefreshTokenExpireSeconds: 120},
		OSS:      OSSConfig{AccessKeyID: "access", SecretAccessKey: "secret"},
		LiveKit:  LiveKitConfig{URL: "ws://localhost:7880", APIKey: "key", APISecret: "secret", TokenExpireSeconds: 60},
	}
	t.Setenv("BOCKER_DEPLOYMENT_MODE", "staging")
	if err := validateConfig(cfg); err == nil {
		t.Fatal("validateConfig accepted unknown deployment mode")
	}
}

func TestLoadConfigRejectsMalformedLiveKitURLs(t *testing.T) {
	base := &ViperConfig{
		Server:   ServerConfig{Port: ":8081", Mode: "release"},
		Database: DatabaseConfig{Host: "127.0.0.1", Username: "user", Dbname: "db", Port: 5432, Password: "password"},
		Redis:    RedisConfig{Addr: "127.0.0.1", Port: 6379},
		Log:      LogConfig{Filename: filepath.Join(t.TempDir(), "app.log"), Level: "info"},
		JWT:      JWTConfig{SecretKey: "a-secure-jwt-secret", AccessTokenExpireSeconds: 60, RefreshTokenExpireSeconds: 120},
		OSS:      OSSConfig{AccessKeyID: "access", SecretAccessKey: "secret"},
		LiveKit:  LiveKitConfig{URL: "ws://localhost:7880", APIKey: "key", APISecret: "secret", TokenExpireSeconds: 60},
	}
	for _, value := range []string{
		"ws://localhost:",
		"ws://localhost:0",
		"ws://localhost:65536",
		"ws://user:password@localhost:7880",
		"ws://::1:7880",
		"ws://localhost:7880\n",
	} {
		t.Run(value, func(t *testing.T) {
			cfg := *base
			cfg.LiveKit.URL = value
			if err := validateConfig(&cfg); err == nil {
				t.Fatalf("validateConfig accepted malformed LiveKit URL %q", value)
			}
		})
	}
}
