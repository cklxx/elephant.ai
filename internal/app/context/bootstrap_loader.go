package context

import (
	"alex/internal/shared/utils"
	"os"
	"path/filepath"
	"strings"
)

const defaultBootstrapMaxChars = 20000
const defaultBootstrapPerFileChars = 2000

type bootstrapRecord struct {
	Name      string
	Path      string
	Content   string
	Source    string // global | workspace | direct | missing
	Missing   bool
	Truncated bool
}

func loadBootstrapRecords(repoRoot string, files []string, maxChars int) []bootstrapRecord {
	cleanRoot := strings.TrimSpace(repoRoot)
	if cleanRoot != "" {
		cleanRoot = filepath.Clean(cleanRoot)
	}
	if maxChars <= 0 {
		maxChars = defaultBootstrapMaxChars
	}
	remainingChars := maxChars

	globalRoot := ""
	if home, err := os.UserHomeDir(); err == nil {
		globalRoot = filepath.Join(home, ".alex", "memory")
	}

	records := make([]bootstrapRecord, 0, len(files))
	for _, raw := range files {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		record := bootstrapRecord{Name: name}
		candidates := bootstrapCandidates(name, globalRoot, cleanRoot)
		for _, candidate := range candidates {
			content, err := os.ReadFile(candidate.path)
			if err != nil {
				continue
			}
			record.Path = candidate.path
			record.Source = candidate.source
			limit := bootstrapRecordLimit(remainingChars, maxChars)
			if limit <= 0 {
				record.Content = "(omitted: bootstrap character budget exhausted)"
				record.Truncated = true
				break
			}
			var consumed int
			record.Content, record.Truncated, consumed = truncateWithMarkerAndCount(string(content), limit)
			remainingChars -= consumed
			break
		}
		if utils.IsBlank(record.Content) && record.Path == "" {
			record.Missing = true
			record.Source = "missing"
			if len(candidates) > 0 {
				record.Path = candidates[0].path
			}
		}
		records = append(records, record)
	}
	return records
}

func bootstrapRecordLimit(remainingChars, maxChars int) int {
	if remainingChars <= 0 || maxChars <= 0 {
		return 0
	}
	limit := defaultBootstrapPerFileChars
	if limit > maxChars {
		limit = maxChars
	}
	if limit > remainingChars {
		limit = remainingChars
	}
	return limit
}

type bootstrapCandidate struct {
	path   string
	source string
}

func bootstrapCandidates(name, globalRoot, repoRoot string) []bootstrapCandidate {
	if filepath.IsAbs(name) {
		return []bootstrapCandidate{{path: filepath.Clean(name), source: "direct"}}
	}
	var candidates []bootstrapCandidate
	if utils.HasContent(globalRoot) {
		candidates = append(candidates, bootstrapCandidate{
			path:   filepath.Join(globalRoot, name),
			source: "global",
		})
	}
	if utils.HasContent(repoRoot) {
		candidates = append(candidates, bootstrapCandidate{
			path:   filepath.Join(repoRoot, name),
			source: "workspace",
		})
	}
	return candidates
}

func truncateWithMarker(raw string, limit int) (string, bool) {
	content, truncated, _ := truncateWithMarkerAndCount(raw, limit)
	return content, truncated
}

func truncateWithMarkerAndCount(raw string, limit int) (string, bool, int) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false, 0
	}
	runes := []rune(trimmed)
	if limit <= 0 {
		return "", true, 0
	}
	if len(runes) <= limit {
		return trimmed, false, len(runes)
	}
	return strings.TrimSpace(string(runes[:limit])) + "\n...[TRUNCATED]", true, limit
}
