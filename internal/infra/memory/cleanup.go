package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/shared/logging"
)

// CleanupConfig controls automatic expiration of daily memory entries.
type CleanupConfig struct {
	ArchiveAfterDays int           // move entries older than N days (0 disables)
	CleanupInterval  time.Duration // how often to scan
}

// CleanupResult reports what a single cleanup pass did.
type CleanupResult struct {
	Archived int
	Errors   int
}

// CleanupExpired scans the daily memory directory and moves entries older than
// the configured threshold into an archive/ subdirectory. The archive mirrors
// the daily directory structure: archive/YYYY-MM-DD.md.
func (e *MarkdownEngine) CleanupExpired(cutoff time.Time) (CleanupResult, error) {
	var result CleanupResult

	root, err := e.requireRoot()
	if err != nil {
		return result, err
	}
	dailyDir := filepath.Join(root, dailyDirName)
	archiveDir := filepath.Join(root, archiveDirName)

	entries, err := os.ReadDir(dailyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return result, nil
		}
		return result, fmt.Errorf("read daily dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		date, ok := parseDailyFileName(entry.Name())
		if !ok {
			continue
		}

		if !date.Before(cutoff) {
			continue
		}

		if err := archiveFile(dailyDir, archiveDir, entry.Name()); err != nil {
			result.Errors++
			continue
		}
		result.Archived++
	}

	return result, nil
}

// parseDailyFileName extracts the date from a "YYYY-MM-DD.md" filename.
func parseDailyFileName(name string) (time.Time, bool) {
	if !strings.HasSuffix(name, ".md") {
		return time.Time{}, false
	}
	dateStr := strings.TrimSuffix(name, ".md")
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// archiveFile moves a file from dailyDir to archiveDir, preserving the filename.
func archiveFile(dailyDir, archiveDir, name string) error {
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return fmt.Errorf("create archive dir: %w", err)
	}
	src := filepath.Join(dailyDir, name)
	dst := filepath.Join(archiveDir, name)
	return os.Rename(src, dst)
}

// StartCleanupLoop launches a background goroutine that periodically runs
// CleanupExpired. It stops when ctx is cancelled.
func (e *MarkdownEngine) StartCleanupLoop(ctx context.Context, cfg CleanupConfig) {
	if cfg.ArchiveAfterDays <= 0 || cfg.CleanupInterval <= 0 {
		return
	}

	logger := logging.NewComponentLogger("MemoryCleanup")

	go func() {
		// Run once at startup after a short delay.
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
		e.runCleanup(logger, cfg.ArchiveAfterDays)

		ticker := time.NewTicker(cfg.CleanupInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.runCleanup(logger, cfg.ArchiveAfterDays)
			}
		}
	}()
}

func (e *MarkdownEngine) runCleanup(logger logging.Logger, archiveAfterDays int) {
	cutoff := time.Now().AddDate(0, 0, -archiveAfterDays)
	result, err := e.CleanupExpired(cutoff)
	if err != nil {
		logger.Warn("memory cleanup failed: %v", err)
		return
	}
	if result.Archived > 0 || result.Errors > 0 {
		logger.Info("archived %d entries older than %d days (errors: %d)", result.Archived, archiveAfterDays, result.Errors)
	}
}
