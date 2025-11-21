package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/types"
	"alex/internal/output"
	"alex/internal/tools/builtin"
	id "alex/internal/utils/id"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Style definitions for consistent terminal output
var (
	// Color styles
	styleGray      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleGreen     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleYellow    = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleBoldGreen = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleBoldCyan  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleSystem    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// Combined styles for elements
	promptArrow = styleBoldGreen.Render(">")
	userArrow   = styleGreen.Render("➤")
)

// NativeChatUI implements a chat interface using native terminal control
type NativeChatUI struct {
	container    *Container
	sessionID    string
	messages     []DisplayMessage
	ctx          context.Context
	startTime    time.Time
	history      []string // Input history
	historyIndex int      // Current position in history (-1 = not browsing)
}

// DisplayMessage represents a message to display
type DisplayMessage struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// NewNativeChatUI creates a new native chat UI
func NewNativeChatUI(container *Container) *NativeChatUI {
	// Set core agent output context
	ctx := context.Background()
	coreOutCtx := &types.OutputContext{
		Level:   types.LevelCore,
		AgentID: "core",
		Verbose: container.Runtime.Verbose,
	}
	ctx = types.WithOutputContext(ctx, coreOutCtx)

	return &NativeChatUI{
		container:    container,
		messages:     make([]DisplayMessage, 0),
		ctx:          ctx,
		startTime:    time.Now(),
		history:      make([]string, 0),
		historyIndex: -1,
	}
}

// Run starts the native chat UI
func (ui *NativeChatUI) Run() error {
	// Always use line mode - simpler and more reliable
	return ui.runLineMode()
}

// runLineMode runs in simple line-based mode (fallback)
func (ui *NativeChatUI) runLineMode() error {
	ui.printWelcome()

	scanner := bufio.NewScanner(os.Stdin)
	messageCount := 0

	for {
		// Print enhanced prompt with context
		ui.printPrompt(messageCount)

		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		if input == "/quit" || input == "/exit" {
			fmt.Println("Goodbye!")
			return nil
		}

		// Add newline to separate input from output
		fmt.Println()

		// Add to history
		ui.history = append(ui.history, input)
		ui.historyIndex = -1 // Reset history browsing

		// Execute task (don't display user message separately, it's already in the prompt)
		messageCount++
		if err := ui.executeTask(input); err != nil {
			ui.addMessage("system", fmt.Sprintf("Error: %v", err))
		}
	}

	return scanner.Err()
}

// printPrompt prints an enhanced prompt with context information using Lipgloss
func (ui *NativeChatUI) printPrompt(messageCount int) {
	// Calculate session duration
	duration := time.Since(ui.startTime)
	durationStr := formatDuration(duration)

	// Session ID (shortened)
	sessionShort := ""
	if ui.sessionID != "" {
		parts := strings.Split(ui.sessionID, "-")
		if len(parts) > 0 {
			sessionShort = parts[len(parts)-1]
		}
	} else {
		sessionShort = "new"
	}

	// Get terminal width for separator line
	width := getTerminalWidth()

	// Print top separator line
	separatorLine := strings.Repeat("─", width)
	fmt.Printf("\n%s\n", styleGray.Render(separatorLine))

	// Print prompt line with session info
	sessionInfo := fmt.Sprintf("[%s] [msgs:%d] [%s]", sessionShort, messageCount, durationStr)
	fmt.Printf("%s %s\n", promptArrow, styleGray.Render(sessionInfo))

	// Print bottom hint line
	hint := "▸▸ Type your request (Ctrl+C to exit)"
	fmt.Printf("%s\n", styleGray.Render(hint))

	// Print input indicator
	fmt.Print("  ")
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	} else {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// getTerminalWidth returns the terminal width, defaults to 80 if unable to detect
func getTerminalWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		return 80 // Default fallback
	}
	return width
}

// executeTask runs a task and displays results
func (ui *NativeChatUI) executeTask(task string) error {
	// Create session if needed
	if ui.sessionID == "" {
		session, err := ui.container.Coordinator.GetSession(ui.ctx, "")
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}
		ui.sessionID = session.ID
	}

	// Create event listener with unified renderer
	listener := newNativeEventListener(ui)

	taskCtx := id.WithSessionID(ui.ctx, ui.sessionID)
	taskCtx = id.WithTaskID(taskCtx, id.NewTaskID())

	// Execute task using coordinator's method
	result, err := ui.container.Coordinator.ExecuteTask(taskCtx, task, ui.sessionID, listener)
	if err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}

	// Display final answer
	if result.Answer != "" {
		ui.addMessage("assistant", result.Answer)
	}

	return nil
}

// addMessage adds a message and displays it
func (ui *NativeChatUI) addMessage(role, content string) {
	msg := DisplayMessage{
		Role:    role,
		Content: content,
	}
	ui.messages = append(ui.messages, msg)
	ui.displayMessage(msg)
}

// displayMessage displays a single message using Lipgloss styles
func (ui *NativeChatUI) displayMessage(msg DisplayMessage) {
	switch msg.Role {
	case "user":
		fmt.Printf("\n%s %s\n", userArrow, msg.Content)
	case "assistant":
		fmt.Printf("\n%s\n", msg.Content)
	case "system":
		fmt.Printf("%s\n", styleSystem.Render(msg.Content))
	}
}

// printWelcome prints the welcome message using Lipgloss for styling
func (ui *NativeChatUI) printWelcome() {
	// Get current directory
	cwd, _ := os.Getwd()
	if len(cwd) > 35 {
		cwd = "..." + cwd[len(cwd)-32:]
	}

	// Get git branch if in git repo
	gitBranch := ui.getGitBranch()

	// Build welcome message with Lipgloss styles
	topBorder := styleBoldCyan.Render("┌─────────────────────────────────────────┐")
	botBorder := styleBoldCyan.Render("└─────────────────────────────────────────┘")
	verticalBar := styleBoldCyan.Render("│")

	// Title line
	titleContent := fmt.Sprintf("  %s %s",
		styleBold.Render(styleGreen.Render("Spinner")),
		styleYellow.Render("- Fragment-to-Fabric Agent"))
	titleLine := fmt.Sprintf("%s%s%-37s%s",
		verticalBar,
		titleContent,
		"",
		verticalBar)

	// Path line
	pathLine := fmt.Sprintf("%s  %s%-35s  %s",
		verticalBar,
		styleGray.Render(""),
		cwd,
		verticalBar)

	// Git branch line (if available)
	var gitLine string
	if gitBranch != "" {
		gitInfo := fmt.Sprintf("git:%s", styleGreen.Render(gitBranch))
		gitLine = fmt.Sprintf("%s  %s%-39s%s\n",
			verticalBar,
			gitInfo,
			"",
			verticalBar)
	}

	// Instructions line
	instrLine := fmt.Sprintf("%s  Type your request, /quit to exit      %s",
		verticalBar,
		verticalBar)

	// Print the welcome banner
	fmt.Printf("%s\n", topBorder)
	fmt.Printf("%s\n", titleLine)
	fmt.Printf("%s\n", pathLine)
	if gitLine != "" {
		fmt.Printf("%s", gitLine)
	}
	fmt.Printf("%s\n", instrLine)
	fmt.Printf("%s\n", botBorder)
}

// getGitBranch returns the current git branch name, or empty if not in a git repo
func (ui *NativeChatUI) getGitBranch() string {
	// Try to read .git/HEAD
	headFile := ".git/HEAD"
	content, err := os.ReadFile(headFile)
	if err != nil {
		return ""
	}

	// Parse branch name from "ref: refs/heads/branch-name"
	line := strings.TrimSpace(string(content))
	if strings.HasPrefix(line, "ref: refs/heads/") {
		branch := strings.TrimPrefix(line, "ref: refs/heads/")
		if len(branch) > 20 {
			branch = branch[:17] + "..."
		}
		return branch
	}

	return ""
}

// NativeEventListener implements domain.EventListener for native UI
// It uses the unified CLIRenderer for consistent output
type NativeEventListener struct {
	ui          *NativeChatUI
	renderer    *output.CLIRenderer
	activeTools map[string]ToolInfo
	subagents   *SubagentDisplay
}

func newNativeEventListener(ui *NativeChatUI) *NativeEventListener {
	return &NativeEventListener{
		ui:          ui,
		renderer:    output.NewCLIRenderer(ui.container.Runtime.Verbose),
		activeTools: make(map[string]ToolInfo),
		subagents:   NewSubagentDisplay(),
	}
}

func (l *NativeEventListener) OnEvent(event ports.AgentEvent) {
	if subtaskEvent, ok := event.(*builtin.SubtaskEvent); ok {
		lines := l.subagents.Handle(subtaskEvent)
		for _, line := range lines {
			fmt.Print(line)
		}
		return
	}

	// Get output context from UI
	outCtx := types.GetOutputContext(l.ui.ctx)

	switch e := event.(type) {
	case *domain.IterationStartEvent:
	// Optional: show iteration info
	case *domain.ThinkingEvent:
		// Optional: show thinking indicator
	case *domain.ThinkCompleteEvent:
		// Optional: show thought
	case *domain.ToolCallStartEvent:
		// Store tool info for duration calculation
		l.activeTools[e.CallID] = ToolInfo{
			Name:      e.ToolName,
			StartTime: e.Timestamp(),
		}

		// Show tool call using unified renderer
		outCtx.Category = output.CategorizeToolName(e.ToolName)
		rendered := l.renderer.RenderToolCallStart(outCtx, e.ToolName, e.Arguments)
		if rendered != "" {
			fmt.Print(rendered)
		}
	case *domain.ToolCallCompleteEvent:
		// Show tool result using unified renderer
		info, exists := l.activeTools[e.CallID]
		if exists {
			outCtx.Category = output.CategorizeToolName(info.Name)
			duration := time.Since(info.StartTime)
			rendered := l.renderer.RenderToolCallComplete(outCtx, info.Name, e.Result, e.Error, duration)
			if rendered != "" {
				fmt.Print(rendered)
			}
			delete(l.activeTools, e.CallID)
		}
	case *domain.IterationCompleteEvent:
		// Optional: show iteration stats
	case *domain.TaskCompleteEvent:
		// Optional: show completion stats
	case *domain.ErrorEvent:
		fmt.Printf("%s\n", styleError.Render(fmt.Sprintf("✗ Error: %v", e.Error)))
	}
}

// RunNativeChatUI starts the native chat UI
func RunNativeChatUI(container *Container) error {
	ui := NewNativeChatUI(container)
	return ui.Run()
}
