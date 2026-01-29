package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileStoreInsertAndSearch(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	entry := Entry{
		UserID:   "user-1",
		Content:  "Deployment requires kubectl apply -f staging.yaml",
		Keywords: []string{"deployment", "kubernetes"},
		Slots:    map[string]string{"source": "lark"},
	}
	saved, err := store.Insert(context.Background(), entry)
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if saved.Key == "" {
		t.Fatal("expected generated key")
	}

	// Verify the file exists on disk.
	fpath := filepath.Join(dir, saved.Key+".md")
	if _, err := os.Stat(fpath); err != nil {
		t.Fatalf("expected file at %s: %v", fpath, err)
	}

	results, err := store.Search(context.Background(), Query{
		UserID:   "user-1",
		Keywords: []string{"deployment"},
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != entry.Content {
		t.Fatalf("content mismatch: %q", results[0].Content)
	}
}

func TestFileStoreSearchFiltersUser(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	_ = store.EnsureSchema(context.Background())

	if _, err := store.Insert(context.Background(), Entry{
		UserID:   "user-a",
		Content:  "alpha note",
		Keywords: []string{"shared"},
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := store.Insert(context.Background(), Entry{
		UserID:   "user-b",
		Content:  "beta note",
		Keywords: []string{"shared"},
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := store.Search(context.Background(), Query{
		UserID:   "user-a",
		Keywords: []string{"shared"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for user-a, got %d", len(results))
	}
	if results[0].UserID != "user-a" {
		t.Fatalf("wrong user: %s", results[0].UserID)
	}
}

func TestFileStoreSearchSortsNewestFirst(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	_ = store.EnsureSchema(context.Background())

	older := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	if _, err := store.Insert(context.Background(), Entry{
		UserID:    "u",
		Content:   "old entry about deployment",
		Keywords:  []string{"deploy"},
		CreatedAt: older,
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := store.Insert(context.Background(), Entry{
		UserID:    "u",
		Content:   "new entry about deployment",
		Keywords:  []string{"deploy"},
		CreatedAt: newer,
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	results, err := store.Search(context.Background(), Query{
		UserID:   "u",
		Keywords: []string{"deploy"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].CreatedAt.Before(results[1].CreatedAt) {
		t.Fatal("expected newest first")
	}
}

func TestFileStoreSearchMatchesSlots(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	_ = store.EnsureSchema(context.Background())

	if _, err := store.Insert(context.Background(), Entry{
		UserID:   "u",
		Content:  "triage auth issue",
		Keywords: []string{"auth"},
		Slots:    map[string]string{"intent": "triage"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Insert(context.Background(), Entry{
		UserID:   "u",
		Content:  "write auth docs",
		Keywords: []string{"auth"},
		Slots:    map[string]string{"intent": "write"},
	}); err != nil {
		t.Fatal(err)
	}

	results, err := store.Search(context.Background(), Query{
		UserID:   "u",
		Keywords: []string{"auth"},
		Slots:    map[string]string{"intent": "triage"},
		Limit:    10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Slots["intent"] != "triage" {
		t.Fatalf("wrong slot: %v", results[0].Slots)
	}
}

func TestFileStoreEmptyDir(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	_ = store.EnsureSchema(context.Background())

	results, err := store.Search(context.Background(), Query{
		UserID:   "u",
		Keywords: []string{"anything"},
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("Search empty dir: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestFileStoreNonExistentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	store := NewFileStore(dir)

	// Search on non-existent dir should return empty, not error.
	results, err := store.Search(context.Background(), Query{
		UserID:   "u",
		Keywords: []string{"x"},
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("Search non-existent dir: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestFileStoreServiceIntegration(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	_ = store.EnsureSchema(context.Background())
	svc := NewService(store)

	_, err := svc.Save(context.Background(), Entry{
		UserID:   "u",
		Content:  "登录 bug 修复总结",
		Keywords: []string{"登录", "bug"},
	})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	results, err := svc.Recall(context.Background(), Query{
		UserID:   "u",
		Keywords: []string{"登录"},
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}
