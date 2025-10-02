package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ToolInfo stores information about an active tool call
type ToolInfo struct {
	Name      string
	StartTime time.Time
}

// StreamingOutputHandler handles streaming output to terminal (not full-screen TUI)
type StreamingOutputHandler struct {
	container *Container

	// State
	activeTools map[string]ToolInfo
	verbose     bool
}

func NewStreamingOutputHandler(container *Container, verbose bool) *StreamingOutputHandler {
	return &StreamingOutputHandler{
		container:   container,
		activeTools: make(map[string]ToolInfo),
		verbose:     verbose,
	}
}

// RunTaskWithStreamOutput executes a task with inline streaming output
func RunTaskWithStreamOutput(container *Container, task string, sessionID string) error {
	handler := NewStreamingOutputHandler(container, isVerbose())

	// Start execution with stream handler
	ctx := context.Background()
	result, err := executeTaskWithStreamHandler(ctx, container, task, sessionID, handler)

	if err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}

	// Print completion summary
	handler.printCompletion(result)

	return nil
}

// executeTaskWithStreamHandler runs task with streaming event handler
func executeTaskWithStreamHandler(
	ctx context.Context,
	container *Container,
	task string,
	sessionID string,
	handler *StreamingOutputHandler,
) (*domain.TaskResult, error) {
	// Create StreamEventBridge that implements domain.EventListener
	bridge := NewStreamEventBridge(handler)

	// Execute task (we'll add ExecuteTaskWithListener method to Coordinator)
	return container.Coordinator.ExecuteTaskWithListener(ctx, task, sessionID, bridge)
}

// StreamEventBridge converts domain events to stream output
type StreamEventBridge struct {
	handler *StreamingOutputHandler
}

func NewStreamEventBridge(handler *StreamingOutputHandler) *StreamEventBridge {
	return &StreamEventBridge{handler: handler}
}

// OnEvent implements domain.EventListener
func (b *StreamEventBridge) OnEvent(event domain.AgentEvent) {
	switch e := event.(type) {
	case *domain.TaskAnalysisEvent:
		b.handler.onTaskAnalysis(e)
	case *domain.IterationStartEvent:
		b.handler.onIterationStart(e)
	case *domain.ThinkingEvent:
		b.handler.onThinking(e)
	case *domain.ThinkCompleteEvent:
		b.handler.onThinkComplete(e)
	case *domain.ToolCallStartEvent:
		b.handler.onToolCallStart(e)
	case *domain.ToolCallCompleteEvent:
		b.handler.onToolCallComplete(e)
	case *domain.ErrorEvent:
		b.handler.onError(e)
	}
}

// Event handlers

func (h *StreamingOutputHandler) onTaskAnalysis(event *domain.TaskAnalysisEvent) {
	// Print task analysis with purple gradient
	text := fmt.Sprintf("👾 %s", event.ActionName)
	gradientText := renderPurpleGradient(text)
	fmt.Printf("\n%s\n\n", gradientText)
}

// renderPurpleGradient creates a purple gradient effect for text
func renderPurpleGradient(text string) string {
	// Purple gradient colors: from light purple to deep purple
	colors := []string{
		"#E0B0FF", // Light purple
		"#D8A7F5", // Mauve
		"#C78EEB", // Medium purple
		"#B678E0", // Purple
		"#9F5FD6", // Deep purple
		"#8B47CC", // Royal purple
	}

	runes := []rune(text)
	if len(runes) == 0 {
		return text
	}

	var result strings.Builder

	// Calculate color step
	colorsLen := len(colors)
	runesLen := len(runes)

	for i, r := range runes {
		// Map character position to color index
		colorIdx := (i * (colorsLen - 1)) / max(runesLen-1, 1)
		if colorIdx >= colorsLen {
			colorIdx = colorsLen - 1
		}

		color := colors[colorIdx]
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		result.WriteString(style.Render(string(r)))
	}

	return result.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (h *StreamingOutputHandler) onIterationStart(event *domain.IterationStartEvent) {
	// Silent - don't print iteration headers in simple mode
}

func (h *StreamingOutputHandler) onThinking(event *domain.ThinkingEvent) {
	// Silent - analysis is shown in think complete
}

func (h *StreamingOutputHandler) onThinkComplete(event *domain.ThinkCompleteEvent) {
	// Silent - don't print analysis output
	// Analysis is internal reasoning, not user-facing output
}

func (h *StreamingOutputHandler) onToolCallStart(event *domain.ToolCallStartEvent) {
	h.activeTools[event.CallID] = ToolInfo{
		Name:      event.ToolName,
		StartTime: event.Timestamp(),
	}

	// Only print in verbose mode
	if !h.verbose {
		return
	}

	// Print tool indicator with bold bright green dot
	args := formatArgsInline(event.Arguments)

	// Format: ● tool_name(args) - bold bright green dot, bold tool name
	dotStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")). // Bright green (#00ff00)
		Bold(true)                             // Bold dot
	toolNameStyle := lipgloss.NewStyle().Bold(true) // Bold tool name

	if args != "" {
		fmt.Printf("%s %s(%s)\n", dotStyle.Render("●"), toolNameStyle.Render(event.ToolName), args)
	} else {
		fmt.Printf("%s %s\n", dotStyle.Render("●"), toolNameStyle.Render(event.ToolName))
	}
}

func (h *StreamingOutputHandler) onToolCallComplete(event *domain.ToolCallCompleteEvent) {
	info, exists := h.activeTools[event.CallID]
	if !exists {
		return
	}

	// Only print in verbose mode
	if !h.verbose {
		delete(h.activeTools, event.CallID)
		return
	}

	if event.Error != nil {
		// Print error
		errStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")) // Red
		fmt.Printf("  %s\n", errStyle.Render(fmt.Sprintf("✗ %s failed: %v", info.Name, event.Error)))
	} else {
		// Smart display based on tool type
		h.printSmartToolOutput(info.Name, event.Result)
	}

	delete(h.activeTools, event.CallID)
}

func (h *StreamingOutputHandler) onError(event *domain.ErrorEvent) {
	errStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		Bold(true)
	fmt.Printf("\n%s\n", errStyle.Render(fmt.Sprintf("✗ Error in %s: %v", event.Phase, event.Error)))
}

func (h *StreamingOutputHandler) printCompletion(result *domain.TaskResult) {
	// Print compact completion summary
	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")) // Green

	if h.verbose {
		// Verbose mode: show detailed stats
		fmt.Printf("\n%s\n", statsStyle.Render(fmt.Sprintf("✓ Task completed in %d iterations", result.Iterations)))
		fmt.Printf("%s\n\n", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("Tokens used: %d", result.TokensUsed)))
	} else {
		// Concise mode: one-line summary
		fmt.Printf("\n%s\n\n", statsStyle.Render(fmt.Sprintf("✓ Done | %d iterations | %d tokens", result.Iterations, result.TokensUsed)))
	}

	// Print answer with markdown rendering
	if result.Answer != "" {
		rendered := h.renderMarkdown(result.Answer)
		fmt.Print(rendered)
		if !strings.HasSuffix(rendered, "\n") {
			fmt.Println()
		}
	}
}

// renderMarkdown renders markdown content with syntax highlighting
func (h *StreamingOutputHandler) renderMarkdown(content string) string {
	// Try dark style first, then fallback to ASCII, then plain text
	styles := []string{"dark", "ascii"}

	for _, style := range styles {
		renderer, err := glamour.NewTermRenderer(
			glamour.WithStylePath(style),
			glamour.WithWordWrap(100), // 100 char width for CLI output
		)
		if err != nil {
			continue
		}

		// Render markdown
		rendered, err := renderer.Render(content)
		if err != nil {
			continue
		}

		return strings.TrimSpace(rendered)
	}

	// Ultimate fallback to plain text
	return content
}

// printSmartToolOutput intelligently displays tool output based on tool type and user needs
func (h *StreamingOutputHandler) printSmartToolOutput(toolName, result string) {
	// Filter out system-reminder tags (for model only, not user display)
	result = filterSystemReminders(result)

	category := getToolCategory(toolName)

	switch toolName {
	case "code_execute":
		// Code execution: ALWAYS show full code and output
		h.printCategorizedOutput(category, "execution result", result, lipgloss.Color("10"))

	case "todo_read", "todo_update":
		// Todo tools: ALWAYS show full task list
		h.printCategorizedOutput(category, "task list", result, lipgloss.Color("12"))

	case "bash":
		// Bash: show summary + important lines
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		if len(result) <= 300 {
			h.printGrayOutput(result)
		} else {
			lines := strings.Split(result, "\n")
			preview := fmt.Sprintf("Output: %d lines total", len(lines))
			fmt.Printf("  → %s\n", grayStyle.Render(preview))

			if !h.verbose {
				fmt.Printf("    %s\n", grayStyle.Render("(Use ALEX_VERBOSE=1 to see full output)"))
			} else {
				h.printGrayOutput(result)
			}
		}

	case "grep", "ripgrep", "code_search", "find":
		// Search tools: show count + preview
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		lines := strings.Split(strings.TrimSpace(result), "\n")
		matchCount := len(lines)
		if result == "" {
			matchCount = 0
		}

		fmt.Printf("  → %s\n", grayStyle.Render(fmt.Sprintf("%d matches", matchCount)))

		// Show first few matches in gray
		if matchCount > 0 && matchCount <= 10 {
			for i, line := range lines {
				if i >= 5 {
					break
				}
				fmt.Printf("    %s\n", grayStyle.Render(line))
			}
			if matchCount > 5 {
				fmt.Printf("    %s\n", grayStyle.Render(fmt.Sprintf("... and %d more", matchCount-5)))
			}
		} else if matchCount > 10 {
			for i := 0; i < 3; i++ {
				fmt.Printf("    %s\n", grayStyle.Render(lines[i]))
			}
			fmt.Printf("    %s\n", grayStyle.Render(fmt.Sprintf("... and %d more (use ALEX_VERBOSE=1 for full output)", matchCount-3)))
		}

	case "file_read":
		// File read: just show line count in gray
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		lines := strings.Count(result, "\n")
		fmt.Printf("  → %s\n", grayStyle.Render(fmt.Sprintf("%d lines read", lines)))

	case "file_write":
		// File write: simple confirmation in gray
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		if strings.Contains(result, "written") {
			fmt.Printf("  → %s\n", grayStyle.Render("✓ file written"))
		} else {
			fmt.Printf("  → %s\n", grayStyle.Render(result))
		}

	case "file_edit":
		// File edit: show what changed in gray
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		if strings.Contains(result, "Success") {
			fmt.Printf("  → %s\n", grayStyle.Render("✓ file edited"))
		} else {
			fmt.Printf("  → %s\n", grayStyle.Render(result))
		}

	case "list_files":
		// List files: show count + first few in gray
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		lines := strings.Split(strings.TrimSpace(result), "\n")
		fileCount := len(lines)
		if result == "" {
			fileCount = 0
		}

		fmt.Printf("  → %s\n", grayStyle.Render(fmt.Sprintf("%d files/directories", fileCount)))
		if fileCount > 0 && fileCount <= 10 {
			for _, line := range lines {
				fmt.Printf("    %s\n", grayStyle.Render(line))
			}
		} else if fileCount > 10 {
			for i := 0; i < 5; i++ {
				fmt.Printf("    %s\n", grayStyle.Render(lines[i]))
			}
			fmt.Printf("    %s\n", grayStyle.Render(fmt.Sprintf("... and %d more", fileCount-5)))
		}

	case "web_search":
		// Web search: show result count in gray
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		fmt.Printf("  → %s\n", grayStyle.Render("✓ search completed"))

	case "web_fetch":
		// Web fetch: show fetched status in gray
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		fmt.Printf("  → %s\n", grayStyle.Render("✓ content fetched"))

	case "think":
		// Think: show summary in gray
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		summary := result
		if len(summary) > 100 {
			summary = summary[:97] + "..."
		}
		fmt.Printf("  → %s\n", grayStyle.Render(summary))

	case "git_commit", "git_pr", "git_history":
		// Git tools: show full output in gray
		h.printGrayOutput(result)

	default:
		// Default: show preview in gray
		grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		preview := result
		if len(preview) > 80 {
			preview = preview[:77] + "..."
		}
		fmt.Printf("  → %s\n", grayStyle.Render(preview))

		// Show full output in verbose mode
		if h.verbose && len(result) > 80 {
			h.printGrayOutput(result)
		}
	}
}

// printGrayOutput prints tool output in gray color
func (h *StreamingOutputHandler) printGrayOutput(content string) {
	if content == "" {
		return
	}

	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		fmt.Printf("  → %s\n", grayStyle.Render(line))
	}
}

// printCategorizedOutput prints tool output with category-specific formatting
func (h *StreamingOutputHandler) printCategorizedOutput(category, label, content string, color lipgloss.Color) {
	if content == "" {
		return
	}

	// Category-specific icons and colors
	categoryStyle := map[string]lipgloss.Style{
		"execution": lipgloss.NewStyle().Foreground(lipgloss.Color("10")), // Green
		"task":      lipgloss.NewStyle().Foreground(lipgloss.Color("12")), // Blue
		"file":      lipgloss.NewStyle().Foreground(lipgloss.Color("14")), // Cyan
		"search":    lipgloss.NewStyle().Foreground(lipgloss.Color("11")), // Yellow
		"web":       lipgloss.NewStyle().Foreground(lipgloss.Color("13")), // Magenta
		"shell":     lipgloss.NewStyle().Foreground(lipgloss.Color("14")), // Cyan
		"reasoning": lipgloss.NewStyle().Foreground(lipgloss.Color("8")),  // Gray
		"other":     lipgloss.NewStyle().Foreground(lipgloss.Color("15")), // White
	}

	style, ok := categoryStyle[category]
	if !ok {
		style = categoryStyle["other"]
	}

	// Print content with category-specific styling
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for i, line := range lines {
		if i == 0 {
			fmt.Printf("  → %s\n", style.Render(line))
		} else {
			fmt.Printf("    %s\n", line)
		}
	}
}

// Shared helper functions (used by both stream_output and tui_streaming)

// filterSystemReminders removes system-reminder tags from content (for display only)
func filterSystemReminders(content string) string {
	// Remove <system-reminder>...</system-reminder> tags
	lines := strings.Split(content, "\n")
	var filtered []string
	inReminder := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<system-reminder>") {
			inReminder = true
			// Skip lines that start with system-reminder
			if strings.HasSuffix(trimmed, "</system-reminder>") {
				inReminder = false
			}
			continue
		}
		if strings.HasSuffix(trimmed, "</system-reminder>") {
			inReminder = false
			continue
		}
		if !inReminder {
			filtered = append(filtered, line)
		}
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}

func formatArgsInline(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}

	var parts []string
	for k, v := range args {
		valStr := fmt.Sprintf("%v", v)
		// Truncate long values
		if len(valStr) > 60 {
			valStr = valStr[:57] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, valStr))
	}

	result := strings.Join(parts, ", ")
	// Truncate if too long
	if len(result) > 80 {
		result = result[:77] + "..."
	}
	return result
}

func isVerbose() bool {
	// Check ALEX_VERBOSE env var
	verbose := os.Getenv("ALEX_VERBOSE")
	if verbose == "" {
		verbose = "false"
	}
	return verbose == "1" || verbose == "true" || verbose == "yes"
}

// getToolCategory returns the category of a tool for smart display
func getToolCategory(toolName string) string {
	categories := map[string]string{
		"bash":         "shell",
		"code_execute": "execution",
		"file_read":    "file",
		"file_write":   "file",
		"file_edit":    "file",
		"list_files":   "file",
		"grep":         "search",
		"ripgrep":      "search",
		"find":         "search",
		"web_search":   "web",
		"web_fetch":    "web",
		"think":        "reasoning",
		"todo_read":    "task",
		"todo_update":  "task",
	}
	if category, ok := categories[toolName]; ok {
		return category
	}
	return "other"
}
