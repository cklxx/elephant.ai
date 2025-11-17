package domain

import (
	"sort"
	"time"
)

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

// SubscriptionTier describes the available subscription plans.
type SubscriptionTier string

const (
	// SubscriptionTierFree represents the default free tier.
	SubscriptionTierFree SubscriptionTier = "free"
	// SubscriptionTierSupporter represents the $20/month supporter tier.
	SubscriptionTierSupporter SubscriptionTier = "supporter"
	// SubscriptionTierProfessional represents the $100/month professional tier.
	SubscriptionTierProfessional SubscriptionTier = "professional"
)

// SubscriptionPlan captures metadata about a subscription tier.
type SubscriptionPlan struct {
	Tier              SubscriptionTier
	DisplayName       string
	MonthlyPriceCents int
	Currency          string
	IsActive          bool
	Metadata          map[string]any
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

var subscriptionCatalog = map[SubscriptionTier]SubscriptionPlan{
	SubscriptionTierFree: {
		Tier:              SubscriptionTierFree,
		DisplayName:       "Free",
		MonthlyPriceCents: 0,
		Currency:          "USD",
		IsActive:          true,
		Metadata:          map[string]any{},
	},
	SubscriptionTierSupporter: {
		Tier:              SubscriptionTierSupporter,
		DisplayName:       "Supporter",
		MonthlyPriceCents: 2000,
		Currency:          "USD",
		IsActive:          true,
		Metadata:          map[string]any{},
	},
	SubscriptionTierProfessional: {
		Tier:              SubscriptionTierProfessional,
		DisplayName:       "Professional",
		MonthlyPriceCents: 10000,
		Currency:          "USD",
		IsActive:          true,
		Metadata:          map[string]any{},
	},
}

// Plan returns the catalog entry for the tier. Unknown tiers resolve to a zero-value plan.
func (t SubscriptionTier) Plan() SubscriptionPlan {
	if plan, ok := subscriptionCatalog[t]; ok {
		return cloneSubscriptionPlan(plan)
	}
	return SubscriptionPlan{Tier: t, Currency: "USD", IsActive: false, Metadata: map[string]any{}}
}

// IsPaid reports whether the tier requires payment.
func (t SubscriptionTier) IsPaid() bool {
	plan := t.Plan()
	return plan.MonthlyPriceCents > 0
}

// IsValid reports whether the tier exists in the catalog.
func (t SubscriptionTier) IsValid() bool {
	_, ok := subscriptionCatalog[t]
	return ok
}

// SubscriptionPlans returns a snapshot of available plans.
func SubscriptionPlans() []SubscriptionPlan {
	plans := make([]SubscriptionPlan, 0, len(subscriptionCatalog))
	for _, plan := range subscriptionCatalog {
		plans = append(plans, cloneSubscriptionPlan(plan))
	}
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].MonthlyPriceCents == plans[j].MonthlyPriceCents {
			return plans[i].Tier < plans[j].Tier
		}
		return plans[i].MonthlyPriceCents < plans[j].MonthlyPriceCents
	})
	return plans
}

func cloneSubscriptionPlan(plan SubscriptionPlan) SubscriptionPlan {
	cloned := plan
	if plan.Metadata == nil {
		cloned.Metadata = map[string]any{}
		return cloned
	}
	meta := make(map[string]any, len(plan.Metadata))
	for key, value := range plan.Metadata {
		meta[key] = value
	}
	cloned.Metadata = meta
	return cloned
}

// User represents a person who can access the platform.
type User struct {
	ID                    string
	Email                 string
	DisplayName           string
	Status                UserStatus
	PasswordHash          string
	PointsBalance         int64
	SubscriptionTier      SubscriptionTier
	SubscriptionExpiresAt *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
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
	ID                      string
	UserID                  string
	RefreshTokenHash        string
	RefreshTokenFingerprint string
	UserAgent               string
	IP                      string
	CreatedAt               time.Time
	ExpiresAt               time.Time
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
