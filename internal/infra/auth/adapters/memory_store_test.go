package adapters

import (
	"context"
	"testing"
	"time"

	"alex/internal/domain/auth"
)

func TestMemoryStateStorePurgeExpired(t *testing.T) {
	_, _, _, states := NewMemoryStores()

	now := time.Now()
	expired := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	if err := states.Save(context.Background(), "expired", domain.ProviderGoogle, expired); err != nil {
		t.Fatalf("save expired state: %v", err)
	}
	if err := states.Save(context.Background(), "future", domain.ProviderGoogle, future); err != nil {
		t.Fatalf("save future state: %v", err)
	}

	removed, err := states.PurgeExpired(context.Background(), now)
	if err != nil {
		t.Fatalf("purge expired: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 state removed, got %d", removed)
	}

	if err := states.Consume(context.Background(), "expired", domain.ProviderGoogle); err == nil {
		t.Fatalf("expected expired state to be removed")
	}
	if err := states.Consume(context.Background(), "future", domain.ProviderGoogle); err != nil {
		t.Fatalf("expected future state to be consumable, got %v", err)
	}
}
