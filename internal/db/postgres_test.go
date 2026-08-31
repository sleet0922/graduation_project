package db

import (
	"context"
	"testing"

	"sleet0922/graduation_project/internal/config"

	"github.com/jackc/pgx/v5"
)

func testDatabaseConfig() *config.ViperConfig {
	return &config.ViperConfig{Database: config.DatabaseConfig{
		Username: "user",
		Password: "password with spaces",
		Host:     "localhost",
		Port:     5432,
		Dbname:   "app",
	}}
}

func TestDSN(t *testing.T) {
	dsn, err := DSN(testDatabaseConfig())
	if err != nil {
		t.Fatalf("DSN returned error: %v", err)
	}
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx.ParseConfig(%q): %v", dsn, err)
	}
	if parsed.Host != "localhost" || parsed.Port != 5432 {
		t.Fatalf("parsed address = %s:%d, want localhost:5432", parsed.Host, parsed.Port)
	}
	if parsed.User != "user" {
		t.Fatalf("parsed user = %q, want user", parsed.User)
	}
	if parsed.Password != "password with spaces" {
		t.Fatalf("parsed password = %q, want password with spaces", parsed.Password)
	}
	if parsed.Database != "app" {
		t.Fatalf("parsed database = %q, want app", parsed.Database)
	}
	if parsed.RuntimeParams["TimeZone"] != "Asia/Shanghai" {
		t.Fatalf("parsed TimeZone = %q, want Asia/Shanghai", parsed.RuntimeParams["TimeZone"])
	}
	if parsed.TLSConfig != nil {
		t.Fatalf("parsed TLSConfig = %#v, want nil for sslmode=disable", parsed.TLSConfig)
	}
}

func TestDSNEscapesURLReservedCharacters(t *testing.T) {
	cfg := testDatabaseConfig()
	cfg.Database.Username = "user@domain"
	cfg.Database.Password = "p@ss word/#?&="
	cfg.Database.Dbname = "app database"

	dsn, err := DSN(cfg)
	if err != nil {
		t.Fatalf("DSN returned error: %v", err)
	}
	parsed, err := pgx.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("pgx.ParseConfig(%q): %v", dsn, err)
	}
	if parsed.User != cfg.Database.Username || parsed.Password != cfg.Database.Password || parsed.Database != cfg.Database.Dbname {
		t.Fatalf("parsed credentials = user %q password %q database %q, want %q %q %q", parsed.User, parsed.Password, parsed.Database, cfg.Database.Username, cfg.Database.Password, cfg.Database.Dbname)
	}
}

func TestDSNRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*config.ViperConfig)
	}{
		{name: "nil", mutate: nil},
		{name: "host", mutate: func(cfg *config.ViperConfig) { cfg.Database.Host = " " }},
		{name: "username", mutate: func(cfg *config.ViperConfig) { cfg.Database.Username = "" }},
		{name: "dbname", mutate: func(cfg *config.ViperConfig) { cfg.Database.Dbname = "" }},
		{name: "port", mutate: func(cfg *config.ViperConfig) { cfg.Database.Port = 70000 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cfg *config.ViperConfig
			if tt.name != "nil" {
				cfg = testDatabaseConfig()
				tt.mutate(cfg)
			}
			if _, err := DSN(cfg); err == nil {
				t.Fatal("DSN accepted invalid config")
			}
		})
	}
}

func TestPingRejectsNilDatabase(t *testing.T) {
	if err := Ping(context.Background(), nil); err == nil {
		t.Fatal("Ping accepted nil database")
	}
}
