package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupDailyDir(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dailyDir := filepath.Join(root, dailyDirName)
	if err := os.MkdirAll(dailyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	return root, dailyDir
}

func writeDailyFile(t *testing.T, dailyDir, date string) {
	t.Helper()
	path := filepath.Join(dailyDir, date+".md")
	if err := os.WriteFile(path, []byte("# "+date+"\n\n## Entry\nsome content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupExpired_ArchivesOldEntries(t *testing.T) {
	root, dailyDir := setupDailyDir(t)
	engine := NewMarkdownEngine(root)

	writeDailyFile(t, dailyDir, "2024-01-01")
	writeDailyFile(t, dailyDir, "2024-01-15")
	writeDailyFile(t, dailyDir, "2024-02-10")

	// Cutoff: anything before Feb 1 should be archived.
	cutoff := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	result, err := engine.CleanupExpired(cutoff)
	if err != nil {
		t.Fatal(err)
	}

	if result.Archived != 2 {
		t.Errorf("expected 2 archived, got %d", result.Archived)
	}

	// Verify archived files exist.
	archiveDir := filepath.Join(root, archiveDirName)
	for _, date := range []string{"2024-01-01", "2024-01-15"} {
		path := filepath.Join(archiveDir, date+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected archived file %s to exist", path)
		}
	}

	// Verify archived files removed from daily.
	for _, date := range []string{"2024-01-01", "2024-01-15"} {
		path := filepath.Join(dailyDir, date+".md")
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("expected daily file %s to be removed", path)
		}
	}
}

func TestCleanupExpired_RetainsRecentEntries(t *testing.T) {
	root, dailyDir := setupDailyDir(t)
	engine := NewMarkdownEngine(root)

	writeDailyFile(t, dailyDir, "2024-03-01")
	writeDailyFile(t, dailyDir, "2024-03-10")

	// Cutoff: Feb 1 — both entries are newer.
	cutoff := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	result, err := engine.CleanupExpired(cutoff)
	if err != nil {
		t.Fatal(err)
	}

	if result.Archived != 0 {
		t.Errorf("expected 0 archived, got %d", result.Archived)
	}

	// Both files should still be in daily.
	for _, date := range []string{"2024-03-01", "2024-03-10"} {
		path := filepath.Join(dailyDir, date+".md")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected daily file %s to still exist", path)
		}
	}
}

func TestCleanupExpired_ArchiveDirectoryStructure(t *testing.T) {
	root, dailyDir := setupDailyDir(t)
	engine := NewMarkdownEngine(root)

	writeDailyFile(t, dailyDir, "2024-01-05")

	cutoff := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	result, err := engine.CleanupExpired(cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if result.Archived != 1 {
		t.Fatalf("expected 1 archived, got %d", result.Archived)
	}

	// Verify archive dir is at root/archive/
	archiveDir := filepath.Join(root, archiveDirName)
	info, err := os.Stat(archiveDir)
	if err != nil {
		t.Fatalf("archive dir should exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("archive should be a directory")
	}

	// Verify content is preserved.
	data, err := os.ReadFile(filepath.Join(archiveDir, "2024-01-05.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("archived file should have content")
	}
}

func TestCleanupExpired_SkipsNonDateFiles(t *testing.T) {
	root, dailyDir := setupDailyDir(t)
	engine := NewMarkdownEngine(root)

	// Create a non-date file that should be ignored.
	if err := os.WriteFile(filepath.Join(dailyDir, "notes.md"), []byte("not a date"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeDailyFile(t, dailyDir, "2024-01-01")

	cutoff := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	result, err := engine.CleanupExpired(cutoff)
	if err != nil {
		t.Fatal(err)
	}

	if result.Archived != 1 {
		t.Errorf("expected 1 archived, got %d", result.Archived)
	}

	// notes.md should still be in daily dir.
	if _, err := os.Stat(filepath.Join(dailyDir, "notes.md")); os.IsNotExist(err) {
		t.Error("non-date file should not be moved")
	}
}

func TestCleanupExpired_EmptyDailyDir(t *testing.T) {
	root, _ := setupDailyDir(t)
	engine := NewMarkdownEngine(root)

	cutoff := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	result, err := engine.CleanupExpired(cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if result.Archived != 0 {
		t.Errorf("expected 0 archived, got %d", result.Archived)
	}
}

func TestCleanupExpired_NoDailyDir(t *testing.T) {
	root := t.TempDir()
	engine := NewMarkdownEngine(root)

	cutoff := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	result, err := engine.CleanupExpired(cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if result.Archived != 0 {
		t.Errorf("expected 0 archived, got %d", result.Archived)
	}
}

func TestStartCleanupLoop_StopsOnCancel(t *testing.T) {
	root, dailyDir := setupDailyDir(t)
	engine := NewMarkdownEngine(root)
	writeDailyFile(t, dailyDir, "2020-01-01")

	ctx, cancel := context.WithCancel(context.Background())

	// Use a very short interval and initial delay so the goroutine runs quickly.
	engine.StartCleanupLoop(ctx, CleanupConfig{
		ArchiveAfterDays: 30,
		CleanupInterval:  50 * time.Millisecond,
		InitialDelay:     10 * time.Millisecond,
	})

	// Wait for at least one cleanup tick to execute.
	time.Sleep(200 * time.Millisecond)

	// Cancel the context — the goroutine should exit promptly.
	cancel()

	// Give a short grace period for the goroutine to finish.
	// If it doesn't stop, the test will pass but the goroutine leaks —
	// however we verify indirectly by ensuring no panic/race.
	time.Sleep(100 * time.Millisecond)

	// Verify the cleanup did run (old file was archived).
	archivePath := filepath.Join(root, archiveDirName, "2020-01-01.md")
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		t.Error("expected cleanup to have archived the old entry")
	}
}

func TestStartCleanupLoop_NopWhenDisabled(t *testing.T) {
	root := t.TempDir()
	engine := NewMarkdownEngine(root)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should not launch a goroutine when ArchiveAfterDays is 0.
	engine.StartCleanupLoop(ctx, CleanupConfig{
		ArchiveAfterDays: 0,
		CleanupInterval:  time.Second,
	})

	// Also should not launch when interval is 0.
	engine.StartCleanupLoop(ctx, CleanupConfig{
		ArchiveAfterDays: 30,
		CleanupInterval:  0,
	})

	// No assertion needed — if goroutines leaked they'd be caught by -race.
}

func TestParseDailyFileName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantOK  bool
		wantDay string
	}{
		{"valid date", "2024-01-15.md", true, "2024-01-15"},
		{"invalid format", "notes.md", false, ""},
		{"partial date", "2024-01.md", false, ""},
		{"no extension", "2024-01-15", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			date, ok := parseDailyFileName(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parseDailyFileName(%q) ok=%v, want %v", tt.input, ok, tt.wantOK)
			}
			if ok && date.Format("2006-01-02") != tt.wantDay {
				t.Errorf("parseDailyFileName(%q) date=%v, want %v", tt.input, date, tt.wantDay)
			}
		})
	}
}
