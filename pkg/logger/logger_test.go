package logger

import (
	"path/filepath"
	"testing"

	"sleet0922/graduation_project/internal/config"
)

func TestNewLoggerRequiresFilename(t *testing.T) {
	_, _, err := New(&config.ViperConfig{})
	if err == nil {
		t.Fatal("New accepted an empty log filename")
	}
}

func TestNewLoggerCloseIsIdempotent(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "nested", "app.log")
	log, closer, err := New(&config.ViperConfig{
		Server: config.ServerConfig{Mode: "test"},
		Log:    config.LogConfig{Filename: filename},
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if log == nil || closer == nil {
		t.Fatal("New returned nil logger or closer")
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	if err := closer.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}
