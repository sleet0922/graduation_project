package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"

	"sleet0922/graduation_project/internal/config"
)

func TestClientSessionStoreUsesBoundClient(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = client.Close()
	})

	store, err := NewSessionStore(client)
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	if _, err := store.SetUserSession(7, "session-a", time.Hour); err != nil {
		t.Fatalf("SetUserSession: %v", err)
	}
	if got, err := store.GetUserSession(7); err != nil || got != "session-a" {
		t.Fatalf("GetUserSession = %q, %v", got, err)
	}
	valid, err := store.IsSessionValid(7, "session-a")
	if err != nil || !valid {
		t.Fatalf("valid session = %v, %v", valid, err)
	}
	valid, err = store.IsSessionValid(7, "wrong")
	if err != nil || valid {
		t.Fatalf("wrong session validity = %v, %v", valid, err)
	}

	if err := store.SetRefreshTokenID(7, "session-a", "refresh-a", time.Hour); err != nil {
		t.Fatalf("SetRefreshTokenID: %v", err)
	}
	if err := store.RotateRefreshTokenID(7, "session-a", "refresh-a", "refresh-b", time.Hour); err != nil {
		t.Fatalf("RotateRefreshTokenID: %v", err)
	}
	if err := store.RotateRefreshTokenID(7, "session-a", "refresh-a", "refresh-c", time.Hour); err == nil {
		t.Fatal("stale refresh token was accepted")
	}
	if err := store.DelUserSession(7); err != nil {
		t.Fatalf("DelUserSession: %v", err)
	}
	if got, err := store.GetUserSession(7); err != nil || got != "" {
		t.Fatalf("session after delete = %q, %v", got, err)
	}
}

func TestSetUserSessionReturnsPreviousValueAndRefreshesTTL(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store, err := NewSessionStore(client)
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}

	old, err := store.SetUserSession(42, "session-first", 2*time.Minute)
	if err != nil {
		t.Fatalf("first SetUserSession: %v", err)
	}
	if old != "" {
		t.Fatalf("first SetUserSession returned old value %q, want empty", old)
	}
	old, err = store.SetUserSession(42, "session-second", 3*time.Minute)
	if err != nil {
		t.Fatalf("replacement SetUserSession: %v", err)
	}
	if old != "session-first" {
		t.Fatalf("replacement SetUserSession returned %q, want session-first", old)
	}
	ttl := server.TTL("user:session:42")
	if ttl <= 0 || ttl > 3*time.Minute {
		t.Fatalf("session TTL = %s, want a positive value no greater than 3m", ttl)
	}
}

func TestNewSessionStoreRejectsNilClient(t *testing.T) {
	if _, err := NewSessionStore(nil); err == nil {
		t.Fatal("NewSessionStore accepted a nil client")
	}
}

func TestOpenRejectsInvalidDatabaseIndexBeforeDial(t *testing.T) {
	cfg := &config.ViperConfig{Redis: config.RedisConfig{Addr: "127.0.0.1", Port: 6379, DB: 16}}
	if _, err := Open(context.Background(), cfg); err == nil {
		t.Fatal("Open accepted an invalid Redis database index")
	}
}

func TestClientSessionStoreRejectsInvalidSessionInputs(t *testing.T) {
	server := miniredis.RunT(t)
	client := goredis.NewClient(&goredis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store, err := NewSessionStore(client)
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	if _, err := store.SetUserSession(1, "", time.Minute); err == nil {
		t.Fatal("empty session id was accepted")
	}
	if _, err := store.SetUserSession(1, "session", 0); err == nil {
		t.Fatal("non-positive session TTL was accepted")
	}
}
