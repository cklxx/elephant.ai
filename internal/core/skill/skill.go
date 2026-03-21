package skill

import (
	"os"
	"path/filepath"
	"strings"
)

// Skill represents a loaded skill document.
type Skill struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Body        string            `json:"body"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	Path        string            `json:"path,omitempty"` // source file path
}

// Discover scans directories for skill files (*.md) and returns discovered skills.
// Each markdown file becomes a Skill with Name derived from filename.
func Discover(dirs ...string) ([]Skill, error) {
	var skills []Skill

	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip unreadable entries
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(info.Name()), ".md") {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return nil // skip unreadable files
			}

			s := Skill{
				Name: deriveName(dir, path),
				Path: path,
			}

			content := string(data)
			desc, meta, body := parseFrontmatter(content)
			s.Description = desc
			s.Metadata = meta
			s.Body = body

			skills = append(skills, s)
			return nil
		})
		if err != nil {
			return skills, err
		}
	}

	return skills, nil
}

// deriveName builds a skill name from the file path relative to the base dir.
// e.g., base="skills", path="skills/deploy/rollback.md" -> "deploy.rollback"
func deriveName(base, path string) string {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		rel = filepath.Base(path)
	}

	// Remove .md extension
	rel = strings.TrimSuffix(rel, filepath.Ext(rel))

	// Replace path separators with dots
	rel = strings.ReplaceAll(rel, string(filepath.Separator), ".")

	return rel
}

// parseFrontmatter extracts YAML frontmatter (between --- delimiters) from content.
// Returns description, metadata map, and the remaining body.
func parseFrontmatter(content string) (description string, metadata map[string]string, body string) {
	metadata = make(map[string]string)

	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return "", metadata, content
	}

	// Find the closing ---
	rest := trimmed[3:] // skip opening ---
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	// Handle empty frontmatter (--- immediately follows opening ---)
	var endIdx int
	if strings.HasPrefix(rest, "---") {
		endIdx = 0
	} else {
		endIdx = strings.Index(rest, "\n---")
		if endIdx >= 0 {
			endIdx++ // point to the '---' itself
		}
	}
	if endIdx < 0 {
		return "", metadata, content
	}

	frontmatter := rest[:endIdx]
	body = strings.TrimLeft(rest[endIdx+3:], "\r\n") // skip past closing --- and newlines

	// Parse simple key: value pairs from frontmatter
	for _, line := range strings.Split(frontmatter, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:colonIdx])
		value := strings.TrimSpace(line[colonIdx+1:])

		switch key {
		case "description":
			description = value
		default:
			metadata[key] = value
		}
	}

	return description, metadata, body
}
