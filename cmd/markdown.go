package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

// MarkdownRenderer handles rendering markdown content in the terminal
type MarkdownRenderer struct {
	renderer *glamour.TermRenderer
}

// NewMarkdownRenderer creates a new markdown renderer with Claude Code styling
func NewMarkdownRenderer() (*MarkdownRenderer, error) {
	return NewMarkdownRendererWithStyle(false)
}

// NewMarkdownRendererWithStyle creates a markdown renderer with optional plain text mode
func NewMarkdownRendererWithStyle(plainText bool) (*MarkdownRenderer, error) {
	// Get terminal width for dynamic word wrapping
	termWidth := 80 // Default fallback
	if width, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && width > 0 {
		termWidth = width - 4 // Leave some margin
		if termWidth > 120 {
			termWidth = 120 // Max width for readability
		}
	}

	var style glamour.TermRendererOption
	if plainText {
		// Use plain text style without colors for TUI compatibility
		style = glamour.WithStandardStyle("notty")
	} else {
		// Use colored style for regular CLI
		style = glamour.WithStandardStyle("dark")
	}

	renderer, err := glamour.NewTermRenderer(
		style,
		glamour.WithWordWrap(termWidth),
		glamour.WithEmoji(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create markdown renderer: %w", err)
	}

	return &MarkdownRenderer{
		renderer: renderer,
	}, nil
}

// Render renders markdown content to styled terminal output
func (mr *MarkdownRenderer) Render(content string) (string, error) {
	if content == "" {
		return "", nil
	}

	// Render the markdown
	rendered, err := mr.renderer.Render(content)
	if err != nil {
		return "", fmt.Errorf("failed to render markdown: %w", err)
	}

	return rendered, nil
}

// RenderAndPrint renders and immediately prints markdown content
func (mr *MarkdownRenderer) RenderAndPrint(content string) error {
	rendered, err := mr.Render(content)
	if err != nil {
		return err
	}

	fmt.Print(rendered)
	return nil
}

// IsMarkdown detects if content contains markdown formatting
func IsMarkdown(content string) bool {
	// More precise heuristics to detect markdown
	content = strings.TrimSpace(content)
	if len(content) == 0 {
		return false
	}

	// Strong indicators of markdown
	strongIndicators := []string{
		"# ",      // Headers
		"## ",     // Headers
		"### ",    // Headers
		"#### ",   // Headers
		"##### ",  // Headers
		"###### ", // Headers
		"```",     // Code blocks
		"- ",      // Lists (but check for context)
		"* ",      // Lists (but check for context)
		"1. ",     // Numbered lists
		"2. ",     // Numbered lists
		"![",      // Images
		"|---|",   // Table separators
	}

	for _, indicator := range strongIndicators {
		if strings.Contains(content, indicator) {
			return true
		}
	}

	// Check for link patterns [text](url)
	if strings.Contains(content, "[") && strings.Contains(content, "](") {
		return true
	}

	// Check for bold/italic patterns (but be more careful)
	if strings.Contains(content, "**") && strings.Count(content, "**") >= 2 {
		return true
	}

	// Check for inline code (but require complete backticks)
	if strings.Contains(content, "`") && strings.Count(content, "`") >= 2 {
		return true
	}

	// Check for blockquotes at line start
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "> ") {
			return true
		}
	}

	return false
}

// ShouldRenderAsMarkdown is a more conservative check for whether content should be rendered as markdown
func ShouldRenderAsMarkdown(content string) bool {
	// Don't render very short content as markdown
	if len(strings.TrimSpace(content)) < 10 {
		return false
	}

	// Don't render single words or simple phrases
	if !strings.Contains(content, "\n") && len(strings.Fields(content)) < 3 {
		return false
	}

	return IsMarkdown(content)
}

// RenderIfMarkdown renders content as markdown if it contains markdown, otherwise returns as-is
func (mr *MarkdownRenderer) RenderIfMarkdown(content string) string {
	if ShouldRenderAsMarkdown(content) {
		rendered, err := mr.Render(content)
		if err != nil {
			// Fall back to original content if rendering fails
			return content
		}
		return rendered
	}
	return content
}

// Global markdown renderer instances
var globalMarkdownRenderer *MarkdownRenderer

// RenderMarkdown is a convenience function that uses the global renderer
func RenderMarkdown(content string) string {
	if globalMarkdownRenderer == nil {
		// If renderer not initialized, return content as-is
		return content
	}
	return globalMarkdownRenderer.RenderIfMarkdown(content)
}

// PrintMarkdown is a convenience function that renders and prints markdown
func PrintMarkdown(content string) {
	if globalMarkdownRenderer == nil {
		fmt.Print(content)
		return
	}

	rendered := globalMarkdownRenderer.RenderIfMarkdown(content)
	fmt.Print(rendered)
}
