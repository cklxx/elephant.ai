package memory

import (
	"context"
	"fmt"
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
	fpath := filepath.Join(dir, "entries", saved.Key+".md")
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

func TestFileStoreSearchWithoutTermsReturnsAll(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	_ = store.EnsureSchema(context.Background())

	saved, err := store.Insert(context.Background(), Entry{
		UserID:   "u",
		Content:  "remember this for later",
		Keywords: []string{"note"},
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// Search with no terms/keywords should return all entries for the user.
	results, err := store.Search(context.Background(), Query{
		UserID: "u",
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result without terms, got %d", len(results))
	}
	if results[0].Content != saved.Content {
		t.Fatalf("content mismatch: got %q, want %q", results[0].Content, saved.Content)
	}
}

func TestFileStoreSearchWithoutTermsRespectsLimit(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	_ = store.EnsureSchema(context.Background())

	for i := 0; i < 5; i++ {
		if _, err := store.Insert(context.Background(), Entry{
			UserID:    "u",
			Content:   fmt.Sprintf("entry %d", i),
			CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	results, err := store.Search(context.Background(), Query{
		UserID: "u",
		Limit:  3,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results (limit), got %d", len(results))
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

func TestFileStoreSearchLayerPriority(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	_ = store.EnsureSchema(context.Background())
	// Ensure daily lookback includes the test date regardless of current time.
	store.dailyLookback = 3650

	if _, err := store.Insert(context.Background(), Entry{
		UserID:    "u",
		Content:   "Raw entry about policy controls",
		Keywords:  []string{"policy"},
		Slots:     map[string]string{"type": "manual"},
		CreatedAt: time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	dailyPath := filepath.Join(dir, "daily", "2026-02-01.md")
	dailyBody := `---
date: 2026-02-01
entry_count: 1
sources:
  - manual
---

## Summary
- Daily summary mentions policy controls`
	if err := os.WriteFile(dailyPath, []byte(dailyBody), 0o644); err != nil {
		t.Fatalf("write daily summary: %v", err)
	}

	memoryPath := filepath.Join(dir, "MEMORY.md")
	memoryBody := `# Long-Term Memory

Updated: 2026-02-01 10:00

## Extracted Facts
- Policy controls prefer YAML`
	if err := os.WriteFile(memoryPath, []byte(memoryBody), 0o644); err != nil {
		t.Fatalf("write memory file: %v", err)
	}

	results, err := store.Search(context.Background(), Query{
		UserID:   "u",
		Keywords: []string{"policy"},
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if got := results[0].Slots["type"]; got != "long_term" {
		t.Fatalf("expected long_term first, got %q", got)
	}
	if got := results[1].Slots["type"]; got != "daily_summary" {
		t.Fatalf("expected daily_summary second, got %q", got)
	}
	if results[2].Slots["type"] != "manual" {
		t.Fatalf("expected raw entry third, got %+v", results[2].Slots)
	}
}

func TestFileStoreMigrationMovesLegacyFiles(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, "legacy.md")
	legacyBody := `---
user: u
created: 2026-02-01T10:00:00Z
---
Legacy memory content`
	if err := os.WriteFile(legacyPath, []byte(legacyBody), 0o644); err != nil {
		t.Fatalf("write legacy file: %v", err)
	}

	store := NewFileStore(dir)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy file moved, stat err=%v", err)
	}

	migratedPath := filepath.Join(dir, "entries", "legacy.md")
	if _, err := os.Stat(migratedPath); err != nil {
		t.Fatalf("expected migrated file at %s: %v", migratedPath, err)
	}
}
