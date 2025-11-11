package ports

import (
	"context"
	"time"

	"alex/internal/auth/domain"
)

// UserRepository abstracts persistence for user records.
type UserRepository interface {
	Create(ctx context.Context, user domain.User) (domain.User, error)
	Update(ctx context.Context, user domain.User) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	FindByID(ctx context.Context, id string) (domain.User, error)
}

// IdentityRepository manages third-party identity links.
type IdentityRepository interface {
	Create(ctx context.Context, identity domain.Identity) (domain.Identity, error)
	Update(ctx context.Context, identity domain.Identity) (domain.Identity, error)
	FindByProvider(ctx context.Context, provider domain.ProviderType, providerID string) (domain.Identity, error)
}

// SessionRepository stores refresh-token backed sessions.
type SessionRepository interface {
	Create(ctx context.Context, session domain.Session) (domain.Session, error)
	DeleteByID(ctx context.Context, id string) error
	DeleteByUser(ctx context.Context, userID string) error
	FindByRefreshToken(ctx context.Context, refreshToken string) (domain.Session, error)
}

// TokenManager issues and validates application JWTs.
type TokenManager interface {
	GenerateAccessToken(ctx context.Context, user domain.User, sessionID string) (token string, expiresAt time.Time, err error)
	GenerateRefreshToken(ctx context.Context) (plain string, hashed string, err error)
	ParseAccessToken(ctx context.Context, token string) (domain.Claims, error)
	HashRefreshToken(token string) (string, error)
	VerifyRefreshToken(token, encodedHash string) (bool, error)
}

// OAuthProvider exchanges codes and builds authorization URLs.
type OAuthProvider interface {
	Provider() domain.ProviderType
	BuildAuthURL(state string) (string, error)
	Exchange(ctx context.Context, code string) (OAuthUserInfo, error)
}

// OAuthUserInfo describes the identity information returned by a provider.
type OAuthUserInfo struct {
	ProviderID   string
	Email        string
	DisplayName  string
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
	Scopes       []string
}

// StateStore keeps transient OAuth state nonces.
type StateStore interface {
	Save(ctx context.Context, state string, provider domain.ProviderType, expiresAt time.Time) error
	Consume(ctx context.Context, state string, provider domain.ProviderType) error
}
