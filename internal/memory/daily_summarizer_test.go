package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailySummarizerGeneratesFile(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	day := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	_, err := store.Insert(context.Background(), Entry{
		UserID:    "u",
		Content:   "Decision: use YAML configs",
		Keywords:  []string{"yaml"},
		Slots:     map[string]string{"type": "manual"},
		CreatedAt: day.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	_, err = store.Insert(context.Background(), Entry{
		UserID:    "u",
		Content:   "Captured workflow trace",
		Keywords:  []string{"workflow"},
		Slots:     map[string]string{"type": "auto_capture"},
		CreatedAt: day.Add(4 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}
	_, err = store.Insert(context.Background(), Entry{
		UserID:    "u",
		Content:   "Old memory",
		Keywords:  []string{"old"},
		Slots:     map[string]string{"type": "manual"},
		CreatedAt: day.Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Insert: %v", err)
	}

	summarizer := NewDailySummarizer(store, WithDailySummaryLocation(time.UTC))
	result, err := summarizer.Generate(context.Background(), "u", day)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !result.Written {
		t.Fatalf("expected summary to be written")
	}
	if result.EntryCount != 2 {
		t.Fatalf("expected 2 entries summarized, got %d", result.EntryCount)
	}
	if !strings.Contains(strings.Join(result.Sources, ","), "manual") {
		t.Fatalf("expected sources to include manual, got %v", result.Sources)
	}
	if !strings.Contains(strings.Join(result.Sources, ","), "auto_capture") {
		t.Fatalf("expected sources to include auto_capture, got %v", result.Sources)
	}

	if result.Path == "" {
		t.Fatalf("expected summary path")
	}
	if _, err := os.Stat(result.Path); err != nil {
		t.Fatalf("expected daily summary file at %s: %v", result.Path, err)
	}
	content, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read summary: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "date: 2026-02-01") && !strings.Contains(text, "date: \"2026-02-01\"") {
		t.Fatalf("expected frontmatter date, got %q", text)
	}
	if !strings.Contains(text, "entry_count: 2") {
		t.Fatalf("expected entry_count in frontmatter, got %q", text)
	}
	if !strings.Contains(text, "Decision: use YAML configs") {
		t.Fatalf("expected summary to mention entry, got %q", text)
	}

	if result.Path != filepath.Join(dir, "daily", "2026-02-01.md") {
		t.Fatalf("unexpected summary path: %s", result.Path)
	}
}
