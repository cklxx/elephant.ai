package markdown

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"alex/internal/shared/logging"
)

func newTestStore(t *testing.T) *VersionedStore {
	t.Helper()
	dir := t.TempDir()
	store := NewVersionedStore(StoreConfig{
		Dir:        dir,
		AutoCommit: true,
		Logger:     logging.Nop(),
	})
	if err := store.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return store
}

func TestVersionedStore_ReadWriteRoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	want := "# Hello\nworld\n"
	if err := store.Write(ctx, "test.md", want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := store.Read("test.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != want {
		t.Errorf("round-trip mismatch: got %q, want %q", got, want)
	}
}

func TestVersionedStore_ReadNonExistent(t *testing.T) {
	store := newTestStore(t)
	content, err := store.Read("missing.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if content != "" {
		t.Errorf("expected empty, got %q", content)
	}
}

func TestVersionedStore_SeedIdempotent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Seed(ctx, "seed.md", "first"); err != nil {
		t.Fatalf("first Seed: %v", err)
	}
	if err := store.Seed(ctx, "seed.md", "second"); err != nil {
		t.Fatalf("second Seed: %v", err)
	}
	got, err := store.Read("seed.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got != "first" {
		t.Errorf("Seed not idempotent: got %q, want %q", got, "first")
	}
}

func TestVersionedStore_CommitAll(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Write(ctx, "a.md", "v1"); err != nil {
		t.Fatalf("Write: %v", err)
	}
	committed, err := store.CommitAll(ctx, "snapshot 1")
	if err != nil {
		t.Fatalf("CommitAll: %v", err)
	}
	if !committed {
		t.Fatal("expected commit to be created")
	}

	// Second call with no changes should be no-op.
	committed, err = store.CommitAll(ctx, "snapshot 2")
	if err != nil {
		t.Fatalf("CommitAll no-op: %v", err)
	}
	if committed {
		t.Fatal("expected no commit when clean")
	}
}

func TestVersionedStore_History(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Write(ctx, "h.md", "v1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitAll(ctx, "commit 1"); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(ctx, "h.md", "v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CommitAll(ctx, "commit 2"); err != nil {
		t.Fatal(err)
	}

	entries, err := store.History(ctx, "h.md", 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(entries))
	}
	if entries[0].Subject != "commit 2" {
		t.Errorf("expected 'commit 2', got %q", entries[0].Subject)
	}
}

func TestVersionedStore_AutoCommitOnWrite(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// First write — creates a staged file.
	if err := store.Write(ctx, "auto.md", "v1"); err != nil {
		t.Fatal(err)
	}
	// Second write — autoCommit should commit "v1" before writing "v2".
	if err := store.Write(ctx, "auto.md", "v2"); err != nil {
		t.Fatal(err)
	}
	// Commit remaining staged change.
	if _, err := store.CommitAll(ctx, "final"); err != nil {
		t.Fatal(err)
	}

	entries, err := store.History(ctx, "auto.md", 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	// Should have at least 2 commits: auto snapshot + final.
	if len(entries) < 2 {
		t.Fatalf("expected >= 2 commits, got %d", len(entries))
	}
}

func TestVersionedStore_AtomicWrite(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.Write(ctx, "atomic.md", "content"); err != nil {
		t.Fatal(err)
	}
	// tmp file should be cleaned up.
	tmp := filepath.Join(store.dir, "atomic.md.tmp")
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("tmp file should not exist after write, err=%v", err)
	}
}

func TestVersionedStore_ConcurrentWrites(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = store.Write(ctx, "concurrent.md", "write from goroutine")
		}(i)
	}
	wg.Wait()

	// File should exist with some content.
	got, err := store.Read("concurrent.md")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty content after concurrent writes")
	}
}

func TestVersionedStore_InitIdempotent(t *testing.T) {
	dir := t.TempDir()
	store := NewVersionedStore(StoreConfig{Dir: dir, Logger: logging.Nop()})
	ctx := context.Background()

	if err := store.Init(ctx); err != nil {
		t.Fatalf("first Init: %v", err)
	}
	if err := store.Init(ctx); err != nil {
		t.Fatalf("second Init: %v", err)
	}
}
