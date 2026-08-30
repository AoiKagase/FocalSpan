package http

import "example.invalid/evidencesample/auth"

type Middleware struct{ Service *auth.Service }

func (m Middleware) Authenticate(token auth.Token) error {
	return m.Service.ValidateToken(token)
}
