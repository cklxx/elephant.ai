package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"

	"golang.org/x/term"
)

// NativeChatUI implements a chat interface using native terminal control
type NativeChatUI struct {
	container     *Container
	sessionID     string
	messages      []DisplayMessage
	toolFormatter *domain.ToolFormatter
	ctx           context.Context
	startTime     time.Time
	history       []string // Input history
	historyIndex  int      // Current position in history (-1 = not browsing)
}

// DisplayMessage represents a message to display
type DisplayMessage struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// NewNativeChatUI creates a new native chat UI
func NewNativeChatUI(container *Container) *NativeChatUI {
	return &NativeChatUI{
		container:     container,
		messages:      make([]DisplayMessage, 0),
		toolFormatter: domain.NewToolFormatter(),
		ctx:           context.Background(),
		startTime:     time.Now(),
		history:       make([]string, 0),
		historyIndex:  -1,
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

// printPrompt prints an enhanced prompt with context information
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

	// ANSI color codes
	grayStyle := "\033[90m"     // Gray
	resetStyle := "\033[0m"     // Reset
	greenStyle := "\033[32m"    // Green
	dimWhiteStyle := "\033[37m" // White

	// Print top separator line
	separator := strings.Repeat("─", width)
	fmt.Printf("\n%s%s%s\n", grayStyle, separator, resetStyle)

	// Print prompt line with session info
	sessionInfo := fmt.Sprintf("[%s] [msgs:%d] [%s]", sessionShort, messageCount, durationStr)
	fmt.Printf("%s>%s %s%s%s\n", greenStyle, resetStyle, grayStyle, sessionInfo, resetStyle)

	// Print bottom hint line
	hint := "▸▸ Type your request (Ctrl+C to exit)"
	fmt.Printf("%s%s%s\n", dimWhiteStyle, hint, resetStyle)

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

	// Create event listener
	listener := &NativeEventListener{ui: ui}

	// Execute task
	result, err := ui.container.Coordinator.ExecuteTaskWithListener(
		ui.ctx,
		task,
		ui.sessionID,
		listener,
	)
	if err != nil {
		return err
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

// displayMessage displays a single message
func (ui *NativeChatUI) displayMessage(msg DisplayMessage) {
	switch msg.Role {
	case "user":
		fmt.Printf("\n\033[32m➤\033[0m %s\n", msg.Content)
	case "assistant":
		fmt.Printf("\n%s\n", msg.Content)
	case "system":
		fmt.Printf("\033[90m%s\033[0m\n", msg.Content)
	}
}

// printWelcome prints the welcome message
func (ui *NativeChatUI) printWelcome() {
	// Color codes
	cyan := "\033[36m"
	green := "\033[32m"
	yellow := "\033[33m"
	gray := "\033[90m"
	reset := "\033[0m"
	bold := "\033[1m"

	// Get current directory
	cwd, _ := os.Getwd()
	if len(cwd) > 35 {
		cwd = "..." + cwd[len(cwd)-32:]
	}

	// Get git branch if in git repo
	gitBranch := ui.getGitBranch()
	gitInfo := ""
	if gitBranch != "" {
		gitInfo = fmt.Sprintf("%sgit:%s%s%s%s", gray, reset, green, gitBranch, reset)
	}

	// Print colorful welcome banner
	fmt.Printf("%s%s┌─────────────────────────────────────────┐%s\n", cyan, bold, reset)
	fmt.Printf("%s%s│%s  %s%sALEX%s %s- AI Coding Agent%s                %s%s│%s\n",
		cyan, bold, reset,
		green, bold, reset,
		yellow, reset,
		cyan, bold, reset)
	fmt.Printf("%s%s│%s  %s%s%-35s%s  %s%s│%s\n",
		cyan, bold, reset,
		gray, reset, cwd, gray,
		cyan, bold, reset)

	if gitInfo != "" {
		fmt.Printf("%s%s│%s  %s                                    %s%s│%s\n",
			cyan, bold, reset,
			gitInfo,
			cyan, bold, reset)
	}

	fmt.Printf("%s%s│%s  Type your request, /quit to exit      %s%s│%s\n",
		cyan, bold, reset,
		cyan, bold, reset)
	fmt.Printf("%s%s└─────────────────────────────────────────┘%s\n", cyan, bold, reset)
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
type NativeEventListener struct {
	ui *NativeChatUI
}

func (l *NativeEventListener) OnEvent(event domain.AgentEvent) {
	switch e := event.(type) {
	case *domain.IterationStartEvent:
		// Optional: show iteration info
	case *domain.ThinkingEvent:
		// Optional: show thinking indicator
	case *domain.ThinkCompleteEvent:
		// Optional: show thought
	case *domain.ToolCallStartEvent:
		// Show tool call
		formatted := l.ui.toolFormatter.FormatToolCall(e.ToolName, e.Arguments)
		fmt.Printf("\033[90m%s\033[0m\n", formatted)
	case *domain.ToolCallCompleteEvent:
		// Show tool result
		formatted := l.ui.toolFormatter.FormatToolResult(e.ToolName, e.Result, e.Error == nil)
		fmt.Printf("\033[90m%s\033[0m\n", formatted)
	case *domain.IterationCompleteEvent:
		// Optional: show iteration stats
	case *domain.TaskCompleteEvent:
		// Optional: show completion stats
	case *domain.ErrorEvent:
		fmt.Printf("\033[91m✗ Error: %v\033[0m\n", e.Error)
	}
}

// RunNativeChatUI starts the native chat UI
func RunNativeChatUI(container *Container) error {
	ui := NewNativeChatUI(container)
	return ui.Run()
}
