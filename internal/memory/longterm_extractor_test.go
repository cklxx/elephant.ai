package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLongTermExtractorAddsFacts(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	if err := store.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	writeDailySummaryFile(t, dir, "2026-02-01", "- Use YAML only")
	writeDailySummaryFile(t, dir, "2026-02-02", "- Use YAML only")
	writeDailySummaryFile(t, dir, "2026-02-03", "- Use YAML only")

	now := time.Date(2026, 2, 4, 10, 0, 0, 0, time.UTC)
	extractor := NewLongTermExtractor(store,
		WithLongTermMinOccurrences(3),
		WithLongTermNow(func() time.Time { return now }),
	)

	result, err := extractor.Extract(context.Background())
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if !result.Written {
		t.Fatalf("expected extraction to write MEMORY.md")
	}

	content, err := os.ReadFile(filepath.Join(dir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "Updated: 2026-02-04 10:00") {
		t.Fatalf("expected Updated timestamp, got %q", text)
	}
	if countOccurrences(text, "Use YAML only") != 1 {
		t.Fatalf("expected fact once, got %q", text)
	}

	result, err = extractor.Extract(context.Background())
	if err != nil {
		t.Fatalf("Extract again: %v", err)
	}
	if result.Written {
		t.Fatalf("expected no write on idempotent extraction")
	}
	content, err = os.ReadFile(filepath.Join(dir, "MEMORY.md"))
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	text = string(content)
	if countOccurrences(text, "Use YAML only") != 1 {
		t.Fatalf("expected fact once after second extraction, got %q", text)
	}
}

func writeDailySummaryFile(t *testing.T, dir, date, bullet string) {
	t.Helper()
	path := filepath.Join(dir, "daily", date+".md")
	body := "---\n" +
		"date: " + date + "\n" +
		"entry_count: 1\n" +
		"sources:\n" +
		"  - manual\n" +
		"---\n\n" +
		"## Summary\n" + bullet + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write daily summary: %v", err)
	}
}

func countOccurrences(haystack, needle string) int {
	count := 0
	idx := 0
	for {
		offset := strings.Index(haystack[idx:], needle)
		if offset < 0 {
			return count
		}
		count++
		idx += offset + len(needle)
	}
}
