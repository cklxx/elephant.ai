package postgresstore

import (
	"context"
	"testing"

	"alex/internal/testutil"
)

func TestPostgresStore_CrossInstanceReadWrite(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	storeA := New(pool)
	storeB := New(pool)

	if err := storeA.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	session, err := storeA.Create(ctx)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	loaded, err := storeB.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("get session from other instance: %v", err)
	}

	loaded.Metadata = map[string]string{"owner": "instance-b"}

	if err := storeB.Save(ctx, loaded); err != nil {
		t.Fatalf("save session from other instance: %v", err)
	}

	updated, err := storeA.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("get updated session: %v", err)
	}
	if updated.Metadata["owner"] != "instance-b" {
		t.Fatalf("expected metadata propagated, got %q", updated.Metadata["owner"])
	}
}

func TestPostgresStore_LastWriteWins(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	storeA := New(pool)
	storeB := New(pool)

	if err := storeA.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	session, err := storeA.Create(ctx)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	first, err := storeA.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	first.Metadata = map[string]string{"version": "a"}
	if err := storeA.Save(ctx, first); err != nil {
		t.Fatalf("save session: %v", err)
	}

	second, err := storeB.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("get session from other instance: %v", err)
	}
	second.Metadata = map[string]string{"version": "b"}
	if err := storeB.Save(ctx, second); err != nil {
		t.Fatalf("save session from other instance: %v", err)
	}

	updated, err := storeA.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("get updated session: %v", err)
	}
	if updated.Metadata["version"] != "b" {
		t.Fatalf("expected last write to win, got %q", updated.Metadata["version"])
	}
}
