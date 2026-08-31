package db

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"sleet0922/graduation_project/internal/config"
	"sleet0922/graduation_project/internal/model"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DSN builds the PostgreSQL connection string after validating the fields that
// would otherwise produce an opaque driver error.
func DSN(cfg *config.ViperConfig) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("database config is nil")
	}
	if strings.TrimSpace(cfg.Database.Host) == "" {
		return "", fmt.Errorf("database host is empty")
	}
	if strings.TrimSpace(cfg.Database.Username) == "" {
		return "", fmt.Errorf("database username is empty")
	}
	if strings.TrimSpace(cfg.Database.Dbname) == "" {
		return "", fmt.Errorf("database name is empty")
	}
	if cfg.Database.Port <= 0 || cfg.Database.Port > 65535 {
		return "", fmt.Errorf("database port %d is invalid", cfg.Database.Port)
	}

	host := strings.TrimSpace(cfg.Database.Host)
	username := strings.TrimSpace(cfg.Database.Username)
	dbname := strings.TrimSpace(cfg.Database.Dbname)

	// Use a URL so credentials and database names are escaped by the standard
	// library. Keyword/value DSNs are ambiguous when a password contains spaces
	// or characters such as '=' and '#'.
	dsn := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(host, strconv.Itoa(cfg.Database.Port)),
		User:   url.UserPassword(username, cfg.Database.Password),
		Path:   "/" + dbname,
		// Keep the slash in the timezone value literal. GORM's postgres
		// dialector extracts TimeZone with a regex before pgx parses the URL;
		// it does not URL-decode that capture, so `%2F` would become the
		// invalid timezone name `Asia%2FShanghai` at connection time.
		RawQuery: "TimeZone=Asia/Shanghai&sslmode=disable",
	}
	return dsn.String(), nil
}

// Open opens and verifies PostgreSQL using the caller's context. The returned
// *gorm.DB owns a database/sql pool that must be closed by the caller.
func Open(ctx context.Context, cfg *config.ViperConfig) (*gorm.DB, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	dsn, err := DSN(cfg)
	if err != nil {
		return nil, err
	}

	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	sqlDB, err := database.DB()
	if err != nil {
		return nil, fmt.Errorf("get postgres pool: %w", err)
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	if cfg.Database.AutoMigrate {
		if err := migrate(ctx, database); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
	}
	return database, nil
}

func migrate(ctx context.Context, database *gorm.DB) error {
	if err := database.WithContext(ctx).AutoMigrate(
		&model.User{},
		&model.UserLocation{},
		&model.Friend{},
		&model.FriendRequest{},
		&model.ChatGroup{},
		&model.ChatGroupMember{},
		&model.E2EEUserPublicKey{},
		&model.E2EEGroupKey{},
		&model.E2EEGroupKeyBox{},
		&model.FeedPost{},
		&model.FeedMedia{},
		&model.FeedLike{},
		&model.FeedComment{},
	); err != nil {
		return fmt.Errorf("database migration: %w", err)
	}

	// Older deployments used soft deletes. Fresh schemas do not have this
	// column, so only clean legacy rows when the column is actually present.
	if database.Migrator().HasColumn(&model.FeedLike{}, "deleted_at") {
		if err := database.WithContext(ctx).Exec("DELETE FROM feed_like WHERE deleted_at IS NOT NULL").Error; err != nil {
			return fmt.Errorf("clean feed_like soft-deleted rows: %w", err)
		}
	}
	return nil
}

// Ping verifies the underlying SQL pool and is suitable for readiness checks.
func Ping(ctx context.Context, database *gorm.DB) error {
	if database == nil {
		return fmt.Errorf("database is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("get postgres pool: %w", err)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}
	return nil
}

// Close closes the SQL pool owned by database.
func Close(database *gorm.DB) error {
	if database == nil {
		return nil
	}
	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("get postgres pool: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close postgres: %w", err)
	}
	return nil
}

// InitDB is retained as an error-returning entry point for callers migrating
// from the old fatal initializer. New code should call Open with a context.
func InitDB(cfg *config.ViperConfig) (*gorm.DB, error) {
	return Open(context.Background(), cfg)
}
