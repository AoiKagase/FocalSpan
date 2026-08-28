# Authsample fixture

This small repository models an authentication service, its HTTP middleware
caller, configuration, tests, and an unrelated report package. An expired
token is rejected by `auth.Service.ValidateToken` with `auth.ErrExpired`.
