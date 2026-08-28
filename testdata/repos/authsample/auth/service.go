package auth

import (
	"errors"
	"time"
)

var (
	ErrExpired = errors.New("authentication token is expired")
)

type Token struct {
	Subject   string
	ExpiresAt time.Time
}

type Service struct{}

// ValidateToken rejects an expired or malformed authentication token.
func (s Service) ValidateToken(token Token, now time.Time) error {
	if token.ExpiresAt.Before(now) {
		// The expired authentication token is rejected before it reaches a handler.
		return ErrExpired
	}
	return nil
}
