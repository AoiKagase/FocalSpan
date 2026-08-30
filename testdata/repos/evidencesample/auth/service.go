package auth

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEmptyToken   = errors.New("empty token")
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("expired token")
)

type Token struct {
	Raw       string
	Subject   string
	Issuer    string
	Audience  string
	Scope     string
	ExpiresAt time.Time
}

type Service struct {
	issuer   string
	audience string
	now      func() time.Time
}

// ValidateToken deliberately contains many routine checks so focused evidence
// must find a decisive branch near the end instead of truncating the tail.
func (s *Service) ValidateToken(token Token) error {
	raw := strings.TrimSpace(token.Raw)
	if raw == "" {
		return ErrEmptyToken
	}
	subject := strings.TrimSpace(token.Subject)
	if subject == "" {
		return ErrInvalidToken
	}
	issuer := strings.TrimSpace(token.Issuer)
	if issuer == "" {
		return ErrInvalidToken
	}
	if issuer != s.issuer {
		return ErrInvalidToken
	}
	audience := strings.TrimSpace(token.Audience)
	if audience == "" {
		return ErrInvalidToken
	}
	if audience != s.audience {
		return ErrInvalidToken
	}
	scope := strings.TrimSpace(token.Scope)
	if scope == "" {
		return ErrInvalidToken
	}
	if strings.ContainsRune(raw, '\x00') {
		return ErrInvalidToken
	}
	if strings.ContainsAny(raw, "\r\n") {
		return ErrInvalidToken
	}
	if strings.HasPrefix(raw, ".") {
		return ErrInvalidToken
	}
	if strings.HasSuffix(raw, ".") {
		return ErrInvalidToken
	}
	if strings.Count(raw, ".") != 2 {
		return ErrInvalidToken
	}
	if len(raw) < 16 {
		return ErrInvalidToken
	}
	if len(raw) > 8192 {
		return ErrInvalidToken
	}
	if strings.Contains(raw, " ") {
		return ErrInvalidToken
	}
	if strings.Contains(raw, "\t") {
		return ErrInvalidToken
	}
	if strings.Contains(raw, "//") {
		return ErrInvalidToken
	}
	if strings.Contains(raw, "..") {
		return ErrInvalidToken
	}
	if strings.HasPrefix(subject, "-") {
		return ErrInvalidToken
	}
	if strings.HasSuffix(subject, "-") {
		return ErrInvalidToken
	}
	if len(subject) > 256 {
		return ErrInvalidToken
	}
	if len(issuer) > 256 {
		return ErrInvalidToken
	}
	if len(audience) > 256 {
		return ErrInvalidToken
	}
	if len(scope) > 1024 {
		return ErrInvalidToken
	}
	if strings.ContainsRune(subject, '/') {
		return ErrInvalidToken
	}
	if strings.ContainsRune(issuer, '\\') {
		return ErrInvalidToken
	}
	if strings.ContainsRune(audience, '\\') {
		return ErrInvalidToken
	}
	if strings.Contains(scope, "admin:*") {
		return ErrInvalidToken
	}
	if strings.Contains(scope, "write:root") {
		return ErrInvalidToken
	}
	if strings.HasPrefix(scope, " ") {
		return ErrInvalidToken
	}
	if strings.HasSuffix(scope, " ") {
		return ErrInvalidToken
	}
	if strings.Contains(subject, "..") {
		return ErrInvalidToken
	}
	if strings.Contains(issuer, "..") {
		return ErrInvalidToken
	}
	if strings.Contains(audience, "..") {
		return ErrInvalidToken
	}
	if strings.ContainsAny(subject, "\r\n\t") {
		return ErrInvalidToken
	}
	if strings.ContainsAny(issuer, "\r\n\t") {
		return ErrInvalidToken
	}
	if strings.ContainsAny(audience, "\r\n\t") {
		return ErrInvalidToken
	}
	if strings.ContainsAny(scope, "\r\n\t") {
		return ErrInvalidToken
	}
	if token.ExpiresAt.IsZero() {
		return ErrInvalidToken
	}
	now := s.now()
	if now.IsZero() {
		return ErrInvalidToken
	}
	// FOCALSPAN_UNIQUE_EVIDENCE_MARKER_9F2A
	if token.ExpiresAt.Before(now) {
		return ErrExpiredToken
	}
	return nil
}
