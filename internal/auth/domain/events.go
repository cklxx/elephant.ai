package domain

import "time"

// SubscriptionEventType enumerates subscription lifecycle milestones.
type SubscriptionEventType string

const (
	// SubscriptionEventCreated occurs when a subscription is inserted.
	SubscriptionEventCreated SubscriptionEventType = "subscription.created"
	// SubscriptionEventUpdated occurs when subscription metadata changes (tier, status, renewal windows).
	SubscriptionEventUpdated SubscriptionEventType = "subscription.updated"
	// SubscriptionEventCanceled occurs when auto-renew is disabled or the record expires immediately.
	SubscriptionEventCanceled SubscriptionEventType = "subscription.canceled"
)

// SubscriptionEvent captures state transitions that should fan out to other systems.
type SubscriptionEvent struct {
	Type          SubscriptionEventType
	Subscription  Subscription
	PreviousTier  SubscriptionTier
	OccurredAt    time.Time
	CorrelationID string
}

// PointsEventType enumerates supported ledger events.
type PointsEventType string

const (
	// PointsEventCredited fires when delta > 0 is appended.
	PointsEventCredited PointsEventType = "points.credited"
	// PointsEventDebited fires when delta < 0 is appended.
	PointsEventDebited PointsEventType = "points.debited"
)

// PointsLedgerEvent represents an immutable debit/credit entry broadcast.
type PointsLedgerEvent struct {
	Type          PointsEventType
	Entry         PointsLedgerEntry
	BalanceBefore *int64
	OccurredAt    time.Time
	CorrelationID string
}
