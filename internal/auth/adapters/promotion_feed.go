package adapters

import (
	"context"
	"sync"

	"alex/internal/auth/domain"
)

// MemoryPromotionFeed exposes a simple per-user promotion list for dev/test.
type MemoryPromotionFeed struct {
	mu       sync.RWMutex
	promos   map[string][]domain.Promotion
	defaultP []domain.Promotion
}

// NewMemoryPromotionFeed constructs an empty promotion feed.
func NewMemoryPromotionFeed() *MemoryPromotionFeed {
	return &MemoryPromotionFeed{
		promos:   map[string][]domain.Promotion{},
		defaultP: []domain.Promotion{},
	}
}

// SetDefault configures the fallback promotions returned when a user has no overrides.
func (f *MemoryPromotionFeed) SetDefault(promotions []domain.Promotion) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.defaultP = clonePromotions(promotions)
}

// SetPromotions overrides the promotions for a single user.
func (f *MemoryPromotionFeed) SetPromotions(userID string, promotions []domain.Promotion) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if promotions == nil {
		delete(f.promos, userID)
		return
	}
	f.promos[userID] = clonePromotions(promotions)
}

// ListActive returns the configured promotions for a user.
func (f *MemoryPromotionFeed) ListActive(ctx context.Context, userID string) ([]domain.Promotion, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if promos, ok := f.promos[userID]; ok {
		return clonePromotions(promos), nil
	}
	return clonePromotions(f.defaultP), nil
}

func clonePromotions(src []domain.Promotion) []domain.Promotion {
	if len(src) == 0 {
		return []domain.Promotion{}
	}
	cloned := make([]domain.Promotion, len(src))
	copy(cloned, src)
	return cloned
}
