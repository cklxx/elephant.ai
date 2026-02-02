package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMarkdownEngineAppendDailyAndLoad(t *testing.T) {
	dir := t.TempDir()
	eng := NewMarkdownEngine(dir)
	if err := eng.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	when := time.Date(2026, 2, 2, 10, 30, 0, 0, time.Local)
	path, err := eng.AppendDaily(context.Background(), "user-1", DailyEntry{
		Title:     "API Decision",
		Content:   "Chose REST over GraphQL.",
		CreatedAt: when,
	})
	if err != nil {
		t.Fatalf("AppendDaily: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("daily file missing: %v", err)
	}

	content, err := eng.LoadDaily(context.Background(), "user-1", when)
	if err != nil {
		t.Fatalf("LoadDaily: %v", err)
	}
	if !strings.Contains(content, "# 2026-02-02") {
		t.Fatalf("expected header, got: %s", content)
	}
	if !strings.Contains(content, "REST over GraphQL") {
		t.Fatalf("expected entry content, got: %s", content)
	}
}

func TestMarkdownEngineSearchAndGetLines(t *testing.T) {
	dir := t.TempDir()
	eng := NewMarkdownEngine(dir)
	if err := eng.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	when := time.Date(2026, 2, 2, 8, 0, 0, 0, time.Local)
	_, err := eng.AppendDaily(context.Background(), "user-1", DailyEntry{
		Title:     "Deploy",
		Content:   "Deployed v2.3.0 to production.",
		CreatedAt: when,
	})
	if err != nil {
		t.Fatalf("AppendDaily: %v", err)
	}

	// Add MEMORY.md
	memDir := filepath.Join(dir, "users", "user-1")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	memoryPath := filepath.Join(memDir, "MEMORY.md")
	if err := os.WriteFile(memoryPath, []byte("# Long-Term Memory\n\n- Prefer TypeScript\n"), 0o644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	hits, err := eng.Search(context.Background(), "user-1", "TypeScript", 5, 0.2)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected hits")
	}
	if hits[0].Path == "" || hits[0].StartLine <= 0 {
		t.Fatalf("expected path + line numbers, got %+v", hits[0])
	}

	text, err := eng.GetLines(context.Background(), "user-1", hits[0].Path, hits[0].StartLine, 3)
	if err != nil {
		t.Fatalf("GetLines: %v", err)
	}
	if !strings.Contains(text, "TypeScript") {
		t.Fatalf("expected snippet to contain TypeScript, got: %s", text)
	}
}
