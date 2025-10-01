package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"

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

	// Print task header
	fmt.Printf("Executing: %s\n\n", task)

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

func (h *StreamingOutputHandler) onIterationStart(event *domain.IterationStartEvent) {
	// Silent - don't print iteration headers in simple mode
}

func (h *StreamingOutputHandler) onThinking(event *domain.ThinkingEvent) {
	// Silent - analysis is shown in think complete
}

func (h *StreamingOutputHandler) onThinkComplete(event *domain.ThinkCompleteEvent) {
	if len(event.Content) == 0 {
		return
	}

	// Print analysis (formatted)
	analysis := strings.TrimSpace(event.Content)
	if len(analysis) > 100 {
		analysis = analysis[:97] + "..."
	}

	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // Muted
	fmt.Printf("Analysis: %s\n\n", style.Render(analysis))
}

func (h *StreamingOutputHandler) onToolCallStart(event *domain.ToolCallStartEvent) {
	h.activeTools[event.CallID] = ToolInfo{
		Name:      event.ToolName,
		StartTime: event.Timestamp(),
	}

	// Print tool indicator
	icon := getToolIcon(event.ToolName)
	args := formatArgsInline(event.Arguments)

	// Simple format: ⏺ tool_name(args)
	fmt.Printf("⏺ %s%s(%s)\n", icon, event.ToolName, args)
}

func (h *StreamingOutputHandler) onToolCallComplete(event *domain.ToolCallCompleteEvent) {
	info, exists := h.activeTools[event.CallID]
	if !exists {
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
	// Print completion line
	successStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10")). // Bright green
		Bold(true)
	fmt.Printf("\n%s\n", successStyle.Render(fmt.Sprintf("✓ Task completed in %d iterations", result.Iterations)))

	// Print token usage
	statsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")) // Muted
	fmt.Printf("%s\n", statsStyle.Render(fmt.Sprintf("Tokens used: %d", result.TokensUsed)))

	// Print answer
	if result.Answer != "" {
		fmt.Println("\nAnswer:")
		fmt.Println(result.Answer)
	}
}

// printSmartToolOutput intelligently displays tool output based on tool type and user needs
func (h *StreamingOutputHandler) printSmartToolOutput(toolName, result string) {
	switch toolName {
	case "code_execute":
		// Code execution: ALWAYS show full code and output
		h.printFullOutput("Execution Result", result, lipgloss.Color("10"))

	case "todo_read", "todo_update":
		// Todo tools: ALWAYS show full task list
		h.printFullOutput("Task List", result, lipgloss.Color("12"))

	case "bash":
		// Bash: show summary + important lines
		if len(result) <= 300 {
			h.printFullOutput("Command Output", result, lipgloss.Color("14"))
		} else {
			// Show first 10 and last 10 lines
			lines := strings.Split(result, "\n")
			preview := fmt.Sprintf("Output: %d lines total", len(lines))
			fmt.Printf("  → %s\n", preview)

			if !h.verbose {
				fmt.Printf("    %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("(Use ALEX_VERBOSE=1 to see full output)"))
			} else {
				h.printFullOutput("Full Output", result, lipgloss.Color("14"))
			}
		}

	case "grep", "ripgrep", "code_search", "find":
		// Search tools: show count + preview
		lines := strings.Split(strings.TrimSpace(result), "\n")
		matchCount := len(lines)
		if result == "" {
			matchCount = 0
		}

		fmt.Printf("  → %d matches\n", matchCount)

		// Show first few matches
		if matchCount > 0 && matchCount <= 10 {
			for i, line := range lines {
				if i >= 5 {
					break
				}
				fmt.Printf("    %s\n", line)
			}
			if matchCount > 5 {
				fmt.Printf("    %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("... and %d more", matchCount-5)))
			}
		} else if matchCount > 10 {
			for i := 0; i < 3; i++ {
				fmt.Printf("    %s\n", lines[i])
			}
			fmt.Printf("    %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("... and %d more (use ALEX_VERBOSE=1 for full output)", matchCount-3)))
		}

	case "file_read":
		// File read: just show line count (content is for LLM)
		lines := strings.Count(result, "\n")
		fmt.Printf("  → %d lines read\n", lines)

	case "file_write":
		// File write: simple confirmation with path if available
		if strings.Contains(result, "written") {
			fmt.Printf("  → %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ file written"))
		} else {
			fmt.Printf("  → %s\n", result)
		}

	case "file_edit":
		// File edit: show what changed
		if strings.Contains(result, "Success") {
			fmt.Printf("  → %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ file edited"))
		} else {
			fmt.Printf("  → %s\n", result)
		}

	case "list_files":
		// List files: show count + first few
		lines := strings.Split(strings.TrimSpace(result), "\n")
		fileCount := len(lines)
		if result == "" {
			fileCount = 0
		}

		fmt.Printf("  → %d files/directories\n", fileCount)
		if fileCount > 0 && fileCount <= 10 {
			for _, line := range lines {
				fmt.Printf("    %s\n", line)
			}
		} else if fileCount > 10 {
			for i := 0; i < 5; i++ {
				fmt.Printf("    %s\n", lines[i])
			}
			fmt.Printf("    %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render(fmt.Sprintf("... and %d more", fileCount-5)))
		}

	case "web_search":
		// Web search: show result count
		fmt.Printf("  → %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ search completed"))

	case "web_fetch":
		// Web fetch: show fetched status
		fmt.Printf("  → %s\n", lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Render("✓ content fetched"))

	case "think":
		// Think: show summary
		summary := result
		if len(summary) > 100 {
			summary = summary[:97] + "..."
		}
		fmt.Printf("  → %s\n", summary)

	case "git_commit", "git_pr", "git_history":
		// Git tools: show full output (usually important)
		h.printFullOutput("Git Result", result, lipgloss.Color("13"))

	default:
		// Default: show preview
		preview := result
		if len(preview) > 80 {
			preview = preview[:77] + "..."
		}
		fmt.Printf("  → %s\n", preview)

		// Show full output in verbose mode
		if h.verbose && len(result) > 80 {
			h.printFullOutput("Full Output", result, lipgloss.Color("8"))
		}
	}
}

// printFullOutput prints the complete tool output with formatting
func (h *StreamingOutputHandler) printFullOutput(label, content string, color lipgloss.Color) {
	if content == "" {
		return
	}

	// Print content with indent
	lines := strings.Split(strings.TrimSpace(content), "\n")
	for _, line := range lines {
		fmt.Printf("    %s\n", line)
	}
}

// Helper functions

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

// getToolIcon returns an emoji icon for a tool
func getToolIcon(toolName string) string {
	icons := map[string]string{
		"bash":         "🖥️  ",
		"code_execute": "▶️  ",
		"file_read":    "📖 ",
		"file_write":   "✏️  ",
		"file_edit":    "📝 ",
		"list_files":   "📁 ",
		"grep":         "🔍 ",
		"ripgrep":      "🔍 ",
		"find":         "🔎 ",
		"web_search":   "🌐 ",
		"web_fetch":    "🌐 ",
		"think":        "💭 ",
		"todo_read":    "📋 ",
		"todo_update":  "✅ ",
	}
	if icon, ok := icons[toolName]; ok {
		return icon
	}
	return "🔧 "
}
