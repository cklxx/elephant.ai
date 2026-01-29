package context

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
)

const (
	maxSOPContentBytes = 4096
	sopCacheTTL        = 30 * time.Minute
)

// SOPResolver reads markdown files, extracts sections by anchor, caches
// results, and enriches KnowledgeReference with resolved content.
type SOPResolver struct {
	repoRoot string
	logger   logging.Logger

	mu    sync.RWMutex
	cache map[string]sopCacheEntry
}

type sopCacheEntry struct {
	content   string
	expiresAt time.Time
}

// NewSOPResolver creates a resolver rooted at the given repository root.
func NewSOPResolver(repoRoot string, logger logging.Logger) *SOPResolver {
	if logging.IsNil(logger) {
		logger = logging.NewComponentLogger("SOPResolver")
	}
	return &SOPResolver{
		repoRoot: filepath.Clean(repoRoot),
		logger:   logger,
		cache:    make(map[string]sopCacheEntry),
	}
}

// ParseSOPRef splits a reference like "path/to/file.md#anchor" into filepath
// and anchor components. If no anchor is present, anchor is empty.
func ParseSOPRef(ref string) (filePath, anchor string) {
	ref = strings.TrimSpace(ref)
	idx := strings.LastIndex(ref, "#")
	if idx < 0 {
		return ref, ""
	}
	return ref[:idx], ref[idx+1:]
}

// ResolveRef resolves a single SOP reference to its markdown content.
func (r *SOPResolver) ResolveRef(ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", nil
	}

	// Check cache.
	r.mu.RLock()
	if entry, ok := r.cache[ref]; ok && time.Now().Before(entry.expiresAt) {
		r.mu.RUnlock()
		return entry.content, nil
	}
	r.mu.RUnlock()

	filePath, anchor := ParseSOPRef(ref)

	// Path traversal guard.
	absPath := filepath.Join(r.repoRoot, filePath)
	absPath = filepath.Clean(absPath)
	if !strings.HasPrefix(absPath, r.repoRoot+string(filepath.Separator)) && absPath != r.repoRoot {
		return "", fmt.Errorf("sop ref %q resolves outside repo root", ref)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			r.logger.Warn("SOP file not found: %s", absPath)
			return "", nil
		}
		return "", fmt.Errorf("read sop file %s: %w", filePath, err)
	}

	content := string(data)
	if anchor != "" {
		section := ExtractMarkdownSection(content, anchor)
		if section != "" {
			content = section
		} else {
			r.logger.Warn("Anchor %q not found in %s, using full file", anchor, filePath)
		}
	}

	// Truncation guard.
	if len(content) > maxSOPContentBytes {
		content = content[:maxSOPContentBytes] + "\n... (truncated)"
	}

	// Cache the result.
	r.mu.Lock()
	r.cache[ref] = sopCacheEntry{
		content:   content,
		expiresAt: time.Now().Add(sopCacheTTL),
	}
	r.mu.Unlock()

	return content, nil
}

// ResolveKnowledgeRefs iterates SOPRefs in each KnowledgeReference and
// populates ResolvedSOPContent with the resolved markdown content.
func (r *SOPResolver) ResolveKnowledgeRefs(refs []agent.KnowledgeReference) []agent.KnowledgeReference {
	out := make([]agent.KnowledgeReference, len(refs))
	copy(out, refs)

	for i := range out {
		if len(out[i].SOPRefs) == 0 {
			continue
		}
		resolved := make(map[string]string, len(out[i].SOPRefs))
		for _, ref := range out[i].SOPRefs {
			content, err := r.ResolveRef(ref)
			if err != nil {
				r.logger.Warn("Failed to resolve SOP ref %q: %v", ref, err)
				continue
			}
			if content != "" {
				resolved[ref] = content
			}
		}
		if len(resolved) > 0 {
			out[i].ResolvedSOPContent = resolved
		}
	}
	return out
}

// ExtractMarkdownSection scans markdown content for a heading whose slugified
// anchor matches the target, then captures everything until the next heading
// at the same or higher level.
func ExtractMarkdownSection(content, anchor string) string {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	var (
		capturing    bool
		captureLevel int
		captured     []string
	)

	for _, line := range lines {
		level, heading := parseHeading(line)
		if level > 0 {
			if capturing {
				// Stop if we hit a heading at the same or higher level.
				if level <= captureLevel {
					break
				}
			} else {
				slug := SlugifyHeading(heading)
				if slug == anchor {
					capturing = true
					captureLevel = level
					captured = append(captured, line)
					continue
				}
			}
		}
		if capturing {
			captured = append(captured, line)
		}
	}

	return strings.TrimSpace(strings.Join(captured, "\n"))
}

// parseHeading returns the heading level (1-6) and text, or 0 if not a heading.
func parseHeading(line string) (int, string) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return 0, ""
	}
	level := 0
	for _, ch := range trimmed {
		if ch == '#' {
			level++
		} else {
			break
		}
	}
	if level > 6 {
		return 0, ""
	}
	text := strings.TrimSpace(trimmed[level:])
	if text == "" && level > 0 {
		return level, ""
	}
	return level, text
}

var slugNonAlnum = regexp.MustCompile(`[^a-z0-9 -]`)

// SlugifyHeading converts a heading to a GitHub-style anchor slug.
// GitHub's algorithm: lowercase, strip non-alphanum except spaces/hyphens,
// replace spaces with hyphens. Consecutive hyphens are NOT collapsed.
func SlugifyHeading(heading string) string {
	heading = strings.TrimSpace(heading)
	heading = strings.ToLower(heading)
	// Remove non-alphanumeric characters except spaces and hyphens.
	heading = slugNonAlnum.ReplaceAllString(heading, "")
	// Replace spaces with hyphens.
	heading = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return '-'
		}
		return r
	}, heading)
	heading = strings.Trim(heading, "-")
	return heading
}

// SOPRefLabel returns a human-readable label for a SOP reference.
func SOPRefLabel(ref string) string {
	filePath, anchor := ParseSOPRef(ref)
	base := filepath.Base(filePath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.ReplaceAll(base, "_", " ")
	if anchor != "" {
		anchor = strings.ReplaceAll(anchor, "-", " ")
		return fmt.Sprintf("%s > %s", base, anchor)
	}
	return base
}
