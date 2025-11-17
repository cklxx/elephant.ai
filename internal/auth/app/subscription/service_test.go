package subscription

import (
	"context"
	"errors"
	"testing"
	"time"

	"alex/internal/auth/domain"
)

func TestChangePlanCreatesPaidSubscription(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	user := domain.User{ID: "user-1", SubscriptionTier: domain.SubscriptionTierFree}
	plan := domain.SubscriptionPlan{Tier: domain.SubscriptionTierSupporter, MonthlyPriceCents: 2000, IsActive: true}

	svc := NewService(&mockUserRepo{}, &mockPlanRepo{plan: plan}, &mockSubscriptionRepo{}, &mockPaymentGateway{}, Config{BillingCycle: 30 * 24 * time.Hour})
	svc.WithNow(func() time.Time { return now })
	publisher := &recordingPublisher{}
	svc.AttachPublisher(publisher)

	event, err := svc.ChangePlan(context.Background(), user, domain.SubscriptionTierSupporter, true, "corr-123")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if event.Type != domain.SubscriptionEventCreated {
		t.Fatalf("expected created event, got %s", event.Type)
	}
	if event.Subscription.Tier != domain.SubscriptionTierSupporter {
		t.Fatalf("expected tier updated, got %s", event.Subscription.Tier)
	}
	if got := len(publisher.subscriptionEvents); got != 1 {
		t.Fatalf("expected 1 published event, got %d", got)
	}
}

func TestChangePlanUpdatesExistingSubscription(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	plan := domain.SubscriptionPlan{Tier: domain.SubscriptionTierProfessional, MonthlyPriceCents: 10000, IsActive: true}
	current := domain.Subscription{ID: "sub-1", UserID: "user-1", Tier: domain.SubscriptionTierSupporter, Status: domain.SubscriptionStatusActive, CreatedAt: now.Add(-time.Hour)}
	subs := &mockSubscriptionRepo{current: &current}
	user := domain.User{ID: "user-1", SubscriptionTier: domain.SubscriptionTierSupporter}

	svc := NewService(&mockUserRepo{}, &mockPlanRepo{plan: plan}, subs, &mockPaymentGateway{}, Config{})
	svc.WithNow(func() time.Time { return now })
	publisher := &recordingPublisher{}
	svc.AttachPublisher(publisher)

	event, err := svc.ChangePlan(context.Background(), user, domain.SubscriptionTierProfessional, true, "corr-456")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if event.Type != domain.SubscriptionEventUpdated {
		t.Fatalf("expected updated event, got %s", event.Type)
	}
	if subs.lastUpdate == nil {
		t.Fatalf("expected subscription to be updated")
	}
	if subs.lastUpdate.Tier != domain.SubscriptionTierProfessional {
		t.Fatalf("expected tier professional, got %s", subs.lastUpdate.Tier)
	}
	if got := len(publisher.subscriptionEvents); got != 1 {
		t.Fatalf("expected 1 published event, got %d", got)
	}
}

func TestCancelEmitsCanceledEvent(t *testing.T) {
	now := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	current := domain.Subscription{ID: "sub-1", UserID: "user-1", Tier: domain.SubscriptionTierSupporter, Status: domain.SubscriptionStatusActive, ExternalSubscriptionID: "ext-1"}
	subs := &mockSubscriptionRepo{current: &current}
	user := domain.User{ID: "user-1", SubscriptionTier: domain.SubscriptionTierSupporter}

	payments := &mockPaymentGateway{}
	svc := NewService(&mockUserRepo{}, &mockPlanRepo{}, subs, payments, Config{})
	svc.WithNow(func() time.Time { return now })
	publisher := &recordingPublisher{}
	svc.AttachPublisher(publisher)

	event, err := svc.Cancel(context.Background(), user, "corr-789")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if event.Type != domain.SubscriptionEventCanceled {
		t.Fatalf("expected canceled event, got %s", event.Type)
	}
	if !payments.canceled {
		t.Fatalf("expected payment cancellation to be invoked")
	}
	if got := len(publisher.subscriptionEvents); got != 1 {
		t.Fatalf("expected cancel event published, got %d", got)
	}
}

type recordingPublisher struct {
	subscriptionEvents []domain.SubscriptionEvent
}

func (r *recordingPublisher) PublishSubscriptionEvent(ctx context.Context, event domain.SubscriptionEvent) error {
	r.subscriptionEvents = append(r.subscriptionEvents, event)
	return nil
}

func (r *recordingPublisher) PublishPointsEvent(ctx context.Context, event domain.PointsLedgerEvent) error {
	return nil
}

type mockUserRepo struct{}

func (m *mockUserRepo) Create(ctx context.Context, user domain.User) (domain.User, error) {
	return user, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user domain.User) (domain.User, error) {
	return user, nil
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return domain.User{}, errors.New("not implemented")
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (domain.User, error) {
	return domain.User{}, errors.New("not implemented")
}

type mockPlanRepo struct {
	plan domain.SubscriptionPlan
}

func (m *mockPlanRepo) List(ctx context.Context, includeInactive bool) ([]domain.SubscriptionPlan, error) {
	return nil, nil
}

func (m *mockPlanRepo) FindByTier(ctx context.Context, tier domain.SubscriptionTier) (domain.SubscriptionPlan, error) {
	if m.plan.Tier == "" {
		return domain.SubscriptionPlan{}, domain.ErrInvalidSubscriptionTier
	}
	if tier != m.plan.Tier {
		return domain.SubscriptionPlan{}, domain.ErrInvalidSubscriptionTier
	}
	return m.plan, nil
}

func (m *mockPlanRepo) Upsert(ctx context.Context, plan domain.SubscriptionPlan) error {
	m.plan = plan
	return nil
}

type mockSubscriptionRepo struct {
	current    *domain.Subscription
	lastCreate *domain.Subscription
	lastUpdate *domain.Subscription
}

func (m *mockSubscriptionRepo) Create(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error) {
	m.lastCreate = &subscription
	m.current = &subscription
	return subscription, nil
}

func (m *mockSubscriptionRepo) Update(ctx context.Context, subscription domain.Subscription) (domain.Subscription, error) {
	m.lastUpdate = &subscription
	m.current = &subscription
	return subscription, nil
}

func (m *mockSubscriptionRepo) FindActiveByUser(ctx context.Context, userID string) (domain.Subscription, error) {
	if m.current == nil {
		return domain.Subscription{}, domain.ErrSubscriptionNotFound
	}
	return *m.current, nil
}

func (m *mockSubscriptionRepo) ListExpiring(ctx context.Context, before time.Time) ([]domain.Subscription, error) {
	return nil, nil
}

type mockPaymentGateway struct {
	canceled bool
}

func (m *mockPaymentGateway) EnsureCustomer(ctx context.Context, user domain.User) (string, error) {
	return "cust-1", nil
}

func (m *mockPaymentGateway) CreateSubscription(ctx context.Context, customerID string, plan domain.SubscriptionPlan) (string, error) {
	return "sub-1", nil
}

func (m *mockPaymentGateway) CancelSubscription(ctx context.Context, gatewaySubscriptionID string) error {
	m.canceled = true
	return nil
}
