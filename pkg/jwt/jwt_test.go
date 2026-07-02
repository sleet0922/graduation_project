package jwt

import (
	"testing"
	"time"
)

func TestGenerateRefreshTokenCarriesRefreshID(t *testing.T) {
	manager := NewJWTManager("test-secret")

	token, err := manager.GenerateTokenWithSessionAndRefreshID(
		42,
		"user42",
		TokenTypeRefresh,
		"session-1",
		"refresh-1",
		time.Hour,
	)
	if err != nil {
		t.Fatalf("GenerateTokenWithSessionAndRefreshID failed: %v", err)
	}

	claims, err := manager.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.UserID != 42 {
		t.Fatalf("UserID = %d, want 42", claims.UserID)
	}
	if claims.TokenType != TokenTypeRefresh {
		t.Fatalf("TokenType = %q, want %q", claims.TokenType, TokenTypeRefresh)
	}
	if claims.SessionID != "session-1" {
		t.Fatalf("SessionID = %q, want session-1", claims.SessionID)
	}
	if claims.RefreshID != "refresh-1" {
		t.Fatalf("RefreshID = %q, want refresh-1", claims.RefreshID)
	}
}

func TestGenerateTokenIDReturnsUniqueIDs(t *testing.T) {
	first, err := GenerateTokenID()
	if err != nil {
		t.Fatalf("GenerateTokenID first failed: %v", err)
	}
	second, err := GenerateTokenID()
	if err != nil {
		t.Fatalf("GenerateTokenID second failed: %v", err)
	}

	if len(first) != 32 || len(second) != 32 {
		t.Fatalf("token id lengths = %d and %d, want 32", len(first), len(second))
	}
	if first == second {
		t.Fatal("GenerateTokenID returned duplicate ids")
	}
}
