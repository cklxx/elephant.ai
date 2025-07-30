package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"alex/internal/agent"
	"alex/internal/config"
	"alex/internal/context/message"
)

// formatToolOutput formats tool output with proper multi-line alignment for TUI mode
func formatToolOutput(title, content string) string {
	// For TUI mode, keep content as raw text to avoid conflicts with TUI styling
	// The TUI styling system will handle the visual formatting

	// Split content into lines for proper alignment
	lines := strings.Split(content, "\n")
	if len(lines) <= 1 {
		// Single line or empty content, use simple format
		return fmt.Sprintf("%s%s", title, content)
	}

	// Multi-line content: align subsequent lines with the first line
	// "⎿ " is 2 characters wide, so we need 3 spaces for alignment (additional space requested)
	indent := "   " // 3 spaces to align with "⎿ " prefix + 1 extra space

	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s%s\n", title, lines[0]))

	// Add subsequent lines with proper indentation
	for i := 1; i < len(lines); i++ {
		result.WriteString(fmt.Sprintf("%s%s\n", indent, lines[i]))
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// Modern TUI with clean, professional interface
var (
	// Color scheme
	primaryColor    = lipgloss.Color("#7C3AED")
	successColor    = lipgloss.Color("#10B981")
	warningColor    = lipgloss.Color("#F59E0B")
	errorColor      = lipgloss.Color("#EF4444")
	mutedColor      = lipgloss.Color("#6B7280")
	toolOutputColor = lipgloss.Color("#6B7280") // Gray color for tool outputs

	// Styles
	headerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1).
			Margin(0, 0, 0, 0) // Reduced margin for more screen space

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4")).
			Bold(true)

	assistantMsgStyle = lipgloss.NewStyle().
				Foreground(successColor)

	toolOutputStyle = lipgloss.NewStyle().
			Foreground(toolOutputColor) // Gray style for tool outputs

	systemMsgStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	processingStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#6B7280")).
			Padding(0, 0).
			Margin(0)
)

// Message types
type (
	streamResponseMsg struct{ content string }
	streamStartMsg    struct{ input string }
	streamChunkMsg    struct{ content string }
	streamContentMsg  struct {
		content    string
		isMarkdown bool
	} // Enhanced message for markdown content
	streamCompleteMsg struct{}
	processingDoneMsg struct{}
	errorOccurredMsg  struct{ err error }
	tickerMsg         struct{}
	tokenUpdateMsg    struct{ count int }
)

// ModernChatModel represents the clean TUI model
type ModernChatModel struct {
	textarea            textarea.Model
	messages            []ChatMessage
	processing          bool
	agent               *agent.ReactAgent
	config              *config.Manager
	width               int
	height              int // Track terminal height for scrolling
	ready               bool
	currentInput        string
	execTimer           ExecutionTimer
	program             *tea.Program
	currentMessage      *ChatMessage    // Track current streaming message
	sessionStartTime    time.Time       // Track session start time
	contentBuffer       strings.Builder // Buffer for accumulating streaming content
	lastRenderedContent string          // Last rendered markdown content to avoid re-rendering
	processingMessage   string          // Fixed processing message for current conversation
	tokenCount          int             // Track consumed tokens
	lastTokenCount      int             // Previous token count for animation
	baseMessageContent  string          // Content from tool outputs (before LLM content)
	scrollOffset        int             // Scroll position for long message history
}

// ChatMessage represents a chat message with type and content
type ChatMessage struct {
	Type      string // "user", "assistant", "system", "processing", "error"
	ChunkType string // "llm_content", "tool_result", "status", "iteration", etc.
	Content   string
	Time      time.Time
}

// ExecutionTimer tracks execution time for processing messages
type ExecutionTimer struct {
	StartTime time.Time
	Duration  time.Duration
	Active    bool
}

// NewModernChatModel creates a clean, modern chat interface
func NewModernChatModel(agent *agent.ReactAgent, config *config.Manager) *ModernChatModel {
	// Configure textarea with clean prompt style
	ta := textarea.New()
	ta.Placeholder = ""
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 2000
	ta.SetHeight(1) // Initial height, will expand dynamically
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(true) // Enable multiline input

	// Set text color to match terminal default (black)
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))

	// No initial messages - we'll display them directly in runModernTUI
	initialMessages := []ChatMessage{}

	return &ModernChatModel{
		textarea:         ta,
		messages:         initialMessages,
		agent:            agent,
		config:           config,
		ready:            false,
		sessionStartTime: time.Now(), // Initialize session start time
	}
}

func (m *ModernChatModel) Init() tea.Cmd {
	return tea.Batch(textarea.Blink, m.startTicker())
}

func (m *ModernChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tiCmd tea.Cmd

	m.textarea, tiCmd = m.textarea.Update(msg)

	// Update textarea height based on content
	m.updateTextareaHeight()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			// Initialize dimensions with 8x8 grid alignment
			textareaWidth := ((msg.Width - 8) / 8) * 8 // Align to 8x8 grid
			m.textarea.SetWidth(textareaWidth)
			m.ready = true
		} else {
			textareaWidth := ((msg.Width - 8) / 8) * 8 // Maintain 8x8 grid alignment
			m.textarea.SetWidth(textareaWidth)
		}
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlC:
			return m, tea.Quit
		case msg.Type == tea.KeyEsc:
			// Interrupt processing
			if m.processing {
				m.processing = false
				m.execTimer.Active = false
				m.addMessage(ChatMessage{
					Type:    "system",
					Content: "⚠️ Processing interrupted by user",
					Time:    time.Now(),
				})
			}
			return m, nil
		case msg.Type == tea.KeyEnter && !msg.Alt:
			// Submit input on plain Enter (Shift+Enter is handled by textarea)
			if !m.processing && strings.TrimSpace(m.textarea.Value()) != "" {
				input := strings.TrimSpace(m.textarea.Value())
				m.currentInput = input
				m.textarea.Reset()

				// Add user message
				m.addMessage(ChatMessage{
					Type:    "user",
					Content: input,
					Time:    time.Now(),
				})

				// Start processing timer and reset token count
				m.processing = true
				m.tokenCount = 0
				m.lastTokenCount = 0
				m.execTimer = ExecutionTimer{
					StartTime: time.Now(),
					Active:    true,
				}

				// Set a fixed processing message for this conversation
				m.processingMessage = message.GetRandomProcessingMessageWithEmoji()

				// No processing message in chat area - status shown above input instead

				return m, tea.Batch(m.processUserInput(input), m.startTicker())
			}
		}

	case streamResponseMsg:
		// No need to remove processing message since it's not in chat area

		// Add execution time to response if available
		content := msg.content
		if m.execTimer.Active || !m.execTimer.StartTime.IsZero() {
			duration := time.Since(m.execTimer.StartTime)
			content += fmt.Sprintf("\n\n⏱️ Execution time: %v", duration.Truncate(10*time.Millisecond))
		}

		m.addMessage(ChatMessage{
			Type:    "assistant",
			Content: content,
			Time:    time.Now(),
		})

		return m, func() tea.Msg { return processingDoneMsg{} }

	case streamStartMsg:
		// No processing message to remove since it's shown above input

		// Create initial assistant message for streaming
		assistantMsg := ChatMessage{
			Type:    "assistant",
			Content: "",
			Time:    time.Now(),
		}
		m.addMessage(assistantMsg)
		m.currentMessage = &m.messages[len(m.messages)-1]

		// Reset base content tracking
		m.baseMessageContent = ""

		return m, nil

	case streamChunkMsg:
		// Handle tool execution and status messages - these are separate from main content
		if m.currentMessage != nil {
			m.currentMessage.Content += msg.content
			// Update base content to include tool outputs
			m.baseMessageContent = m.currentMessage.Content
			// Auto-scroll to show new content
			m.scrollToBottom()
		}
		return m, nil

	case streamContentMsg:
		// Handle LLM streaming content
		if m.currentMessage != nil {
			// In TUI mode, always append content directly to avoid markdown rendering conflicts
			m.currentMessage.Content += msg.content
			// Auto-scroll to show new content
			m.scrollToBottom()
		}
		return m, nil

	case streamCompleteMsg:
		// Add execution time and token consumption to final message
		if m.currentMessage != nil && (m.execTimer.Active || !m.execTimer.StartTime.IsZero()) {
			duration := time.Since(m.execTimer.StartTime)
			executionInfo := fmt.Sprintf("\n\n⏱️ Execution time: %v", duration.Truncate(10*time.Millisecond))

			// Add token consumption if available
			if m.tokenCount > 0 {
				executionInfo += fmt.Sprintf(" • 🪙 %d tokens", m.tokenCount)
			}

			m.currentMessage.Content += executionInfo
		}
		m.currentMessage = nil
		// Reset streaming state for next message
		m.contentBuffer.Reset()
		m.lastRenderedContent = ""
		m.baseMessageContent = ""
		return m, func() tea.Msg { return processingDoneMsg{} }

	case tickerMsg:
		if m.execTimer.Active {
			m.execTimer.Duration = time.Since(m.execTimer.StartTime)

			// Update token count animation state
			m.lastTokenCount = m.tokenCount

			// Real token consumption will be updated via streaming callbacks
			// No fake simulation here

			return m, m.startTicker() // Continue ticking
		} else {
			// Continue ticking for potential future use
			return m, m.startTicker()
		}

	case processingDoneMsg:
		m.processing = false
		m.execTimer.Active = false
		if m.execTimer.StartTime.IsZero() {
			m.execTimer.Duration = 0
		} else {
			m.execTimer.Duration = time.Since(m.execTimer.StartTime)
		}

	case tokenUpdateMsg:
		// Update token count from real agent data
		m.lastTokenCount = m.tokenCount
		m.tokenCount = msg.count
		return m, nil

	case errorOccurredMsg:
		// No processing message to remove since it's shown above input

		// Add execution time and token consumption to error message if available
		errorContent := fmt.Sprintf("Error: %v", msg.err)
		if m.execTimer.Active || !m.execTimer.StartTime.IsZero() {
			duration := time.Since(m.execTimer.StartTime)
			executionInfo := fmt.Sprintf("\n⏱️ Execution time: %v", duration.Truncate(10*time.Millisecond))

			// Add token consumption if available
			if m.tokenCount > 0 {
				executionInfo += fmt.Sprintf(" • 🪙 %d tokens", m.tokenCount)
			}

			errorContent += executionInfo
		}

		m.addMessage(ChatMessage{
			Type:    "error",
			Content: errorContent,
			Time:    time.Now(),
		})
		m.processing = false
		m.execTimer.Active = false
	}

	return m, tiCmd
}

func (m *ModernChatModel) addMessage(msg ChatMessage) {
	m.messages = append(m.messages, msg)
	// Auto-scroll to bottom when new message is added
	m.scrollToBottom()
}

// scrollToBottom scrolls to show the latest messages
func (m *ModernChatModel) scrollToBottom() {
	// Calculate total content height
	totalLines := m.calculateTotalContentHeight()
	availableHeight := m.height - 6 // Reserve space for input and status

	if totalLines > availableHeight {
		m.scrollOffset = totalLines - availableHeight
	} else {
		m.scrollOffset = 0
	}
}

// calculateTotalContentHeight calculates the total height needed to display all messages
func (m *ModernChatModel) calculateTotalContentHeight() int {
	totalLines := 2 // Header + spacing

	for i, msg := range m.messages {
		// Skip empty assistant messages
		if msg.Type == "assistant" && strings.TrimSpace(msg.Content) == "" {
			continue
		}

		// Add spacing between messages
		if i > 0 {
			totalLines++
		}

		// Count lines in message content
		processedContent := m.processContentForTUI(msg.Content)
		lines := strings.Split(processedContent, "\n")
		totalLines += len(lines)
	}

	return totalLines
}

// updateTextareaHeight adjusts the textarea height based on content lines
func (m *ModernChatModel) updateTextareaHeight() {
	content := m.textarea.Value()
	lines := strings.Split(content, "\n")
	lineCount := len(lines)

	// Ensure minimum height of 1, maximum of 10 lines
	if lineCount < 1 {
		lineCount = 1
	}
	if lineCount > 10 {
		lineCount = 10
	}

	// Only update if height changed
	if m.textarea.Height() != lineCount {
		m.textarea.SetHeight(lineCount)
	}
}

func (m *ModernChatModel) startTicker() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickerMsg{}
	})
}

func (m *ModernChatModel) processUserInput(input string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		// Start processing and send immediate start message
		go func() {
			streamCallback := func(chunk agent.StreamChunk) {
				// Send each chunk immediately as it arrives
				var content string
				switch chunk.Type {
				case "status":
					// Skip status messages - handled by processing status above input
					return
				case "iteration":
					// Skip iteration messages - handled by processing status above input
					return
				case "tool_start":
					if chunk.Content != "" {
						content = "\n\n" + chunk.Content
					}
				case "tool_result":
					if chunk.Content != "" {
						content = "\n" + formatToolOutput("⎿ ", chunk.Content)
					}
				case "tool_error":
					if chunk.Content != "" {
						content = "\n❌ " + chunk.Content
					}
				case "final_answer":
					if chunk.Content != "" {
						content = "\n✨ " + chunk.Content
					}
				case "llm_content":
					// Use enhanced streaming for potential markdown content
					m.program.Send(streamContentMsg{content: chunk.Content, isMarkdown: true})
					return // Skip the normal content sending below
				case "complete":
					if chunk.Content != "" {
						content = "\n✅ " + chunk.Content
					}
				case "max_iterations":
					if chunk.Content != "" {
						content = "\n⚠️ " + chunk.Content
					}
				case "context_management":
					if chunk.Content != "" {
						content = "\n🧠 " + chunk.Content
					}
				case "token_usage":
					// Update token count from real usage data
					if chunk.Content != "" {
						// Parse token count from content (assuming it's a number)
						if tokenCount, err := strconv.Atoi(strings.TrimSpace(chunk.Content)); err == nil {
							m.program.Send(tokenUpdateMsg{count: tokenCount})
						}
					}
					return // Don't display token usage in chat
				case "error":
					// Error will be handled separately
				}

				// Send streaming update immediately
				if content != "" {
					m.program.Send(streamChunkMsg{content: content})
				}
			}

			err := m.agent.ProcessMessageStream(ctx, input, m.config.GetConfig(), streamCallback)
			if err != nil {
				m.program.Send(errorOccurredMsg{err: err})
			} else {
				m.program.Send(streamCompleteMsg{})
			}
		}()

		// Return immediately with processing started message
		return streamStartMsg{input: input}
	}
}

func (m *ModernChatModel) View() string {
	if !m.ready {
		return "Initializing Alex Code Agent..."
	}

	var parts []string

	// Header
	header := headerStyle.Render("Alex Code Agent")
	parts = append(parts, header, "")

	// Calculate available height for messages
	headerHeight := 2 // Header + spacing
	inputHeight := 4  // Input area + processing status + shortcuts
	availableHeight := m.height - headerHeight - inputHeight
	if availableHeight < 5 {
		availableHeight = 5 // Minimum height
	}

	// Render messages with scrolling
	messageLines := m.renderMessagesWithScrolling(availableHeight)
	parts = append(parts, messageLines...)

	// Add spacing before input area
	parts = append(parts, "")

	// Processing status (when processing)
	if m.processing {
		elapsed := m.execTimer.Duration.Truncate(time.Second)

		// Format token count with animation effect
		tokenDisplay := fmt.Sprintf("%d", m.tokenCount)
		if m.tokenCount != m.lastTokenCount {
			// Add subtle animation for token changes
			tokenDisplay = fmt.Sprintf("↑ %d", m.tokenCount)
		}

		// Main text in processing color, parentheses content in gray
		mainText := processingStyle.Render("Wibbling… ")
		grayInfo := lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("(%v · %s tokens)", elapsed, tokenDisplay))
		statusMsg := mainText + grayInfo
		parts = append(parts, statusMsg)
	}

	// Input area
	inputArea := inputStyle.Render(m.textarea.View())
	parts = append(parts, inputArea)

	// Add shortcuts hint
	shortcutsHint := systemMsgStyle.Render("  Enter: send • Shift+Enter: new line • Use terminal scroll to view history")
	parts = append(parts, shortcutsHint)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// renderMessagesWithScrolling renders messages with proper scrolling support
func (m *ModernChatModel) renderMessagesWithScrolling(availableHeight int) []string {
	var allLines []string

	// Build all message lines
	for i, msg := range m.messages {
		// Skip empty assistant messages
		if msg.Type == "assistant" && strings.TrimSpace(msg.Content) == "" {
			continue
		}

		// Add spacing between messages
		if i > 0 {
			allLines = append(allLines, "")
		}

		var styledContent string
		switch msg.Type {
		case "user":
			styledContent = userMsgStyle.Render("> ") + msg.Content
		case "assistant":
			// Process content to ensure proper formatting for TUI display
			processedContent := m.processContentForTUI(msg.Content)
			// Apply tool output styling to tool results while keeping regular assistant text green
			styledContent = m.styleAssistantContent(processedContent, msg.ChunkType)
		case "system":
			styledContent = systemMsgStyle.Render(msg.Content)
		case "processing":
			styledContent = processingStyle.Render("⚡ " + msg.Content)
		case "error":
			styledContent = errorMsgStyle.Render("❌ " + msg.Content)
		default:
			styledContent = msg.Content
		}

		// Split multi-line content
		lines := strings.Split(styledContent, "\n")
		allLines = append(allLines, lines...)
	}

	// Apply scrolling
	totalLines := len(allLines)
	if totalLines <= availableHeight {
		// All content fits, no scrolling needed
		m.scrollOffset = 0
		return allLines
	}

	// Apply scroll offset
	startLine := m.scrollOffset
	endLine := startLine + availableHeight

	if startLine < 0 {
		startLine = 0
	}
	if endLine > totalLines {
		endLine = totalLines
	}
	if startLine >= totalLines {
		startLine = totalLines - availableHeight
		if startLine < 0 {
			startLine = 0
		}
	}

	return allLines[startLine:endLine]
}

// styleAssistantContent applies appropriate styling to assistant content
// Uses ChunkType to determine styling: tool outputs are rendered in gray, regular content in green
func (m *ModernChatModel) styleAssistantContent(content string, chunkType string) string {
	// Check if this is tool-related content based on ChunkType
	isToolOutput := chunkType == "tool_result" || chunkType == "tool_start" || chunkType == "tool_error"

	lines := strings.Split(content, "\n")
	var styledLines []string

	for _, line := range lines {
		if isToolOutput || strings.HasPrefix(strings.TrimSpace(line), "⎿") {
			// Tool output line - apply gray styling
			styledLines = append(styledLines, toolOutputStyle.Render(line))
		} else {
			// Regular assistant content - apply green styling
			styledLines = append(styledLines, assistantMsgStyle.Render(line))
		}
	}

	return strings.Join(styledLines, "\n")
}

// processContentForTUI processes content for proper TUI display without full markdown rendering
func (m *ModernChatModel) processContentForTUI(content string) string {
	// Ensure proper line breaks
	content = strings.ReplaceAll(content, "\\n", "\n")

	// Split into lines and process each line
	lines := strings.Split(content, "\n")
	var processedLines []string

	// Calculate available width for content (accounting for margins and styling)
	availableWidth := m.width - 8 // Reserve space for margins and styling
	if availableWidth < 40 {
		availableWidth = 40 // Minimum width to prevent overly narrow text
	}

	for _, line := range lines {
		// Trim excessive whitespace but preserve intentional formatting
		trimmed := strings.TrimRight(line, " \t")

		// Handle long lines by wrapping them
		if len(trimmed) > availableWidth {
			wrapped := m.wrapLine(trimmed, availableWidth)
			processedLines = append(processedLines, wrapped...)
		} else {
			processedLines = append(processedLines, trimmed)
		}
	}

	// Join lines back together
	result := strings.Join(processedLines, "\n")

	// Remove excessive consecutive newlines (more than 2)
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}

// wrapLine wraps a long line into multiple lines based on available width
func (m *ModernChatModel) wrapLine(line string, maxWidth int) []string {
	if len(line) <= maxWidth {
		return []string{line}
	}

	var wrappedLines []string
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{line}
	}

	currentLine := ""
	for _, word := range words {
		// If adding this word would exceed the limit
		if len(currentLine)+len(word)+1 > maxWidth {
			if currentLine != "" {
				wrappedLines = append(wrappedLines, currentLine)
				currentLine = word
			} else {
				// Single word is too long, break it
				if len(word) > maxWidth {
					for len(word) > maxWidth {
						wrappedLines = append(wrappedLines, word[:maxWidth])
						word = word[maxWidth:]
					}
					currentLine = word
				} else {
					currentLine = word
				}
			}
		} else {
			if currentLine == "" {
				currentLine = word
			} else {
				currentLine += " " + word
			}
		}
	}

	if currentLine != "" {
		wrappedLines = append(wrappedLines, currentLine)
	}

	return wrappedLines
}

// Run the modern TUI
func runModernTUI(agent *agent.ReactAgent, config *config.Manager) error {
	model := NewModernChatModel(agent, config)

	// Add initial system message
	model.addMessage(ChatMessage{
		Type:    "system",
		Content: "Press Ctrl+C to exit",
		Time:    time.Now(),
	})

	program := tea.NewProgram(
		model,
		tea.WithAltScreen(), // Use alt screen for proper TUI experience
	)

	// Set the program reference for streaming callbacks
	model.program = program

	_, err := program.Run()
	return err
}
