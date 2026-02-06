package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a reusable workflow playbook stored as Markdown.
type Skill struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Title       string `yaml:"-"`
	Body        string `yaml:"-"`
	SourcePath  string `yaml:"-"`

	Triggers       *SkillTriggers `yaml:"triggers,omitempty"`
	Priority       int            `yaml:"priority,omitempty"`
	ExclusiveGroup string         `yaml:"exclusive_group,omitempty"`
	Prerequisites  []string       `yaml:"prerequisites,omitempty"`
	MaxTokens      int            `yaml:"max_tokens,omitempty"`
	Cooldown       int            `yaml:"cooldown,omitempty"`
	Output         *SkillOutput   `yaml:"output,omitempty"`
	Chain          []ChainStep    `yaml:"chain,omitempty"`
}

type SkillTriggers struct {
	IntentPatterns      []string        `yaml:"intent_patterns,omitempty"`
	ToolSignals         []string        `yaml:"tool_signals,omitempty"`
	ContextSignals      *ContextSignals `yaml:"context_signals,omitempty"`
	ConfidenceThreshold float64         `yaml:"confidence_threshold,omitempty"`
}

type ContextSignals struct {
	Keywords []string            `yaml:"keywords,omitempty"`
	Slots    map[string][]string `yaml:"slots,omitempty"`
}

type SkillOutput struct {
	Format       string `yaml:"format,omitempty"`
	Artifacts    bool   `yaml:"artifacts,omitempty"`
	ArtifactType string `yaml:"artifact_type,omitempty"`
}

type ChainStep struct {
	SkillName string            `yaml:"skill"`
	InputFrom string            `yaml:"input_from,omitempty"`
	OutputAs  string            `yaml:"output_as,omitempty"`
	Params    map[string]string `yaml:"params,omitempty"`
}

type SkillChain struct {
	Steps []ChainStep `yaml:"chain"`
}

// Library is a loaded collection of skills.
type Library struct {
	skills []Skill
	byName map[string]Skill
	root   string
}

// Root returns the directory the library was loaded from (empty for none).
func (l Library) Root() string { return l.root }

// List returns all skills sorted by name.
func (l Library) List() []Skill {
	out := append([]Skill(nil), l.skills...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns a skill by name.
func (l Library) Get(name string) (Skill, bool) {
	if l.byName == nil {
		return Skill{}, false
	}
	skill, ok := l.byName[NormalizeName(name)]
	return skill, ok
}

// Load loads skill Markdown files from dir.
func Load(dir string) (Library, error) {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return Library{}, nil
	}

	info, err := os.Stat(trimmed)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Library{}, nil
		}
		return Library{}, fmt.Errorf("stat skills dir: %w", err)
	}
	if !info.IsDir() {
		return Library{}, fmt.Errorf("skills dir %s is not a directory", trimmed)
	}

	skillFiles, err := discoverSkillFiles(trimmed)
	if err != nil {
		return Library{}, fmt.Errorf("discover skills: %w", err)
	}

	skills := make([]Skill, 0, len(skillFiles))
	byName := make(map[string]Skill, len(skillFiles))
	for _, path := range skillFiles {
		skill, err := parseSkillFile(path)
		if err != nil {
			return Library{}, err
		}
		if skill.Name == "" {
			return Library{}, fmt.Errorf("skill %s missing name front matter", path)
		}
		if skill.Description == "" {
			return Library{}, fmt.Errorf("skill %s missing description front matter", path)
		}
		key := NormalizeName(skill.Name)
		if _, exists := byName[key]; exists {
			return Library{}, fmt.Errorf("duplicate skill name %q in %s", key, path)
		}
		byName[key] = skill
		skills = append(skills, skill)
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })

	return Library{skills: skills, byName: byName, root: trimmed}, nil
}

func discoverSkillFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			for _, candidate := range []string{"SKILL.md", "SKILL.mdx"} {
				path := filepath.Join(root, name, candidate)
				info, err := os.Stat(path)
				if err == nil && !info.IsDir() {
					paths = append(paths, path)
					break
				}
			}
			continue
		}
	}

	sort.Strings(paths)
	return paths, nil
}

type skillFrontMatter struct {
	Name           string         `yaml:"name"`
	Description    string         `yaml:"description"`
	Triggers       *SkillTriggers `yaml:"triggers,omitempty"`
	Priority       int            `yaml:"priority,omitempty"`
	ExclusiveGroup string         `yaml:"exclusive_group,omitempty"`
	Prerequisites  []string       `yaml:"prerequisites,omitempty"`
	MaxTokens      int            `yaml:"max_tokens,omitempty"`
	Cooldown       int            `yaml:"cooldown,omitempty"`
	Output         *SkillOutput   `yaml:"output,omitempty"`
	Chain          []ChainStep    `yaml:"chain,omitempty"`
}

func parseSkillFile(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("read skill %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	metaText, bodyText, hasFrontMatter := splitFrontMatter(content)
	var meta skillFrontMatter
	if hasFrontMatter {
		if err := yaml.Unmarshal([]byte(metaText), &meta); err != nil {
			return Skill{}, fmt.Errorf("parse skill front matter %s: %w", path, err)
		}
	}

	body := strings.TrimSpace(bodyText)
	title := extractMarkdownTitle(body)
	if title == "" {
		title = meta.Name
	}

	skill := Skill{
		Name:           strings.TrimSpace(meta.Name),
		Description:    strings.TrimSpace(meta.Description),
		Title:          title,
		Body:           body,
		SourcePath:     path,
		Triggers:       meta.Triggers,
		Priority:       meta.Priority,
		ExclusiveGroup: meta.ExclusiveGroup,
		Prerequisites:  meta.Prerequisites,
		MaxTokens:      meta.MaxTokens,
		Cooldown:       meta.Cooldown,
		Output:         meta.Output,
		Chain:          meta.Chain,
	}
	if skill.Priority == 0 {
		skill.Priority = 5
	}
	if skill.MaxTokens == 0 {
		skill.MaxTokens = 2000
	}
	return skill, nil
}

func splitFrontMatter(content string) (string, string, bool) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return "", content, false
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			meta := strings.Join(lines[1:i], "\n")
			body := strings.Join(lines[i+1:], "\n")
			return meta, body, true
		}
	}
	return "", content, false
}

func extractMarkdownTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		}
		break
	}
	return ""
}

// NormalizeName normalizes a skill name for lookups.
func NormalizeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.ReplaceAll(name, " ", "_")
	return name
}
