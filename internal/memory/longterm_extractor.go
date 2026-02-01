package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	defaultLongTermLookbackDays   = 30
	defaultLongTermMinOccurrences = 3
)

// LongTermExtractionResult captures the outcome of long-term extraction.
type LongTermExtractionResult struct {
	Added   int
	Facts   []string
	Written bool
}

// LongTermExtractor promotes repeated daily facts into MEMORY.md.
type LongTermExtractor struct {
	store          *FileStore
	lookbackDays   int
	minOccurrences int
	now            func() time.Time
}

// LongTermExtractorOption customizes extraction behavior.
type LongTermExtractorOption func(*LongTermExtractor)

// NewLongTermExtractor constructs a long-term extractor bound to a layered file store.
func NewLongTermExtractor(store *FileStore, opts ...LongTermExtractorOption) *LongTermExtractor {
	e := &LongTermExtractor{
		store:          store,
		lookbackDays:   defaultLongTermLookbackDays,
		minOccurrences: defaultLongTermMinOccurrences,
		now:            time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(e)
		}
	}
	return e
}

// WithLongTermLookbackDays sets the number of days to scan for facts.
func WithLongTermLookbackDays(days int) LongTermExtractorOption {
	return func(e *LongTermExtractor) {
		if days > 0 {
			e.lookbackDays = days
		}
	}
}

// WithLongTermMinOccurrences sets the minimum repetition threshold for facts.
func WithLongTermMinOccurrences(count int) LongTermExtractorOption {
	return func(e *LongTermExtractor) {
		if count > 0 {
			e.minOccurrences = count
		}
	}
}

// WithLongTermNow overrides the time source for deterministic tests.
func WithLongTermNow(now func() time.Time) LongTermExtractorOption {
	return func(e *LongTermExtractor) {
		if now != nil {
			e.now = now
		}
	}
}

// Extract scans recent daily summaries and updates MEMORY.md with durable facts.
func (e *LongTermExtractor) Extract(ctx context.Context) (LongTermExtractionResult, error) {
	if e == nil || e.store == nil {
		return LongTermExtractionResult{}, fmt.Errorf("long-term extractor not initialized")
	}

	now := e.now()
	summaries, err := e.readRecentSummaries(now)
	if err != nil {
		return LongTermExtractionResult{}, err
	}
	if len(summaries) == 0 {
		return LongTermExtractionResult{Written: false}, nil
	}

	candidates := collectRepeatedFacts(summaries, e.minOccurrences)
	if len(candidates) == 0 {
		return LongTermExtractionResult{Written: false}, nil
	}

	existing, err := os.ReadFile(e.store.memoryFile)
	if err != nil && !os.IsNotExist(err) {
		return LongTermExtractionResult{}, fmt.Errorf("read memory file: %w", err)
	}
	newContent, added, changed := upsertLongTermContent(string(existing), candidates, now)
	if !changed {
		return LongTermExtractionResult{Written: false, Facts: candidates}, nil
	}
	if err := os.WriteFile(e.store.memoryFile, []byte(newContent), 0o644); err != nil {
		return LongTermExtractionResult{}, fmt.Errorf("write memory file: %w", err)
	}
	return LongTermExtractionResult{Added: added, Facts: candidates, Written: true}, nil
}

func (e *LongTermExtractor) readRecentSummaries(now time.Time) ([]dailySummary, error) {
	dirEntries, err := os.ReadDir(e.store.dailyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read daily dir: %w", err)
	}

	cutoff := time.Time{}
	if e.lookbackDays > 0 {
		utcNow := now.UTC()
		start := time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day(), 0, 0, 0, 0, time.UTC)
		cutoff = start.AddDate(0, 0, -(e.lookbackDays - 1))
	}

	var summaries []dailySummary
	for _, de := range dirEntries {
		if de.IsDir() || !strings.HasSuffix(de.Name(), ".md") {
			continue
		}
		dateStr := strings.TrimSuffix(de.Name(), ".md")
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if !cutoff.IsZero() && date.Before(cutoff) {
			continue
		}
		path := filepath.Join(e.store.dailyDir, de.Name())
		summary, err := e.store.parseDailyFile(path, date)
		if err != nil {
			continue
		}
		summaries = append(summaries, summary)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Date.After(summaries[j].Date)
	})
	return summaries, nil
}

func collectRepeatedFacts(summaries []dailySummary, minOccurrences int) []string {
	counts := map[string]int{}
	firstSeen := map[string]string{}
	for _, summary := range summaries {
		facts := extractFacts(summary.Content)
		for _, fact := range facts {
			normalized := normalizeFact(fact)
			if normalized == "" {
				continue
			}
			counts[normalized]++
			if _, ok := firstSeen[normalized]; !ok {
				firstSeen[normalized] = fact
			}
		}
	}

	if len(counts) == 0 {
		return nil
	}

	var candidates []factScore
	for normalized, count := range counts {
		if count < minOccurrences {
			continue
		}
		candidates = append(candidates, factScore{
			fact:  firstSeen[normalized],
			count: count,
		})
	}
	if len(candidates) == 0 {
		return nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].count == candidates[j].count {
			return candidates[i].fact < candidates[j].fact
		}
		return candidates[i].count > candidates[j].count
	})

	out := make([]string, 0, len(candidates))
	for _, item := range candidates {
		out = append(out, item.fact)
	}
	return out
}

type factScore struct {
	fact  string
	count int
}

func extractFacts(content string) []string {
	var facts []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		fact := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if fact == "" {
			continue
		}
		facts = append(facts, fact)
	}
	return facts
}

func normalizeFact(fact string) string {
	trimmed := strings.TrimSpace(strings.ToLower(fact))
	trimmed = strings.Trim(trimmed, ".;:")
	trimmed = strings.Join(strings.Fields(trimmed), " ")
	return trimmed
}

func upsertLongTermContent(existing string, newFacts []string, now time.Time) (string, int, bool) {
	lines := splitLines(existing)
	existingFacts, start, end := findExtractedFacts(lines)
	merged, added := mergeFacts(existingFacts, newFacts)
	if added == 0 && existing != "" {
		return existing, 0, false
	}

	updatedLine := fmt.Sprintf("Updated: %s", now.UTC().Format("2006-01-02 15:00"))
	lines = removeUpdatedLines(lines)
	lines = ensureHeader(lines)
	lines = insertUpdatedLine(lines, updatedLine)
	lines = replaceExtractedFacts(lines, merged, start, end)

	content := strings.Join(lines, "\n")
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content, added, true
}

func splitLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if content == "" {
		return nil
	}
	return strings.Split(content, "\n")
}

func removeUpdatedLines(lines []string) []string {
	if len(lines) == 0 {
		return lines
	}
	filtered := lines[:0]
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "Updated:") {
			continue
		}
		filtered = append(filtered, line)
	}
	return filtered
}

func ensureHeader(lines []string) []string {
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			return lines
		}
	}
	header := []string{"# Long-Term Memory", ""}
	return append(header, lines...)
}

func insertUpdatedLine(lines []string, updatedLine string) []string {
	insertAt := -1
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			insertAt = i + 1
			break
		}
	}
	if insertAt < 0 {
		return append([]string{updatedLine, ""}, lines...)
	}

	out := make([]string, 0, len(lines)+2)
	out = append(out, lines[:insertAt]...)
	out = append(out, updatedLine)
	out = append(out, "")
	out = append(out, lines[insertAt:]...)
	return out
}

func findExtractedFacts(lines []string) ([]string, int, int) {
	start := -1
	end := len(lines)
	var facts []string
	for i, line := range lines {
		if strings.TrimSpace(line) == "## Extracted Facts" {
			start = i
			for j := i + 1; j < len(lines); j++ {
				trimmed := strings.TrimSpace(lines[j])
				if strings.HasPrefix(trimmed, "## ") {
					end = j
					break
				}
				if strings.HasPrefix(trimmed, "- ") {
					fact := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
					if fact != "" {
						facts = append(facts, fact)
					}
				}
			}
			break
		}
	}
	return facts, start, end
}

func replaceExtractedFacts(lines []string, facts []string, start, end int) []string {
	section := []string{"## Extracted Facts"}
	for _, fact := range facts {
		section = append(section, "- "+fact)
	}
	if start >= 0 {
		out := make([]string, 0, len(lines)-max(0, end-start)+len(section))
		out = append(out, lines[:start]...)
		out = append(out, section...)
		if end < len(lines) {
			out = append(out, lines[end:]...)
		}
		return out
	}

	out := append([]string{}, lines...)
	if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		out = append(out, "")
	}
	out = append(out, section...)
	return out
}

func mergeFacts(existing []string, incoming []string) ([]string, int) {
	seen := map[string]bool{}
	merged := make([]string, 0, len(existing)+len(incoming))
	for _, fact := range existing {
		normalized := normalizeFact(fact)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		merged = append(merged, fact)
	}
	added := 0
	for _, fact := range incoming {
		normalized := normalizeFact(fact)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		merged = append(merged, fact)
		added++
	}
	return merged, added
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
