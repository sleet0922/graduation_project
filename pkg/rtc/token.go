package rtc

import (
	"fmt"
	"strings"
	"time"

	"github.com/livekit/protocol/auth"
)

// GenerateToken generates a LiveKit JWT token for a participant to join a room.
func GenerateToken(apiKey, apiSecret, roomID, participantIdentity string, expireSeconds int) (string, error) {
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(apiSecret) == "" {
		return "", fmt.Errorf("livekit API key and secret are required")
	}
	if strings.TrimSpace(roomID) == "" {
		return "", fmt.Errorf("livekit room ID is required")
	}
	if strings.TrimSpace(participantIdentity) == "" {
		return "", fmt.Errorf("livekit participant identity is required")
	}
	if expireSeconds <= 0 {
		return "", fmt.Errorf("livekit token lifetime must be positive")
	}

	at := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{
		RoomJoin: true,
		Room:     roomID,
	}
	at.SetVideoGrant(grant).
		SetIdentity(participantIdentity).
		SetValidFor(time.Duration(expireSeconds) * time.Second)

	return at.ToJWT()
}
