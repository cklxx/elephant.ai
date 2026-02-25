package context

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultBootstrapMaxChars = 20000

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
			record.Content, record.Truncated = truncateWithMarker(string(content), maxChars)
			break
		}
		if strings.TrimSpace(record.Content) == "" && record.Path == "" {
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

type bootstrapCandidate struct {
	path   string
	source string
}

func bootstrapCandidates(name, globalRoot, repoRoot string) []bootstrapCandidate {
	if filepath.IsAbs(name) {
		return []bootstrapCandidate{{path: filepath.Clean(name), source: "direct"}}
	}
	var candidates []bootstrapCandidate
	if strings.TrimSpace(globalRoot) != "" {
		candidates = append(candidates, bootstrapCandidate{
			path:   filepath.Join(globalRoot, name),
			source: "global",
		})
	}
	if strings.TrimSpace(repoRoot) != "" {
		candidates = append(candidates, bootstrapCandidate{
			path:   filepath.Join(repoRoot, name),
			source: "workspace",
		})
	}
	return candidates
}

func truncateWithMarker(raw string, limit int) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed, false
	}
	return strings.TrimSpace(string(runes[:limit])) + "\n...[TRUNCATED]", true
}

