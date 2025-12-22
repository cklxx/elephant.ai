package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill represents a reusable workflow playbook stored as Markdown.
type Skill struct {
	Name        string
	Description string
	Title       string
	Body        string
	SourcePath  string
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
		if os.IsNotExist(err) {
			return Library{}, nil
		}
		return Library{}, fmt.Errorf("stat skills dir: %w", err)
	}
	if !info.IsDir() {
		return Library{}, fmt.Errorf("skills dir %s is not a directory", trimmed)
	}

	entries, err := os.ReadDir(trimmed)
	if err != nil {
		return Library{}, fmt.Errorf("read skills dir: %w", err)
	}

	var skills []Skill
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".md") && !strings.HasSuffix(lower, ".mdx") {
			continue
		}

		path := filepath.Join(trimmed, name)
		skill, err := parseSkillFile(path)
		if err != nil {
			return Library{}, err
		}
		skills = append(skills, skill)
	}

	return buildLibrary(skills, trimmed)
}

type frontMatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func parseSkillFile(path string) (Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Skill{}, fmt.Errorf("read skill %s: %w", path, err)
	}
	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	metaText, bodyText, hasFrontMatter := splitFrontMatter(content)
	var meta frontMatter
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

	return Skill{
		Name:        strings.TrimSpace(meta.Name),
		Description: strings.TrimSpace(meta.Description),
		Title:       title,
		Body:        body,
		SourcePath:  path,
	}, nil
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

func buildLibrary(skills []Skill, root string) (Library, error) {
	byName := make(map[string]Skill, len(skills))
	var unique []Skill

	for _, skill := range skills {
		skill.Name = strings.TrimSpace(skill.Name)
		skill.Description = strings.TrimSpace(skill.Description)
		key := NormalizeName(skill.Name)
		if key == "" {
			return Library{}, fmt.Errorf("skill missing name: %s", skill.SourcePath)
		}
		if skill.Description == "" {
			return Library{}, fmt.Errorf("skill %s missing description", key)
		}
		if _, exists := byName[key]; exists {
			continue
		}
		skill.Name = key
		unique = append(unique, skill)
		byName[key] = skill
	}

	sort.Slice(unique, func(i, j int) bool { return unique[i].Name < unique[j].Name })

	return Library{skills: unique, byName: byName, root: strings.TrimSpace(root)}, nil
}
