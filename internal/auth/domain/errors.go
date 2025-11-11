package domain

import "errors"

var (
	// ErrUserExists indicates a user already exists with the provided identifier.
	ErrUserExists = errors.New("user already exists")
	// ErrUserNotFound indicates that the user could not be located.
	ErrUserNotFound = errors.New("user not found")
	// ErrIdentityNotFound indicates a third-party identity link was not found.
	ErrIdentityNotFound = errors.New("identity not found")
	// ErrInvalidCredentials indicates username/password authentication failure.
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrProviderNotConfigured indicates the requested OAuth provider is unavailable.
	ErrProviderNotConfigured = errors.New("provider not configured")
	// ErrStateMismatch indicates OAuth state validation failure.
	ErrStateMismatch = errors.New("oauth state mismatch")
	// ErrSessionNotFound indicates refresh session missing.
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired indicates the refresh token is expired.
	ErrSessionExpired = errors.New("session expired")
)
