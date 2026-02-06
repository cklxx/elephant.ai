package oauth

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStateStore_Consume(t *testing.T) {
	store := NewMemoryStateStore()
	ctx := context.Background()
	if err := store.Save(ctx, "state-1", time.Now().Add(1*time.Minute)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Consume(ctx, "state-1"); err != nil {
		t.Fatalf("Consume: %v", err)
	}
	if err := store.Consume(ctx, "state-1"); err == nil {
		t.Fatal("expected ErrStateNotFound on second consume")
	}
}

func TestMemoryStateStore_Expired(t *testing.T) {
	store := NewMemoryStateStore()
	ctx := context.Background()
	if err := store.Save(ctx, "state-2", time.Now().Add(-1*time.Minute)); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Consume(ctx, "state-2"); err == nil {
		t.Fatal("expected ErrStateNotFound for expired state")
	}
}
