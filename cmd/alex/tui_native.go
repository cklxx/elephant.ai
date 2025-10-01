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

// readLine reads a line of input in raw mode
func (ui *NativeChatUI) readLine() (string, error) {
	var line strings.Builder
	buf := make([]byte, 1)

	for {
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue
		}

		b := buf[0]

		// Handle special keys
		switch b {
		case 3: // Ctrl+C
			fmt.Println()
			return "/quit", nil
		case 13, 10: // Enter
			fmt.Println()
			return line.String(), nil
		case 127, 8: // Backspace
			if line.Len() > 0 {
				// Remove last character
				str := line.String()
				line.Reset()
				line.WriteString(str[:len(str)-1])
				// Clear character on screen
				fmt.Print("\b \b")
			}
		default:
			if b >= 32 && b < 127 { // Printable characters
				line.WriteByte(b)
				fmt.Printf("%c", b)
			}
		}
	}
}

// clearScreen clears the terminal screen
func (ui *NativeChatUI) clearScreen() {
	fmt.Print("\033[2J\033[H")
}

// printWelcome prints the welcome message
func (ui *NativeChatUI) printWelcome() {
	// Color codes
	cyan := "\033[36m"
	green := "\033[32m"
	yellow := "\033[33m"
	reset := "\033[0m"
	bold := "\033[1m"

	// Print colorful welcome banner
	fmt.Printf("%s%s┌─────────────────────────────────────────┐%s\n", cyan, bold, reset)
	fmt.Printf("%s%s│%s  %s%sALEX%s %s- AI Coding Agent%s                %s%s│%s\n",
		cyan, bold, reset,
		green, bold, reset,
		yellow, reset,
		cyan, bold, reset)
	fmt.Printf("%s%s│%s  Type your request, /quit to exit      %s%s│%s\n",
		cyan, bold, reset,
		cyan, bold, reset)
	fmt.Printf("%s%s└─────────────────────────────────────────┘%s\n", cyan, bold, reset)
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
