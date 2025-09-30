package prompts

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed *.md
var promptFS embed.FS

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
func (l *Loader) GetSystemPrompt(workingDir, goal string) (string, error) {
	// Load project memory (ALEX.md or CLAUDE.md)
	memory := l.loadProjectMemory(workingDir)

	// Load git information
	gitInfo := l.loadGitInfo(workingDir)

	variables := map[string]string{
		"WorkingDir": workingDir,
		"Goal":       goal,
		"Memory":     memory,
		"GitInfo":    gitInfo,
	}

	return l.Render("coder", variables)
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
