package rtc

import (
	"testing"

	"github.com/livekit/protocol/auth"
)

func TestGenerateTokenCreatesScopedLiveKitJWT(t *testing.T) {
	token, err := GenerateToken("api-key", "api-secret", "room-1", "42", 3600)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	verifier, err := auth.ParseAPIToken(token)
	if err != nil {
		t.Fatalf("ParseAPIToken failed: %v", err)
	}
	if verifier.APIKey() != "api-key" {
		t.Fatalf("API key = %q, want api-key", verifier.APIKey())
	}
	_, grants, err := verifier.Verify("api-secret")
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if grants.Identity != "42" || grants.Video == nil || !grants.Video.RoomJoin || grants.Video.Room != "room-1" {
		t.Fatalf("grants = %#v, want identity 42 scoped to room-1", grants)
	}
}

func TestGenerateTokenRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name       string
		apiKey     string
		apiSecret  string
		roomID     string
		identity   string
		expiresSec int
	}{
		{name: "missing key", apiSecret: "secret", roomID: "room", identity: "1", expiresSec: 1},
		{name: "missing secret", apiKey: "key", roomID: "room", identity: "1", expiresSec: 1},
		{name: "missing room", apiKey: "key", apiSecret: "secret", identity: "1", expiresSec: 1},
		{name: "missing identity", apiKey: "key", apiSecret: "secret", roomID: "room", expiresSec: 1},
		{name: "invalid lifetime", apiKey: "key", apiSecret: "secret", roomID: "room", identity: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := GenerateToken(tt.apiKey, tt.apiSecret, tt.roomID, tt.identity, tt.expiresSec); err == nil {
				t.Fatal("GenerateToken succeeded, want error")
			}
		})
	}
}
