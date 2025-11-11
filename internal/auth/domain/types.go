package domain

import "time"

// ProviderType identifies an external identity provider.
type ProviderType string

const (
	// ProviderLocal represents local username/password accounts.
	ProviderLocal ProviderType = "local"
	// ProviderGoogle represents Google OAuth accounts.
	ProviderGoogle ProviderType = "google"
	// ProviderWeChat represents WeChat OAuth accounts.
	ProviderWeChat ProviderType = "wechat"
)

// UserStatus represents the lifecycle state of an account.
type UserStatus string

const (
	// UserStatusActive indicates a usable account.
	UserStatusActive UserStatus = "active"
	// UserStatusDisabled indicates the account is disabled and cannot sign in.
	UserStatusDisabled UserStatus = "disabled"
)

// User represents a person who can access the platform.
type User struct {
	ID           string
	Email        string
	DisplayName  string
	Status       UserStatus
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Identity links a user to a third-party identity provider.
type Identity struct {
	ID         string
	UserID     string
	Provider   ProviderType
	ProviderID string
	Tokens     OAuthTokens
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// OAuthTokens captures third-party token material.
type OAuthTokens struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	Scopes       []string
}

// Session represents a refresh-token backed login session.
type Session struct {
	ID               string
	UserID           string
	RefreshTokenHash string
	UserAgent        string
	IP               string
	CreatedAt        time.Time
	ExpiresAt        time.Time
}

// Claims represents JWT payload extracted from issued access tokens.
type Claims struct {
	Subject   string
	Email     string
	SessionID string
	ExpiresAt time.Time
}

// TokenPair bundles issued tokens together with expiry metadata.
type TokenPair struct {
	AccessToken   string
	AccessExpiry  time.Time
	RefreshToken  string
	RefreshExpiry time.Time
}
