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
	NodeID    string
	// RelatedCount is the number of linked memory entries connected to this hit.
	RelatedCount int
}

// RelatedHit is a graph-adjacent memory result for a given memory node/span.
type RelatedHit struct {
	Path         string
	StartLine    int
	EndLine      int
	Score        float64
	Snippet      string
	RelationType string
	NodeID       string
}

// DailySnapshot holds one day's memory entry for inspection.
type DailySnapshot struct {
	Date    string // "2024-01-15" or filename without extension
	Path    string // relative path, e.g. "memory/2024-01-15.md"
	Content string
}

// Engine provides Markdown-based memory operations.
type Engine interface {
	EnsureSchema(ctx context.Context) error
	AppendDaily(ctx context.Context, userID string, entry DailyEntry) (string, error)
	Search(ctx context.Context, userID, query string, maxResults int, minScore float64) ([]SearchHit, error)
	Related(ctx context.Context, userID, path string, fromLine, toLine, maxResults int) ([]RelatedHit, error)
	GetLines(ctx context.Context, userID, path string, fromLine, lineCount int) (string, error)
	LoadDaily(ctx context.Context, userID string, day time.Time) (string, error)
	LoadLongTerm(ctx context.Context, userID string) (string, error)

	// LoadIdentity returns the soul and user identity markdown content.
	// Creates default files from defaultSoul/defaultUser if they don't exist.
	LoadIdentity(ctx context.Context, userID, defaultSoul, defaultUser string) (soul, user string, err error)

	// ListDailyEntries returns all daily memory entries sorted newest-first.
	ListDailyEntries(ctx context.Context, userID string) ([]DailySnapshot, error)
}
