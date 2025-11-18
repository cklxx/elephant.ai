package journal

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Reader exposes methods for consuming structured turn journal entries.
type Reader interface {
	// Stream walks every entry for the supplied session, invoking fn per entry in order.
	Stream(ctx context.Context, sessionID string, fn func(TurnJournalEntry) error) error
	// ReadAll loads every entry for the supplied session.
	ReadAll(ctx context.Context, sessionID string) ([]TurnJournalEntry, error)
}

// FileReader loads JSONL journal entries written by FileWriter.
type FileReader struct {
	dir string
}

// NewFileReader instantiates a reader rooted at dir.
func NewFileReader(dir string) (*FileReader, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("journal directory required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("ensure journal dir: %w", err)
	}
	return &FileReader{dir: dir}, nil
}

// Stream walks the session journal and invokes fn for each entry.
func (r *FileReader) Stream(ctx context.Context, sessionID string, fn func(TurnJournalEntry) error) error {
	if r == nil {
		return fmt.Errorf("nil reader")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}
	if fn == nil {
		return fmt.Errorf("callback required")
	}
	path := filepath.Join(r.dir, fmt.Sprintf("%s.jsonl", sessionID))
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open journal file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			// Surface close failures via stderr for observability without
			// clobbering the first scan/streaming error.
			fmt.Fprintf(os.Stderr, "journal: close %s: %v\n", path, cerr)
		}
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		var entry TurnJournalEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			return fmt.Errorf("decode journal entry: %w", err)
		}
		if entry.SessionID == "" {
			entry.SessionID = sessionID
		}
		if err := fn(entry); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan journal file: %w", err)
	}
	return nil
}

// ReadAll materializes every entry for the given session.
func (r *FileReader) ReadAll(ctx context.Context, sessionID string) ([]TurnJournalEntry, error) {
	entries := []TurnJournalEntry{}
	if err := r.Stream(ctx, sessionID, func(entry TurnJournalEntry) error {
		entries = append(entries, entry)
		return nil
	}); err != nil {
		return nil, err
	}
	return entries, nil
}
