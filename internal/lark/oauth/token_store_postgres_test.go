package oauth

import (
	"context"
	"testing"
	"time"

	"alex/internal/testutil"
)

func TestPostgresTokenStore_CRUD(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	store := NewPostgresTokenStore(pool)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	token := Token{
		OpenID:       "ou_123",
		AccessToken:  "u-token",
		RefreshToken: "r-token",
		ExpiresAt:    time.Now().Add(2 * time.Hour),
		UpdatedAt:    time.Now(),
	}
	if err := store.Upsert(context.Background(), token); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := store.Get(context.Background(), "ou_123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AccessToken != "u-token" {
		t.Fatalf("access_token=%q, want %q", got.AccessToken, "u-token")
	}

	if err := store.Delete(context.Background(), "ou_123"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := store.Get(context.Background(), "ou_123"); err == nil {
		t.Fatal("expected ErrTokenNotFound after delete")
	}
}
