package memory

import (
	"context"
	"time"
)

// DailyEntry represents a single appended record in a daily memory log.
type DailyEntry struct {
	Title     string
	Content   string
	CreatedAt time.Time
}

// SearchHit is a scored match returned from memory search.
type SearchHit struct {
	Path      string
	StartLine int
	EndLine   int
	Score     float64
	Snippet   string
	Source    string
}

// Engine provides Markdown-based memory operations.
type Engine interface {
	EnsureSchema(ctx context.Context) error
	AppendDaily(ctx context.Context, userID string, entry DailyEntry) (string, error)
	Search(ctx context.Context, userID, query string, maxResults int, minScore float64) ([]SearchHit, error)
	GetLines(ctx context.Context, userID, path string, fromLine, lineCount int) (string, error)
	LoadDaily(ctx context.Context, userID string, day time.Time) (string, error)
	LoadLongTerm(ctx context.Context, userID string) (string, error)
	RootDir() string
}
