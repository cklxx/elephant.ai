package okr

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const frontmatterSeparator = "---"

// GoalStore provides read/write access to goal files on disk.
type GoalStore struct {
	config OKRConfig
}

// NewGoalStore creates a GoalStore with the given configuration.
func NewGoalStore(cfg OKRConfig) *GoalStore {
	return &GoalStore{config: cfg}
}

// ListGoals returns the IDs of all goal files in the goals directory.
// IDs are derived from filenames by stripping the .md extension.
func (s *GoalStore) ListGoals() ([]string, error) {
	entries, err := os.ReadDir(s.config.GoalsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list goals: %w", err)
	}

	var ids []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".md") {
			ids = append(ids, strings.TrimSuffix(name, ".md"))
		}
	}
	return ids, nil
}

// ReadGoal reads and parses a goal file by ID.
func (s *GoalStore) ReadGoal(goalID string) (*GoalFile, error) {
	path := s.config.GoalPath(goalID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read goal %s: %w", goalID, err)
	}
	return ParseGoalFile(data)
}

// WriteGoal writes a goal file atomically via temp + rename.
func (s *GoalStore) WriteGoal(goalID string, goal *GoalFile) error {
	if err := os.MkdirAll(s.config.GoalsRoot, 0o755); err != nil {
		return fmt.Errorf("create goals dir: %w", err)
	}

	data, err := RenderGoalFile(goal)
	if err != nil {
		return fmt.Errorf("render goal %s: %w", goalID, err)
	}

	dest := s.config.GoalPath(goalID)
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp goal: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename goal: %w", err)
	}
	return nil
}

// ReadGoalRaw returns the raw bytes of a goal file.
func (s *GoalStore) ReadGoalRaw(goalID string) ([]byte, error) {
	path := s.config.GoalPath(goalID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read goal %s: %w", goalID, err)
	}
	return data, nil
}

// WriteGoalRaw writes raw content to a goal file atomically.
// It validates that the content can be parsed before writing.
func (s *GoalStore) WriteGoalRaw(goalID string, content []byte) error {
	if _, err := ParseGoalFile(content); err != nil {
		return fmt.Errorf("validate goal content: %w", err)
	}

	if err := os.MkdirAll(s.config.GoalsRoot, 0o755); err != nil {
		return fmt.Errorf("create goals dir: %w", err)
	}

	dest := s.config.GoalPath(goalID)
	tmp := dest + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("write temp goal: %w", err)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename goal: %w", err)
	}
	return nil
}

// ParseGoalFile splits a markdown file into YAML frontmatter and body.
func ParseGoalFile(data []byte) (*GoalFile, error) {
	content := string(data)
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, frontmatterSeparator) {
		return nil, fmt.Errorf("goal file must start with YAML frontmatter (---)")
	}

	// Find closing ---
	rest := content[len(frontmatterSeparator):]
	idx := strings.Index(rest, "\n"+frontmatterSeparator)
	if idx < 0 {
		return nil, fmt.Errorf("unterminated YAML frontmatter")
	}

	yamlBlock := strings.TrimSpace(rest[:idx])
	body := strings.TrimSpace(rest[idx+len("\n"+frontmatterSeparator):])

	var meta GoalMeta
	if err := yaml.Unmarshal([]byte(yamlBlock), &meta); err != nil {
		return nil, fmt.Errorf("parse YAML frontmatter: %w", err)
	}

	return &GoalFile{
		Meta: meta,
		Body: body,
	}, nil
}

// RenderGoalFile serializes a GoalFile back to markdown with YAML frontmatter.
func RenderGoalFile(goal *GoalFile) ([]byte, error) {
	var buf bytes.Buffer

	yamlData, err := yaml.Marshal(&goal.Meta)
	if err != nil {
		return nil, fmt.Errorf("marshal YAML frontmatter: %w", err)
	}

	buf.WriteString(frontmatterSeparator + "\n")
	buf.Write(yamlData)
	buf.WriteString(frontmatterSeparator + "\n")

	if body := strings.TrimSpace(goal.Body); body != "" {
		buf.WriteString("\n")
		buf.WriteString(body)
		buf.WriteString("\n")
	}

	return buf.Bytes(), nil
}

// GoalsRoot returns the configured goals directory path.
func (s *GoalStore) GoalsRoot() string {
	return s.config.GoalsRoot
}

// GoalPath returns the full path for a goal ID.
func (s *GoalStore) GoalPath(goalID string) string {
	return s.config.GoalPath(goalID)
}

// GoalExists checks whether a goal file exists on disk.
func (s *GoalStore) GoalExists(goalID string) bool {
	path := s.config.GoalPath(goalID)
	_, err := os.Stat(path)
	return err == nil
}

// DeleteGoal removes a goal file from disk.
func (s *GoalStore) DeleteGoal(goalID string) error {
	path := s.config.GoalPath(goalID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete goal %s: %w", goalID, err)
	}
	return nil
}

// ListActiveGoals returns parsed GoalFiles for all goals with status "active".
func (s *GoalStore) ListActiveGoals() ([]*GoalFile, error) {
	ids, err := s.ListGoals()
	if err != nil {
		return nil, err
	}

	var active []*GoalFile
	for _, id := range ids {
		goal, err := s.ReadGoal(id)
		if err != nil {
			continue
		}
		if goal.Meta.Status == "active" {
			active = append(active, goal)
		}
	}
	return active, nil
}

// GoalIDFromPath extracts the goal ID from a file path.
func GoalIDFromPath(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".md")
}
