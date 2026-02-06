package oauth

import (
	"context"
	"testing"
	"time"
)

func TestFileTokenStore_Persist(t *testing.T) {
	dir := t.TempDir()
	store, err := NewFileTokenStore(dir)
	if err != nil {
		t.Fatalf("NewFileTokenStore: %v", err)
	}

	token := Token{
		OpenID:       "ou_123",
		AccessToken:  "u-token",
		RefreshToken: "r-token",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		UpdatedAt:    time.Now(),
	}
	if err := store.Upsert(context.Background(), token); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	reloaded, err := NewFileTokenStore(dir)
	if err != nil {
		t.Fatalf("reload store: %v", err)
	}
	got, err := reloaded.Get(context.Background(), "ou_123")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.AccessToken != "u-token" {
		t.Fatalf("access_token=%q, want %q", got.AccessToken, "u-token")
	}

	if err := reloaded.Delete(context.Background(), "ou_123"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := reloaded.Get(context.Background(), "ou_123"); err == nil {
		t.Fatal("expected ErrTokenNotFound after delete")
	}
}
