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

// StateStoreWithCleanup extends StateStore with the ability to purge expired records.
type StateStoreWithCleanup interface {
	StateStore
	PurgeExpired(ctx context.Context, before time.Time) (int64, error)
}

// PlanRepository manages subscription catalog metadata sourced from auth_plans.
type PlanRepository interface {
	List(ctx context.Context, includeInactive bool) ([]domain.SubscriptionPlan, error)
	FindByTier(ctx context.Context, tier domain.SubscriptionTier) (domain.SubscriptionPlan, error)
	Upsert(ctx context.Context, plan domain.SubscriptionPlan) error
}

// SubscriptionRepository persists per-user subscription lifecycles backed by auth_subscriptions.
type SubscriptionRepository interface {
	Create(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error)
	Update(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error)
	FindActiveByUser(ctx context.Context, userID string) (domain.Subscription, error)
	ListExpiring(ctx context.Context, before time.Time) ([]domain.Subscription, error)
}

// PointsLedgerRepository stores immutable debit/credit rows and exposes balance helpers.
type PointsLedgerRepository interface {
	AppendEntry(ctx context.Context, entry domain.PointsLedgerEntry) (domain.PointsLedgerEntry, error)
	ListEntries(ctx context.Context, userID string, limit int) ([]domain.PointsLedgerEntry, error)
	LatestBalance(ctx context.Context, userID string) (int64, error)
}

// PaymentGatewayPort encapsulates the remote billing platform (e.g., Stripe, WeChat Pay).
type PaymentGatewayPort interface {
	EnsureCustomer(ctx context.Context, user domain.User) (customerID string, err error)
	CreateSubscription(ctx context.Context, customerID string, plan domain.SubscriptionPlan) (gatewaySubscriptionID string, err error)
	CancelSubscription(ctx context.Context, gatewaySubscriptionID string) error
}

// PromotionFeedPort exposes promotional campaigns that may affect subscriptions or points bonuses.
type PromotionFeedPort interface {
	ListActive(ctx context.Context, userID string) ([]domain.Promotion, error)
}

// EventPublisher fan-outs domain events to downstream systems (e.g., message buses).
type EventPublisher interface {
	PublishSubscriptionEvent(ctx context.Context, event domain.SubscriptionEvent) error
	PublishPointsEvent(ctx context.Context, event domain.PointsLedgerEvent) error
}
