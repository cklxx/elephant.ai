package adapters

import (
	"context"
	"sync"

	"alex/internal/auth/domain"
	"alex/internal/auth/ports"
)

// MemoryEventPublisher collects emitted events for inspection in tests or local dev.
type MemoryEventPublisher struct {
	mu                 sync.Mutex
	subscriptionEvents []domain.SubscriptionEvent
	pointsEvents       []domain.PointsLedgerEvent
}

// NewMemoryEventPublisher constructs an in-memory publisher that satisfies ports.EventPublisher.
func NewMemoryEventPublisher() *MemoryEventPublisher {
	return &MemoryEventPublisher{}
}

// PublishSubscriptionEvent appends the event to the in-memory slice.
func (m *MemoryEventPublisher) PublishSubscriptionEvent(_ context.Context, event domain.SubscriptionEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.subscriptionEvents = append(m.subscriptionEvents, event)
	return nil
}

// PublishPointsEvent appends the event to the in-memory slice.
func (m *MemoryEventPublisher) PublishPointsEvent(_ context.Context, event domain.PointsLedgerEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pointsEvents = append(m.pointsEvents, event)
	return nil
}

// SubscriptionEvents returns a copy of the collected subscription events.
func (m *MemoryEventPublisher) SubscriptionEvents() []domain.SubscriptionEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.SubscriptionEvent, len(m.subscriptionEvents))
	copy(out, m.subscriptionEvents)
	return out
}

// PointsEvents returns a copy of the collected points ledger events.
func (m *MemoryEventPublisher) PointsEvents() []domain.PointsLedgerEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.PointsLedgerEvent, len(m.pointsEvents))
	copy(out, m.pointsEvents)
	return out
}

// Ensure MemoryEventPublisher implements the EventPublisher interface.
var _ ports.EventPublisher = (*MemoryEventPublisher)(nil)
