package adapters

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"

	"alex/internal/auth/domain"
)

// FakePaymentGateway satisfies PaymentGatewayPort for local/test usage.
type FakePaymentGateway struct {
	mu            sync.Mutex
	customers     map[string]string // userID -> customerID
	subscriptions map[string]gatewaySubscription
}

type gatewaySubscription struct {
	ID       string
	PlanTier domain.SubscriptionTier
	Customer string
}

// NewFakePaymentGateway constructs an in-memory gateway facade.
func NewFakePaymentGateway() *FakePaymentGateway {
	return &FakePaymentGateway{
		customers:     map[string]string{},
		subscriptions: map[string]gatewaySubscription{},
	}
}

// EnsureCustomer returns a stable customer ID per user.
func (g *FakePaymentGateway) EnsureCustomer(ctx context.Context, user domain.User) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if id, ok := g.customers[user.ID]; ok {
		return id, nil
	}
	id := "cust_" + uuid.NewString()
	g.customers[user.ID] = id
	return id, nil
}

// CreateSubscription pretends to provision a paid subscription remotely.
func (g *FakePaymentGateway) CreateSubscription(ctx context.Context, customerID string, plan domain.SubscriptionPlan) (string, error) {
	if customerID == "" {
		return "", fmt.Errorf("customer id required")
	}
	if plan.MonthlyPriceCents <= 0 {
		return "", fmt.Errorf("paid plan required for remote subscription")
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	subID := "sub_" + uuid.NewString()
	g.subscriptions[subID] = gatewaySubscription{ID: subID, PlanTier: plan.Tier, Customer: customerID}
	return subID, nil
}

// CancelSubscription removes the fake subscription entry.
func (g *FakePaymentGateway) CancelSubscription(ctx context.Context, gatewaySubscriptionID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.subscriptions, gatewaySubscriptionID)
	return nil
}
