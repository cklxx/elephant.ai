package domain

import "time"

// SubscriptionStatus represents the lifecycle of a paid subscription.
type SubscriptionStatus string

const (
	// SubscriptionStatusActive indicates an auto-renewing subscription in good standing.
	SubscriptionStatusActive SubscriptionStatus = "active"
	// SubscriptionStatusPastDue indicates payment failures and grace periods.
	SubscriptionStatusPastDue SubscriptionStatus = "past_due"
	// SubscriptionStatusCanceled indicates the subscription will not renew.
	SubscriptionStatusCanceled SubscriptionStatus = "canceled"
)

// Subscription captures the authoritative subscription row persisted in auth_subscriptions.
type Subscription struct {
	ID                     string
	UserID                 string
	Tier                   SubscriptionTier
	Status                 SubscriptionStatus
	AutoRenew              bool
	CurrentPeriodStart     time.Time
	CurrentPeriodEnd       *time.Time
	ExternalCustomerID     string
	ExternalSubscriptionID string
	ExternalPlanID         string
	Metadata               map[string]any
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

// IsActive returns true when the subscription should grant entitlements.
func (s Subscription) IsActive(now time.Time) bool {
	if s.Status == SubscriptionStatusCanceled || s.Status == SubscriptionStatusPastDue {
		if s.CurrentPeriodEnd == nil {
			return false
		}
		return now.Before(*s.CurrentPeriodEnd)
	}
	if s.CurrentPeriodEnd == nil {
		return true
	}
	return now.Before(*s.CurrentPeriodEnd)
}

// PointsLedgerEntry records a single debit or credit mutation.
type PointsLedgerEntry struct {
	ID           string
	UserID       string
	Delta        int64
	BalanceAfter *int64
	Reason       string
	Metadata     map[string]any
	CreatedAt    time.Time
}

// Promotion describes a temporary discount or bonus surfaced to a user.
type Promotion struct {
	Code            string
	Description     string
	PointsBonus     int64
	DiscountPercent float64
	ExpiresAt       *time.Time
}

// IsExpired reports whether the promotion is no longer valid at the provided time.
func (p Promotion) IsExpired(now time.Time) bool {
	if p.ExpiresAt == nil {
		return false
	}
	return !now.Before(*p.ExpiresAt)
}
