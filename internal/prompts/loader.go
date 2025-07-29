package prompts

import (
	"alex/pkg/types"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed *.md
var promptFS embed.FS

// PromptTemplate represents a prompt template with metadata
type PromptTemplate struct {
	Name      string
	Content   string
	Variables map[string]string
}

// PromptLoader handles loading and rendering prompt templates
type PromptLoader struct {
	templates map[string]*PromptTemplate
}

// NewPromptLoader creates a new prompt loader
func NewPromptLoader() (*PromptLoader, error) {
	loader := &PromptLoader{
		templates: make(map[string]*PromptTemplate),
	}

	// Load all prompt templates
	if err := loader.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load prompt templates: %w", err)
	}

	return loader, nil
}

// loadTemplates loads all markdown prompt templates from embedded filesystem
func (p *PromptLoader) loadTemplates() error {
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
			p.templates[templateName] = &PromptTemplate{
				Name:      templateName,
				Content:   string(content),
				Variables: make(map[string]string),
			}
		}
	}

	return nil
}

// GetPrompt returns a prompt template by name
func (p *PromptLoader) GetPrompt(name string) (*PromptTemplate, error) {
	template, exists := p.templates[name]
	if !exists {
		return nil, fmt.Errorf("prompt template '%s' not found", name)
	}

	return template, nil
}

// RenderPrompt renders a prompt template with variable substitution
func (p *PromptLoader) RenderPrompt(name string, variables map[string]string) (string, error) {
	template, err := p.GetPrompt(name)
	if err != nil {
		return "", err
	}

	content := template.Content

	// Simple variable substitution
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		content = strings.ReplaceAll(content, placeholder, value)
	}

	return content, nil
}

// ListPrompts returns all available prompt template names
func (p *PromptLoader) ListPrompts() []string {
	names := make([]string, 0, len(p.templates))
	for name := range p.templates {
		names = append(names, name)
	}
	return names
}

// GetReActThinkingPrompt returns the ReAct thinking phase prompt
func (p *PromptLoader) GetReActThinkingPrompt(taskCtx *types.ReactTaskContext) (string, error) {
	// Try to read ALEX.md from working directory, fallback to default if not found
	memory := p.loadProjectMemory(taskCtx.WorkingDir)
	
	// Load git information
	gitInfo := p.loadGitInfo(taskCtx.WorkingDir)

	variables := map[string]string{
		"WorkingDir":    taskCtx.WorkingDir,
		"DirectoryInfo": taskCtx.DirectoryInfo.Description,
		"Goal":          taskCtx.Goal,
		"Memory":        memory,
		"GitInfo":       gitInfo,
	}

	// Add project summary if available
	if taskCtx.ProjectSummary != nil {
		variables["ProjectInfo"] = taskCtx.ProjectSummary.Info
		variables["SystemContext"] = taskCtx.ProjectSummary.Context
	}

	return p.RenderPrompt("coder", variables)
}

// GetReActObservationPrompt returns the observation phase prompt with variables
func (p *PromptLoader) GetReActObservationPrompt(originalThought, toolResults string) (string, error) {
	variables := map[string]string{
		"original_thought": originalThought,
		"tool_results":     toolResults,
	}
	return p.RenderPrompt("react_observation", variables)
}

// GetUserContextPrompt returns formatted user context
func (p *PromptLoader) GetUserContextPrompt(conversationHistory, currentRequest string) (string, error) {
	variables := map[string]string{
		"conversation_history": conversationHistory,
		"current_request":      currentRequest,
	}
	return p.RenderPrompt("user_context", variables)
}

// loadProjectMemory loads project memory from ALEX.md file if it exists
func (p *PromptLoader) loadProjectMemory(workingDir string) string {
	defaultMemory := "You are a helpful assistant that can help the user with their tasks."

	if workingDir == "" {
		return defaultMemory
	}

	alexFilePath := filepath.Join(workingDir, "ALEX.md")

	// Check if ALEX.md exists
	if _, err := os.Stat(alexFilePath); os.IsNotExist(err) {
		return defaultMemory
	}

	// Try to read ALEX.md content
	content, err := os.ReadFile(alexFilePath)
	if err != nil {
		// If read fails, return default
		return defaultMemory
	}

	// Return file content as memory, or default if empty
	fileContent := strings.TrimSpace(string(content))
	if fileContent == "" {
		return defaultMemory
	}

	return fileContent
}

// loadGitInfo loads current git information from the working directory
func (p *PromptLoader) loadGitInfo(workingDir string) string {
	if workingDir == "" {
		return "Not in a git repository"
	}

	var gitInfo strings.Builder
	
	// Change to the working directory for git commands
	oldWd, _ := os.Getwd()
	defer func() {
		_ = os.Chdir(oldWd)
	}()
	
	if err := os.Chdir(workingDir); err != nil {
		return "Not in a git repository"
	}
	
	// Get current branch
	if cmd := exec.Command("git", "branch", "--show-current"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			branch := strings.TrimSpace(string(output))
			if branch != "" {
				gitInfo.WriteString(fmt.Sprintf("Current branch: %s\n", branch))
			}
		}
	}
	
	// Get main branch
	if cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			mainRef := strings.TrimSpace(string(output))
			if mainRef != "" {
				mainBranch := strings.TrimPrefix(mainRef, "refs/remotes/origin/")
				gitInfo.WriteString(fmt.Sprintf("Main branch: %s\n", mainBranch))
			}
		}
	}
	
	// Get git status
	if cmd := exec.Command("git", "status", "--porcelain"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			status := strings.TrimSpace(string(output))
			if status != "" {
				gitInfo.WriteString(fmt.Sprintf("Status:\n%s\n", status))
			} else {
				gitInfo.WriteString("Status: Clean working directory\n")
			}
		}
	}
	
	// Get recent commits
	if cmd := exec.Command("git", "log", "--oneline", "-5"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			commits := strings.TrimSpace(string(output))
			if commits != "" {
				gitInfo.WriteString(fmt.Sprintf("Recent commits:\n%s\n", commits))
			}
		}
	}
	
	result := gitInfo.String()
	if result == "" {
		return "Not in a git repository"
	}
	
	return strings.TrimSpace(result)
}
