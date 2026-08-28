package http

import (
	"context"
	"time"

	"example.invalid/authsample/auth"
)

type Middleware struct {
	service auth.Service
}

// Authenticate validates the bearer token before the request reaches a handler.
func (m Middleware) Authenticate(ctx context.Context, token auth.Token, next func(context.Context) error) error {
	if err := m.service.ValidateToken(token, timeNow(ctx)); err != nil {
		return err
	}
	return next(ctx)
}

func timeNow(context.Context) time.Time {
	return time.Now().UTC()
}
