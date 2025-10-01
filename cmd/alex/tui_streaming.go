package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/app"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// StreamingTUIModel is a minimal TUI for streaming agent execution
type StreamingTUIModel struct {
	width   int
	height  int
	content []string
	status  string
	done    bool
	err     error

	// State
	currentIteration int
	totalIterations  int
	activeTools      map[string]ToolInfo
	startTime        time.Time

	// Rendering
	renderer *glamour.TermRenderer
}

func initialStreamingModel() StreamingTUIModel {
	return StreamingTUIModel{
		activeTools: make(map[string]ToolInfo),
		content:     []string{},
		status:      "Initializing...",
		startTime:   time.Now(),
	}
}

func (m StreamingTUIModel) Init() tea.Cmd {
	return nil
}

func (m StreamingTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.done = true
			return m, tea.Quit
		}

	// Agent events
	case app.IterationStartMsg:
		m.currentIteration = msg.Iteration
		m.totalIterations = msg.TotalIters
		m.status = fmt.Sprintf("Iteration %d/%d", msg.Iteration, msg.TotalIters)
		m.addLine(fmt.Sprintf("\n═══ Iteration %d/%d ═══", msg.Iteration, msg.TotalIters))
		return m, nil

	case app.ThinkingMsg:
		m.status = "Thinking..."
		m.addLine("💭 Thinking...")
		return m, nil

	case app.ThinkCompleteMsg:
		if msg.ToolCallCount > 0 {
			m.status = fmt.Sprintf("Executing %d tools...", msg.ToolCallCount)
		} else {
			m.status = "Processing..."
		}
		// Render thought as markdown
		if len(msg.Content) > 0 {
			rendered := m.renderMarkdown(msg.Content, 100)
			m.addLine(fmt.Sprintf("   %s", rendered))
		}
		return m, nil

	case app.ToolCallStartMsg:
		m.activeTools[msg.CallID] = ToolInfo{
			Name:      msg.ToolName,
			StartTime: msg.Timestamp,
		}
		icon := getToolIcon(msg.ToolName)
		argsStr := formatArgs(msg.Arguments)
		toolStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("11")). // Yellow for running
			Bold(true)
		m.addLine(toolStyle.Render(fmt.Sprintf("%s %s(%s)", icon, msg.ToolName, argsStr)))
		return m, nil

	case app.ToolCallCompleteMsg:
		info, exists := m.activeTools[msg.CallID]
		if !exists {
			return m, nil
		}

		duration := msg.Duration
		icon := getToolIcon(info.Name)

		if msg.Error != nil {
			errStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("9")). // Red
				Bold(true)
			m.addLine(errStyle.Render(fmt.Sprintf("   ✗ %s %s failed: %v (%s)", icon, info.Name, msg.Error, duration)))
		} else {
			preview := createToolPreview(info.Name, msg.Result)
			successStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("2")). // Green
				Bold(true)
			m.addLine(successStyle.Render(fmt.Sprintf("   ✓ %s %s: %s (%s)", icon, info.Name, preview, duration)))
		}

		delete(m.activeTools, msg.CallID)
		return m, nil

	case app.IterationCompleteMsg:
		m.addLine(fmt.Sprintf("   Tokens: %d | Tools: %d", msg.TokensUsed, msg.ToolsRun))
		return m, nil

	case app.TaskCompleteMsg:
		m.done = true
		m.status = "Complete!"

		completeStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")). // Bright green
			Bold(true)
		m.addLine(completeStyle.Render(fmt.Sprintf("\n✓ Task completed in %d iterations (%s)", msg.TotalIterations, msg.Duration)))

		statsStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")) // Muted
		m.addLine(statsStyle.Render(fmt.Sprintf("Total tokens: %d", msg.TotalTokens)))

		// Show final answer with markdown rendering
		if msg.FinalAnswer != "" {
			dividerStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")). // Bright blue
				Bold(true)
			m.addLine(dividerStyle.Render("\n━━━ Final Answer ━━━"))

			// Render full answer as markdown
			rendered := m.renderMarkdown(msg.FinalAnswer, -1) // -1 = no truncation
			m.addLine(rendered)
		}

		return m, tea.Quit

	case app.ErrorMsg:
		m.err = msg.Error
		m.addLine(fmt.Sprintf("✗ Error in %s: %v", msg.Phase, msg.Error))
		return m, nil
	}

	return m, nil
}

func (m StreamingTUIModel) View() string {
	if m.done {
		return m.renderFinal()
	}

	return m.renderStreaming()
}

func (m StreamingTUIModel) renderStreaming() string {
	var b strings.Builder

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Padding(0, 1)

	elapsed := time.Since(m.startTime).Round(time.Second)
	header := fmt.Sprintf("ALEX Agent | %s | %s", m.status, elapsed)
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n\n")

	// Content (last N lines to fit screen)
	maxLines := 30
	if m.height > 10 {
		maxLines = m.height - 5
	}

	startIdx := 0
	if len(m.content) > maxLines {
		startIdx = len(m.content) - maxLines
	}

	for i := startIdx; i < len(m.content); i++ {
		b.WriteString(m.content[i])
		b.WriteString("\n")
	}

	// Footer with help
	b.WriteString("\n")
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

	helpText := lipgloss.JoinHorizontal(lipgloss.Left,
		muted.Render("press "),
		key.Render("ctrl+c"),
		muted.Render(" or "),
		key.Render("q"),
		muted.Render(" to quit • "),
		key.Render("↑↓"),
		muted.Render(" to scroll"),
	)
	b.WriteString(helpText)

	return b.String()
}

func (m StreamingTUIModel) renderFinal() string {
	var b strings.Builder

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("10"))

	b.WriteString(headerStyle.Render("✓ Task Complete"))
	b.WriteString("\n\n")

	// Show all content
	for _, line := range m.content {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		b.WriteString("\n")
		b.WriteString(errStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	b.WriteString("\nPress Enter to exit...")

	return b.String()
}

func (m *StreamingTUIModel) addLine(line string) {
	m.content = append(m.content, line)
}

// Helper functions

func formatArgs(args map[string]interface{}) string {
	if len(args) == 0 {
		return ""
	}

	var parts []string
	for k, v := range args {
		valStr := fmt.Sprintf("%v", v)
		if len(valStr) > 40 {
			valStr = valStr[:37] + "..."
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, valStr))
	}

	result := strings.Join(parts, ", ")
	if len(result) > 80 {
		result = result[:77] + "..."
	}
	return result
}

func createToolPreview(toolName, result string) string {
	switch toolName {
	case "file_read":
		lines := strings.Count(result, "\n")
		return fmt.Sprintf("%d lines", lines)

	case "grep", "ripgrep", "code_search":
		matches := strings.Count(result, "\n")
		return fmt.Sprintf("%d matches", matches)

	case "file_write", "file_edit":
		return "written"

	case "bash", "code_execute":
		if len(result) == 0 {
			return "success"
		}
		firstLine := strings.Split(result, "\n")[0]
		if len(firstLine) > 50 {
			firstLine = firstLine[:47] + "..."
		}
		return firstLine

	case "list_files":
		files := strings.Count(result, "\n")
		return fmt.Sprintf("%d files", files)

	case "web_search":
		return "search complete"

	case "web_fetch":
		return "fetched"

	case "think":
		if len(result) > 60 {
			return result[:57] + "..."
		}
		return result

	default:
		if len(result) > 60 {
			return result[:57] + "..."
		}
		return result
	}
}

// renderMarkdown renders markdown content with syntax highlighting
func (m *StreamingTUIModel) renderMarkdown(content string, maxChars int) string {
	// Truncate if needed
	if maxChars > 0 && len(content) > maxChars {
		content = content[:maxChars] + "..."
	}

	// Initialize renderer if needed
	if m.renderer == nil {
		width := m.width
		if width == 0 {
			width = 80
		}

		var err error
		m.renderer, err = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width-4), // Account for padding
		)
		if err != nil {
			// Fallback to plain text if glamour fails
			return content
		}
	}

	// Render markdown
	rendered, err := m.renderer.Render(content)
	if err != nil {
		// Fallback to plain text
		return content
	}

	return strings.TrimSpace(rendered)
}

// RunTaskWithTUI executes a task with streaming TUI
func RunTaskWithTUI(container *Container, task string, sessionID string) error {
	// Create TUI model
	model := initialStreamingModel()

	// Create Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run agent in background goroutine
	go func() {
		ctx := context.Background()
		_, err := container.Coordinator.ExecuteTaskWithTUI(ctx, task, sessionID, p)
		if err != nil {
			p.Send(app.ErrorMsg{
				Timestamp: time.Now(),
				Phase:     "execution",
				Error:     err,
			})
		}
	}()

	// Run TUI (blocks until done)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
