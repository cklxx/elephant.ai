package points

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"

	"alex/internal/auth/domain"
	"alex/internal/auth/ports"
)

// Config controls guardrails around the points service.
type Config struct {
	// MaxBalance caps the balance any user can hold.
	MaxBalance int64
}

// Service reconciles immutable ledger entries with the mutable user balance.
type Service struct {
	users      ports.UserRepository
	ledger     ports.PointsLedgerRepository
	promotions ports.PromotionFeedPort
	events     ports.EventPublisher
	config     Config
	now        func() time.Time
}

// NewService constructs the points service.
func NewService(users ports.UserRepository, ledger ports.PointsLedgerRepository, promotions ports.PromotionFeedPort, cfg Config) *Service {
	if cfg.MaxBalance == 0 {
		cfg.MaxBalance = math.MaxInt64
	}
	return &Service{
		users:      users,
		ledger:     ledger,
		promotions: promotions,
		config:     cfg,
		now:        time.Now,
	}
}

// WithNow injects a deterministic clock for tests.
func (s *Service) WithNow(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// AttachPublisher wires the optional event publisher so ledger adjustments propagate outward.
func (s *Service) AttachPublisher(publisher ports.EventPublisher) {
	s.events = publisher
}

// Adjust adds (delta > 0) or removes (delta < 0) points for a user and persists a ledger entry.
func (s *Service) Adjust(ctx context.Context, userID string, delta int64, reason string, metadata map[string]any, correlationID string) (domain.PointsLedgerEvent, error) {
	if delta == 0 {
		return domain.PointsLedgerEvent{}, fmt.Errorf("delta must be non-zero")
	}
	if reason == "" {
		return domain.PointsLedgerEvent{}, fmt.Errorf("reason is required")
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return domain.PointsLedgerEvent{}, err
	}
	now := s.now()
	totalDelta := delta
	meta := cloneMetadata(metadata)
	if delta > 0 {
		bonus, codes, err := s.applyPromotions(ctx, userID, now)
		if err != nil {
			return domain.PointsLedgerEvent{}, err
		}
		if bonus > 0 {
			if totalDelta > math.MaxInt64-bonus {
				return domain.PointsLedgerEvent{}, fmt.Errorf("points delta overflow")
			}
			totalDelta += bonus
			meta["promotion_bonus"] = bonus
			if len(codes) > 0 {
				meta["promotion_codes"] = codes
			}
		}
		if user.PointsBalance > s.config.MaxBalance-totalDelta {
			return domain.PointsLedgerEvent{}, fmt.Errorf("points balance would exceed maximum")
		}
	}
	balanceBefore := user.PointsBalance
	if totalDelta < 0 {
		if user.PointsBalance < -totalDelta {
			return domain.PointsLedgerEvent{}, domain.ErrInsufficientPoints
		}
	}
	newBalance := user.PointsBalance + totalDelta
	if newBalance < 0 {
		return domain.PointsLedgerEvent{}, domain.ErrInsufficientPoints
	}
	entry := domain.PointsLedgerEntry{
		ID:        uuid.NewString(),
		UserID:    user.ID,
		Delta:     totalDelta,
		Reason:    reason,
		Metadata:  meta,
		CreatedAt: now,
	}
	entry.BalanceAfter = &newBalance
	savedEntry, err := s.ledger.AppendEntry(ctx, entry)
	if err != nil {
		return domain.PointsLedgerEvent{}, err
	}
	user.PointsBalance = newBalance
	user.UpdatedAt = now
	if _, err := s.users.Update(ctx, user); err != nil {
		return domain.PointsLedgerEvent{}, err
	}
	eventType := domain.PointsEventCredited
	if totalDelta < 0 {
		eventType = domain.PointsEventDebited
	}
	evt := domain.PointsLedgerEvent{
		Type:          eventType,
		Entry:         savedEntry,
		BalanceBefore: &balanceBefore,
		OccurredAt:    now,
		CorrelationID: correlationID,
	}
	if err := s.publishEvent(ctx, evt); err != nil {
		return domain.PointsLedgerEvent{}, err
	}
	return evt, nil
}

func (s *Service) applyPromotions(ctx context.Context, userID string, now time.Time) (int64, []string, error) {
	if s.promotions == nil {
		return 0, nil, nil
	}
	promos, err := s.promotions.ListActive(ctx, userID)
	if err != nil {
		return 0, nil, err
	}
	var bonus int64
	var codes []string
	for _, promo := range promos {
		if promo.PointsBonus <= 0 {
			continue
		}
		if promo.IsExpired(now) {
			continue
		}
		if bonus > math.MaxInt64-promo.PointsBonus {
			return 0, nil, fmt.Errorf("promotion bonus overflow")
		}
		bonus += promo.PointsBonus
		codes = append(codes, promo.Code)
	}
	return bonus, codes, nil
}

func cloneMetadata(meta map[string]any) map[string]any {
	if meta == nil {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(meta))
	for key, value := range meta {
		cloned[key] = value
	}
	return cloned
}

func (s *Service) publishEvent(ctx context.Context, event domain.PointsLedgerEvent) error {
	if s.events == nil {
		return nil
	}
	return s.events.PublishPointsEvent(ctx, event)
}
