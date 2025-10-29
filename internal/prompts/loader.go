package prompts

import (
	"alex/internal/agent/ports"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed *.md
var promptFS embed.FS

// Ensure Loader implements ports.PromptLoader interface
var _ ports.PromptLoader = (*Loader)(nil)

// Loader handles loading and rendering prompt templates
type Loader struct {
	templates map[string]string
}

// New creates a new prompt loader
func New() *Loader {
	loader := &Loader{
		templates: make(map[string]string),
	}

	// Load all prompt templates
	_ = loader.loadTemplates()

	return loader
}

// loadTemplates loads all markdown prompt templates from embedded filesystem
func (l *Loader) loadTemplates() error {
	entries, err := promptFS.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read prompts directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			content, err := promptFS.ReadFile(entry.Name())
			if err != nil {
				return fmt.Errorf("failed to read prompt file %s: %w", entry.Name(), err)
			}

			templateName := strings.TrimSuffix(entry.Name(), ".md")
			l.templates[templateName] = string(content)
		}
	}

	return nil
}

// Get returns a prompt template by name
func (l *Loader) Get(name string) (string, error) {
	content, exists := l.templates[name]
	if !exists {
		return "", fmt.Errorf("prompt template '%s' not found", name)
	}
	return content, nil
}

// Render renders a prompt template with variable substitution
func (l *Loader) Render(name string, variables map[string]string) (string, error) {
	content, err := l.Get(name)
	if err != nil {
		return "", err
	}

	// Simple variable substitution
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		content = strings.ReplaceAll(content, placeholder, value)
	}

	return content, nil
}

// List returns all available prompt template names
func (l *Loader) List() []string {
	names := make([]string, 0, len(l.templates))
	for name := range l.templates {
		names = append(names, name)
	}
	return names
}

// GetSystemPrompt returns the system prompt with context
func (l *Loader) GetSystemPrompt(workingDir, goal string, analysis *ports.TaskAnalysisInfo) (string, error) {
	// Load project memory (ALEX.md or CLAUDE.md)
	memory := l.loadProjectMemory(workingDir)

	// Load git information
	gitInfo := l.loadGitInfo(workingDir)

	// Format task analysis if provided
	var taskAnalysis string
	if analysis != nil && analysis.Action != "" {
		taskAnalysis = fmt.Sprintf("Action: %s\nGoal: %s\nApproach: %s",
			analysis.Action, analysis.Goal, analysis.Approach)
	}

	skillsInfo := l.loadSkillsInfo(workingDir)

	variables := map[string]string{
		"WorkingDir":   workingDir,
		"Goal":         goal,
		"Memory":       memory,
		"GitInfo":      gitInfo,
		"TaskAnalysis": taskAnalysis,
	}

	prompt, err := l.Render("coder", variables)
	if err != nil {
		return "", err
	}

	if skillsInfo != "" {
		prompt = fmt.Sprintf("%s\n\n---\n# Custom Skills\n%s", prompt, skillsInfo)
	}

	return prompt, nil
}

// loadProjectMemory loads project memory from ALEX.md file with CLAUDE.md fallback
func (l *Loader) loadProjectMemory(workingDir string) string {
	defaultMemory := "You are a helpful AI assistant."

	if workingDir == "" {
		return defaultMemory
	}

	// Try ALEX.md first
	alexPath := filepath.Join(workingDir, "ALEX.md")
	if content := l.tryReadFile(alexPath); content != "" {
		return content
	}

	// Fallback to CLAUDE.md
	claudePath := filepath.Join(workingDir, "CLAUDE.md")
	if content := l.tryReadFile(claudePath); content != "" {
		return content
	}

	return defaultMemory
}

// loadGitInfo loads current git information
func (l *Loader) loadGitInfo(workingDir string) string {
	if workingDir == "" {
		return "Not in a git repository"
	}

	var gitInfo strings.Builder

	// Save current directory
	oldWd, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	if err := os.Chdir(workingDir); err != nil {
		return "Not in a git repository"
	}

	// Get current branch
	if output, err := exec.Command("git", "branch", "--show-current").Output(); err == nil {
		branch := strings.TrimSpace(string(output))
		if branch != "" {
			gitInfo.WriteString(fmt.Sprintf("Current branch: %s\n", branch))
		}
	}

	// Get status
	if output, err := exec.Command("git", "status", "--porcelain").Output(); err == nil {
		status := strings.TrimSpace(string(output))
		if status != "" {
			gitInfo.WriteString(fmt.Sprintf("Status:\n%s\n", status))
		} else {
			gitInfo.WriteString("Status: Clean working directory\n")
		}
	}

	// Get recent commits
	if output, err := exec.Command("git", "log", "--oneline", "-5").Output(); err == nil {
		commits := strings.TrimSpace(string(output))
		if commits != "" {
			gitInfo.WriteString(fmt.Sprintf("Recent commits:\n%s\n", commits))
		}
	}

	result := gitInfo.String()
	if result == "" {
		return "Not in a git repository"
	}

	return strings.TrimSpace(result)
}

// tryReadFile attempts to read a file
func (l *Loader) tryReadFile(filePath string) string {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return ""
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}

func (l *Loader) loadSkillsInfo(workingDir string) string {
	if workingDir == "" {
		return ""
	}

	skillsDir := filepath.Join(workingDir, "skills")
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return ""
	}

	var lines []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}

		title, summary := l.extractSkillSummary(filepath.Join(skillsDir, name))
		if title == "" {
			title = strings.TrimSuffix(name, filepath.Ext(name))
		}
		display := fmt.Sprintf("- %s — 使用 `file_read(\"skills/%s\")` 查看完整指南", title, name)
		if summary != "" {
			display = fmt.Sprintf("%s。%s", display, summary)
		}
		lines = append(lines, display)
	}

	if len(lines) == 0 {
		return ""
	}

	sort.Strings(lines)

	return fmt.Sprintf("在项目根目录检测到自定义技能：\n%s", strings.Join(lines, "\n"))
}

func (l *Loader) extractSkillSummary(path string) (string, string) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}

	lines := strings.Split(string(content), "\n")
	var title string
	var summary string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "<!--") {
			continue
		}

		normalized := strings.TrimSpace(trimmed)
		if strings.HasPrefix(normalized, "#") {
			normalized = strings.TrimSpace(strings.TrimLeft(normalized, "#"))
		}

		if title == "" {
			title = normalized
			continue
		}

		summary = strings.TrimSpace(strings.TrimLeft(normalized, "-* "))
		if summary != "" {
			break
		}
	}

	return title, summary
}
