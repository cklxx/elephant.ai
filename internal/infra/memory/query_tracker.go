package memory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// QueryCategory classifies a user query for distribution tracking.
type QueryCategory string

const (
	CategoryCode         QueryCategory = "code"
	CategoryArchitecture QueryCategory = "architecture"
	CategoryOps          QueryCategory = "ops"
	CategoryDebug        QueryCategory = "debug"
	CategoryDocs         QueryCategory = "docs"
	CategoryGeneral      QueryCategory = "general"
)

var allCategories = []QueryCategory{
	CategoryCode, CategoryArchitecture, CategoryOps,
	CategoryDebug, CategoryDocs, CategoryGeneral,
}

var categoryKeywords = map[QueryCategory][]string{
	CategoryCode:         {"implement", "function", "bug", "fix", "refactor", "test", "compile", "error", "code", "method", "class", "variable", "type", "struct", "interface", "package", "import"},
	CategoryArchitecture: {"design", "architecture", "pattern", "boundary", "layer", "module", "dependency", "decouple", "diagram"},
	CategoryOps:          {"deploy", "config", "yaml", "env", "ci", "pipeline", "monitor", "docker", "kubernetes", "helm", "terraform"},
	CategoryDebug:        {"debug", "log", "trace", "crash", "panic", "stacktrace", "breakpoint", "inspect", "profile"},
	CategoryDocs:         {"document", "readme", "explain", "describe", "comment", "annotation", "wiki"},
}

// QueryDistribution holds per-category counts.
type QueryDistribution struct {
	Counts map[QueryCategory]int
	Total  int
}

// QueryTracker classifies queries and maintains a rolling distribution.
type QueryTracker struct {
	rootDir string
	mu      sync.Mutex
}

// NewQueryTracker creates a tracker rooted at the given memory directory.
func NewQueryTracker(rootDir string) *QueryTracker {
	return &QueryTracker{rootDir: rootDir}
}

// Classify returns the best-matching category for a query using keyword heuristics.
func (t *QueryTracker) Classify(query string) QueryCategory {
	lower := strings.ToLower(query)
	best := CategoryGeneral
	bestScore := 0
	for cat, keywords := range categoryKeywords {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			best = cat
		}
	}
	return best
}

// Record increments the count for the given category.
func (t *QueryTracker) Record(_ context.Context, _ string, category QueryCategory) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	dist, err := t.load()
	if err != nil {
		dist = QueryDistribution{Counts: make(map[QueryCategory]int)}
	}
	dist.Counts[category]++
	dist.Total++
	return t.save(dist)
}

// Load returns the current query distribution.
func (t *QueryTracker) Load(_ context.Context, _ string) (QueryDistribution, error) {
	return t.load()
}

// Weights returns normalized weights (0-1) for each category.
func (t *QueryTracker) Weights(_ context.Context, _ string) (map[QueryCategory]float64, error) {
	dist, err := t.load()
	if err != nil {
		return nil, err
	}
	weights := make(map[QueryCategory]float64, len(allCategories))
	if dist.Total == 0 {
		for _, cat := range allCategories {
			weights[cat] = 1.0 / float64(len(allCategories))
		}
		return weights, nil
	}
	for _, cat := range allCategories {
		weights[cat] = float64(dist.Counts[cat]) / float64(dist.Total)
	}
	return weights, nil
}

func (t *QueryTracker) statsPath() string {
	root := ResolveUserRoot(t.rootDir, "")
	return filepath.Join(root, queryStatsFileName)
}

func (t *QueryTracker) load() (QueryDistribution, error) {
	path := t.statsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return QueryDistribution{Counts: make(map[QueryCategory]int)}, nil
		}
		return QueryDistribution{}, err
	}
	return parseQueryStats(string(data)), nil
}

func (t *QueryTracker) save(dist QueryDistribution) error {
	path := t.statsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(formatQueryStats(dist)), 0o644)
}

// formatQueryStats renders the distribution as a Markdown table.
func formatQueryStats(dist QueryDistribution) string {
	var b strings.Builder
	b.WriteString("# Query Distribution\n\n")
	b.WriteString("| Category | Count |\n")
	b.WriteString("|----------|-------|\n")
	for _, cat := range allCategories {
		count := dist.Counts[cat]
		b.WriteString(fmt.Sprintf("| %s | %d |\n", cat, count))
	}
	b.WriteString(fmt.Sprintf("\nTotal: %d\n", dist.Total))
	return b.String()
}

// parseQueryStats reads a Markdown table back into a QueryDistribution.
func parseQueryStats(content string) QueryDistribution {
	dist := QueryDistribution{Counts: make(map[QueryCategory]int)}
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Total:") {
			if n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(trimmed, "Total:"))); err == nil {
				dist.Total = n
			}
			continue
		}
		if !strings.HasPrefix(trimmed, "|") || strings.Contains(trimmed, "---") || strings.Contains(trimmed, "Category") {
			continue
		}
		parts := strings.Split(trimmed, "|")
		if len(parts) < 3 {
			continue
		}
		cat := QueryCategory(strings.TrimSpace(parts[1]))
		countStr := strings.TrimSpace(parts[2])
		if n, err := strconv.Atoi(countStr); err == nil {
			dist.Counts[cat] = n
		}
	}
	return dist
}
