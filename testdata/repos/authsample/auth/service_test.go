package auth

import (
	"testing"
	"time"
)

func TestValidateExpiredToken(t *testing.T) {
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	token := Token{Subject: "alice", ExpiresAt: now.Add(-time.Minute)}
	// An expired token must be rejected by the authentication service.
	if err := (Service{}).ValidateToken(token, now); err != ErrExpired {
		t.Fatalf("ValidateToken() error = %v, want %v", err, ErrExpired)
	}
}
