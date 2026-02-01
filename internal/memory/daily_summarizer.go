package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultDailySummaryMaxEntries = 200
	maxDailySummaryLineLen        = 160
)

// DailySummaryResult captures the output of a daily summary generation.
type DailySummaryResult struct {
	Date       string
	EntryCount int
	Sources    []string
	Path       string
	Content    string
	Written    bool
}

// DailySummarizer builds daily summary files from raw memory entries.
type DailySummarizer struct {
	store      *FileStore
	maxEntries int
	location   *time.Location
}

// DailySummarizerOption customizes daily summary behavior.
type DailySummarizerOption func(*DailySummarizer)

// NewDailySummarizer constructs a daily summarizer bound to a layered file store.
func NewDailySummarizer(store *FileStore, opts ...DailySummarizerOption) *DailySummarizer {
	s := &DailySummarizer{
		store:      store,
		maxEntries: defaultDailySummaryMaxEntries,
		location:   time.UTC,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	return s
}

// WithDailySummaryLocation sets the timezone used to bucket entries by day.
func WithDailySummaryLocation(loc *time.Location) DailySummarizerOption {
	return func(s *DailySummarizer) {
		if loc != nil {
			s.location = loc
		}
	}
}

// WithDailySummaryMaxEntries caps how many entries are considered per day.
func WithDailySummaryMaxEntries(max int) DailySummarizerOption {
	return func(s *DailySummarizer) {
		if max > 0 {
			s.maxEntries = max
		}
	}
}

// Generate creates a daily summary for the given date and user.
func (s *DailySummarizer) Generate(ctx context.Context, userID string, day time.Time) (DailySummaryResult, error) {
	if s == nil || s.store == nil {
		return DailySummaryResult{}, fmt.Errorf("daily summarizer not initialized")
	}
	if strings.TrimSpace(userID) == "" {
		return DailySummaryResult{}, fmt.Errorf("user_id is required")
	}

	loc := s.location
	if loc == nil {
		loc = time.UTC
	}
	if day.IsZero() {
		day = time.Now().In(loc)
	}
	start := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	end := start.Add(24 * time.Hour)
	dateStr := start.Format("2006-01-02")

	entries, err := s.store.readEntries()
	if err != nil {
		return DailySummaryResult{}, err
	}

	filtered := make([]Entry, 0, len(entries))
	sourceSet := map[string]struct{}{}
	for _, entry := range entries {
		if entry.UserID != userID {
			continue
		}
		if entry.CreatedAt.IsZero() {
			continue
		}
		if entry.CreatedAt.Before(start) || !entry.CreatedAt.Before(end) {
			continue
		}
		typ := entryType(entry)
		if typ == dailySummaryType || typ == longTermType {
			continue
		}
		filtered = append(filtered, entry)
		source := typ
		if source == "" {
			source = "manual"
		}
		sourceSet[source] = struct{}{}
		if s.maxEntries > 0 && len(filtered) >= s.maxEntries {
			break
		}
	}

	if len(filtered) == 0 {
		return DailySummaryResult{Date: dateStr, Written: false}, nil
	}

	sources := make([]string, 0, len(sourceSet))
	for source := range sourceSet {
		sources = append(sources, source)
	}
	sort.Strings(sources)

	lines := make([]string, 0, len(filtered))
	for _, entry := range filtered {
		line := summarizeContent(entry.Content)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}

	var body strings.Builder
	body.WriteString("## Summary\n")
	for _, line := range lines {
		body.WriteString("- ")
		body.WriteString(line)
		body.WriteByte('\n')
	}

	fm := dailyFrontmatter{
		Date:       dateStr,
		EntryCount: len(filtered),
		Sources:    sources,
	}
	fmBytes, err := yaml.Marshal(fm)
	if err != nil {
		return DailySummaryResult{}, fmt.Errorf("marshal daily frontmatter: %w", err)
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	buf.Write(fmBytes)
	buf.WriteString("---\n\n")
	buf.WriteString(body.String())

	path := filepath.Join(s.store.dailyDir, dateStr+".md")
	if err := os.MkdirAll(s.store.dailyDir, 0o755); err != nil {
		return DailySummaryResult{}, fmt.Errorf("ensure daily dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(buf.String()), 0o644); err != nil {
		return DailySummaryResult{}, fmt.Errorf("write daily summary: %w", err)
	}

	return DailySummaryResult{
		Date:       dateStr,
		EntryCount: len(filtered),
		Sources:    sources,
		Path:       path,
		Content:    body.String(),
		Written:    true,
	}, nil
}

func summarizeContent(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if len(trimmed) > maxDailySummaryLineLen {
			return trimmed[:maxDailySummaryLineLen] + "..."
		}
		return trimmed
	}
	return ""
}
