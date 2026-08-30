package auth

import (
	"errors"
	"testing"
	"time"
)

func TestValidateTokenRejectsExpiredToken(t *testing.T) {
	now := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	service := Service{issuer: "issuer", audience: "api", now: func() time.Time { return now }}
	token := Token{Raw: "header.payload.signature", Subject: "user", Issuer: "issuer", Audience: "api", Scope: "read", ExpiresAt: now.Add(-time.Second)}
	if err := service.ValidateToken(token); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("ValidateToken() error = %v, want ErrExpiredToken", err)
	}
}
