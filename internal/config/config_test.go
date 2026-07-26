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
}
