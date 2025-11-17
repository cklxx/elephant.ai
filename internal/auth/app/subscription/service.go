package subscription

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"alex/internal/auth/domain"
	"alex/internal/auth/ports"
)

// Config controls subscription lifecycle behaviour.
type Config struct {
	BillingCycle time.Duration
}

// Service orchestrates plan selection, billing, and subscription state persistence.
type Service struct {
	users    ports.UserRepository
	plans    ports.PlanRepository
	subs     ports.SubscriptionRepository
	payments ports.PaymentGatewayPort
	events   ports.EventPublisher
	config   Config
	now      func() time.Time
}

// NewService constructs a Subscription Service.
func NewService(users ports.UserRepository, plans ports.PlanRepository, subs ports.SubscriptionRepository, payments ports.PaymentGatewayPort, cfg Config) *Service {
	if cfg.BillingCycle == 0 {
		cfg.BillingCycle = 30 * 24 * time.Hour
	}
	return &Service{
		users:    users,
		plans:    plans,
		subs:     subs,
		payments: payments,
		config:   cfg,
		now:      time.Now,
	}
}

// WithNow allows tests to inject a deterministic clock.
func (s *Service) WithNow(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// AttachPublisher wires the optional event publisher to fan out lifecycle events.
func (s *Service) AttachPublisher(publisher ports.EventPublisher) {
	s.events = publisher
}

// ChangePlan switches the user to the requested tier, returning the emitted subscription event.
func (s *Service) ChangePlan(ctx context.Context, user domain.User, target domain.SubscriptionTier, autoRenew bool, correlationID string) (domain.SubscriptionEvent, error) {
	if target == "" {
		return domain.SubscriptionEvent{}, domain.ErrInvalidSubscriptionTier
	}
	// Degrade to free is equivalent to cancel.
	plan, err := s.plans.FindByTier(ctx, target)
	if err != nil {
		return domain.SubscriptionEvent{}, err
	}
	if !plan.IsActive {
		return domain.SubscriptionEvent{}, domain.ErrInvalidSubscriptionTier
	}
	if plan.MonthlyPriceCents == 0 {
		return s.Cancel(ctx, user, correlationID)
	}

	now := s.now()
	current, err := s.subs.FindActiveByUser(ctx, user.ID)
	var hasCurrent bool
	if err != nil {
		if !errors.Is(err, domain.ErrSubscriptionNotFound) {
			return domain.SubscriptionEvent{}, err
		}
	} else {
		hasCurrent = true
	}

	subscription := domain.Subscription{
		ID:                 uuid.NewString(),
		UserID:             user.ID,
		Tier:               target,
		Status:             domain.SubscriptionStatusActive,
		AutoRenew:          autoRenew,
		CurrentPeriodStart: now,
		Metadata:           cloneMetadata(plan.Metadata),
		ExternalPlanID:     string(plan.Tier),
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	periodEnd := now.Add(s.config.BillingCycle)
	subscription.CurrentPeriodEnd = &periodEnd

	previousTier := user.SubscriptionTier
	if hasCurrent {
		subscription.ID = current.ID
		subscription.CreatedAt = current.CreatedAt
		subscription.ExternalCustomerID = current.ExternalCustomerID
		subscription.ExternalSubscriptionID = current.ExternalSubscriptionID
		previousTier = current.Tier
	}

	if plan.MonthlyPriceCents > 0 {
		if s.payments == nil {
			return domain.SubscriptionEvent{}, fmt.Errorf("payment gateway not configured for paid tier %s", target)
		}
		customerID := subscription.ExternalCustomerID
		if customerID == "" {
			customerID, err = s.payments.EnsureCustomer(ctx, user)
			if err != nil {
				return domain.SubscriptionEvent{}, err
			}
		}
		subscription.ExternalCustomerID = customerID
		gatewaySubID, err := s.payments.CreateSubscription(ctx, customerID, plan)
		if err != nil {
			return domain.SubscriptionEvent{}, err
		}
		subscription.ExternalSubscriptionID = gatewaySubID
	}

	var saved domain.Subscription
	if hasCurrent {
		saved, err = s.subs.Update(ctx, subscription)
	} else {
		saved, err = s.subs.Create(ctx, subscription)
	}
	if err != nil {
		return domain.SubscriptionEvent{}, err
	}

	user.SubscriptionTier = target
	user.SubscriptionExpiresAt = subscription.CurrentPeriodEnd
	user.UpdatedAt = now
	if _, err := s.users.Update(ctx, user); err != nil {
		return domain.SubscriptionEvent{}, err
	}

	evtType := domain.SubscriptionEventCreated
	if hasCurrent {
		evtType = domain.SubscriptionEventUpdated
	}
	evt := domain.SubscriptionEvent{
		Type:          evtType,
		Subscription:  saved,
		PreviousTier:  previousTier,
		OccurredAt:    now,
		CorrelationID: correlationID,
	}
	if err := s.publishEvent(ctx, evt); err != nil {
		return domain.SubscriptionEvent{}, err
	}
	return evt, nil
}

// Cancel stops auto-renewal and marks the current subscription as canceled.
func (s *Service) Cancel(ctx context.Context, user domain.User, correlationID string) (domain.SubscriptionEvent, error) {
	current, err := s.subs.FindActiveByUser(ctx, user.ID)
	if err != nil {
		return domain.SubscriptionEvent{}, err
	}
	now := s.now()
	if s.payments != nil && current.ExternalSubscriptionID != "" {
		if err := s.payments.CancelSubscription(ctx, current.ExternalSubscriptionID); err != nil {
			return domain.SubscriptionEvent{}, err
		}
	}

	current.Status = domain.SubscriptionStatusCanceled
	current.AutoRenew = false
	end := now
	current.CurrentPeriodEnd = &end
	current.UpdatedAt = now

	updated, err := s.subs.Update(ctx, current)
	if err != nil {
		return domain.SubscriptionEvent{}, err
	}

	user.SubscriptionTier = domain.SubscriptionTierFree
	user.SubscriptionExpiresAt = nil
	user.UpdatedAt = now
	if _, err := s.users.Update(ctx, user); err != nil {
		return domain.SubscriptionEvent{}, err
	}

	evt := domain.SubscriptionEvent{
		Type:          domain.SubscriptionEventCanceled,
		Subscription:  updated,
		PreviousTier:  current.Tier,
		OccurredAt:    now,
		CorrelationID: correlationID,
	}
	if err := s.publishEvent(ctx, evt); err != nil {
		return domain.SubscriptionEvent{}, err
	}
	return evt, nil
}

func (s *Service) publishEvent(ctx context.Context, event domain.SubscriptionEvent) error {
	if s.events == nil {
		return nil
	}
	return s.events.PublishSubscriptionEvent(ctx, event)
}

func cloneMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(meta))
	for k, v := range meta {
		cloned[k] = v
	}
	return cloned
}
