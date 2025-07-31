package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"alex/internal/agent"
	"alex/internal/config"
	"alex/internal/context/message"
	"alex/internal/llm"
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
			Padding(0, 1).
			Margin(0, 1)
)

// Message types
type (
	streamResponseMsg struct{ content string }
	streamStartMsg    struct{ input string }
	streamChunkMsg    struct {
		content   string
		chunkType string
	}
	streamContentMsg struct {
		content    string
		isMarkdown bool
	} // Enhanced message for markdown content
	streamCompleteMsg  struct{}
	processingDoneMsg  struct{}
	errorOccurredMsg   struct{ err error }
	tickerMsg          struct{}
	tokenUpdateMsg     struct{ count int }
	messagesLoadedMsg  struct{ messages []ChatMessage }
	scrollAnimationMsg struct{}
	forceInitMsg       struct{}
)

// VirtualMessageList handles virtual scrolling for large message lists
type VirtualMessageList struct {
	messages      []ChatMessage
	visibleStart  int
	visibleEnd    int
	bufferSize    int
	isLoading     bool
	hasMore       bool
	totalMessages int
	maxMessages   int // Maximum messages to keep in memory
	lastCleanup   time.Time
}

func newVirtualMessageList() *VirtualMessageList {
	return &VirtualMessageList{
		messages:      make([]ChatMessage, 0),
		visibleStart:  0,
		visibleEnd:    0,
		bufferSize:    50, // Show 50 messages at a time
		isLoading:     false,
		hasMore:       true,
		totalMessages: 0,
		maxMessages:   500, // Keep max 500 messages in memory
		lastCleanup:   time.Now(),
	}
}

// ModernChatModel represents the clean TUI model
type ModernChatModel struct {
	textarea            textarea.Model
	viewport            viewport.Model      // Viewport for message scrolling
	virtualList         *VirtualMessageList // Virtual message list for infinite scrolling
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
	viewportNeedsUpdate bool            // Flag to track if viewport content needs updating
	inputAtBottom       bool            // Flag to track if input should be at bottom or after last message
	scrollAnimation     bool            // Flag to enable smooth scrolling animations
	animationFrame      int             // Current animation frame for visual effects
	searchMode          bool            // Flag for search mode
	searchQuery         string          // Current search query
	searchResults       []int           // Indices of matching messages
	currentSearchIndex  int             // Current search result index
	renderedCache       map[int]string  // Cache for already rendered messages
	lastCacheUpdate     time.Time       // Last time cache was updated
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

	// Configure viewport for message scrolling
	vp := viewport.New(80, 24) // Initial size, will be updated on window resize
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	return &ModernChatModel{
		textarea:           ta,
		viewport:           vp,
		virtualList:        newVirtualMessageList(),
		agent:              agent,
		config:             config,
		ready:              false,
		sessionStartTime:   time.Now(), // Initialize session start time
		inputAtBottom:      false,      // Default to input following last message
		scrollAnimation:    true,       // Enable smooth scrolling by default
		animationFrame:     0,
		searchMode:         false, // Default to normal mode
		searchQuery:        "",
		searchResults:      make([]int, 0),
		currentSearchIndex: -1,
		renderedCache:      make(map[int]string),
		lastCacheUpdate:    time.Now(),
	}
}

func (m *ModernChatModel) Init() tea.Cmd {
	// Send delayed force init message as fallback
	forceInit := func() tea.Msg {
		time.Sleep(500 * time.Millisecond)
		return forceInitMsg{}
	}
	return tea.Batch(textarea.Blink, m.startTicker(), forceInit)
}

func (m *ModernChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tiCmd, vpCmd tea.Cmd

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	// Update textarea height based on content
	m.updateTextareaHeight()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if !m.ready {
			// Initialize dimensions with proper padding
			padding := 4 // 2 padding on each side
			textareaWidth := msg.Width - padding
			if textareaWidth < 20 {
				textareaWidth = 20 // Minimum width
			}
			m.textarea.SetWidth(textareaWidth)

			// Configure viewport dimensions
			m.viewport.Width = msg.Width
			m.viewport.Height = 20 // Will be updated dynamically in View()

			m.ready = true
		} else {
			// Update dimensions with proper padding
			padding := 4 // 2 padding on each side
			textareaWidth := msg.Width - padding
			if textareaWidth < 20 {
				textareaWidth = 20 // Minimum width
			}
			m.textarea.SetWidth(textareaWidth)

			// Update viewport dimensions
			m.viewport.Width = msg.Width
		}
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch {
		case msg.Type == tea.KeyCtrlC:
			// Cleanup Kimi cache before quit
			m.cleanupKimiCache()
			return m, tea.Quit
		case msg.Type == tea.KeyEsc:
			// Handle escape key based on current mode
			if m.searchMode {
				// Exit search mode
				m.exitSearchMode()
				return m, nil
			} else if m.processing {
				// Interrupt processing
				m.processing = false
				m.execTimer.Active = false
				m.addMessage(ChatMessage{
					Type:      "system",
					ChunkType: "system",
					Content:   "⚠️ Processing interrupted by user",
					Time:      time.Now(),
				})
			}
			return m, nil
		case msg.Type == tea.KeyCtrlI:
			// Toggle input position (Ctrl+I)
			m.inputAtBottom = !m.inputAtBottom
			m.viewportNeedsUpdate = true
			var positionMsg string
			if m.inputAtBottom {
				positionMsg = "📝 Input moved to bottom"
			} else {
				positionMsg = "📝 Input follows last message"
			}
			m.addMessage(ChatMessage{
				Type:      "system",
				ChunkType: "system",
				Content:   positionMsg,
				Time:      time.Now(),
			})
			return m, nil
		case msg.Type == tea.KeyTab:
			// Toggle smooth scrolling animation (Tab key)
			m.scrollAnimation = !m.scrollAnimation
			var animationMsg string
			if m.scrollAnimation {
				animationMsg = "✨ Smooth scrolling enabled"
			} else {
				animationMsg = "✨ Smooth scrolling disabled"
			}
			m.addMessage(ChatMessage{
				Type:      "system",
				ChunkType: "system",
				Content:   animationMsg,
				Time:      time.Now(),
			})
			return m, nil
		case msg.String() == "pgup":
			// Page up - scroll up quickly
			m.viewport.PageUp()
			return m, nil
		case msg.String() == "pgdown":
			// Page down - scroll down quickly
			m.viewport.PageDown()
			return m, nil
		case msg.String() == "home":
			// Go to top of conversation
			m.viewport.GotoTop()
			return m, nil
		case msg.String() == "end":
			// Go to bottom of conversation
			m.viewport.GotoBottom()
			return m, nil
		case msg.String() == "alt+up":
			// Alt+Up - scroll up by 3 lines
			for i := 0; i < 3; i++ {
				m.viewport.ScrollUp(1)
			}
			return m, nil
		case msg.String() == "alt+down":
			// Alt+Down - scroll down by 3 lines
			for i := 0; i < 3; i++ {
				m.viewport.ScrollDown(1)
			}
			return m, nil
		case msg.String() == "/":
			// Start search mode (Ctrl+F alternative)
			m.startSearchMode()
			return m, nil
		case msg.String() == "ctrl+e":
			// Export conversation history
			m.exportConversation()
			return m, nil
		case msg.Type == tea.KeyEnter && !msg.Alt:
			// Handle Enter key based on current mode
			if m.searchMode {
				// In search mode, Enter finds next result
				m.goToNextSearchResult()
				return m, nil
			} else if !m.processing && strings.TrimSpace(m.textarea.Value()) != "" {
				// Normal mode - submit input
				input := strings.TrimSpace(m.textarea.Value())
				m.currentInput = input
				m.textarea.Reset()

				// Add user message
				m.addMessage(ChatMessage{
					Type:      "user",
					ChunkType: "user_input",
					Content:   input,
					Time:      time.Now(),
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
		default:
			// Handle text input in search mode
			if m.searchMode && len(msg.String()) == 1 {
				// Add character to search query
				m.searchQuery += msg.String()
				m.performSearch(m.searchQuery)
				return m, nil
			} else if m.searchMode && msg.Type == tea.KeyBackspace {
				// Remove character from search query
				if len(m.searchQuery) > 0 {
					m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
					m.performSearch(m.searchQuery)
				}
				return m, nil
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
			Type:      "assistant",
			ChunkType: "final_answer",
			Content:   content,
			Time:      time.Now(),
		})

		return m, func() tea.Msg { return processingDoneMsg{} }

	case streamStartMsg:
		// No processing message to remove since it's shown above input

		// Create initial assistant message for streaming
		assistantMsg := ChatMessage{
			Type:      "assistant",
			ChunkType: "llm_content",
			Content:   "",
			Time:      time.Now(),
		}
		m.addMessage(assistantMsg)
		m.currentMessage = &m.virtualList.messages[len(m.virtualList.messages)-1]

		// Reset base content tracking
		m.baseMessageContent = ""

		return m, nil

	case streamChunkMsg:
		// Handle tool execution and status messages - these are separate from main content
		if m.currentMessage != nil {
			m.currentMessage.Content += msg.content
			// Update ChunkType based on the stream chunk type
			if msg.chunkType != "" {
				m.currentMessage.ChunkType = msg.chunkType
			}
			// Update base content to include tool outputs
			m.baseMessageContent = m.currentMessage.Content
			// Mark viewport for update
			m.viewportNeedsUpdate = true
		}
		return m, nil

	case streamContentMsg:
		// Handle LLM streaming content
		if m.currentMessage != nil {
			// In TUI mode, always append content directly to avoid markdown rendering conflicts
			m.currentMessage.Content += msg.content
			// Set ChunkType for LLM content
			m.currentMessage.ChunkType = "llm_content"
			// Mark viewport for update
			m.viewportNeedsUpdate = true
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

	case scrollAnimationMsg:
		// Handle scroll animation frame updates
		m.animationFrame++
		if m.scrollAnimation {
			return m, m.startScrollAnimation()
		}
		return m, nil

	case forceInitMsg:
		// Force initialization if WindowSizeMsg wasn't received
		if !m.ready {
			// Set default dimensions
			m.textarea.SetWidth(60)
			m.viewport.Width = 80
			m.viewport.Height = 20
			m.width = 80
			m.height = 24
			m.ready = true
			m.viewportNeedsUpdate = true
		}
		return m, nil

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

	case messagesLoadedMsg:
		// Handle loaded historical messages
		m.virtualList.isLoading = false

		if len(msg.messages) > 0 {
			// Prepend historical messages to the beginning
			m.virtualList.messages = append(msg.messages, m.virtualList.messages...)
			m.virtualList.totalMessages = len(m.virtualList.messages)

			// Update visible range to maintain user's position
			m.updateVisibleRange()
			m.viewportNeedsUpdate = true
		} else {
			// No more historical messages available
			m.virtualList.hasMore = false
		}

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
			Type:      "error",
			ChunkType: "error",
			Content:   errorContent,
			Time:      time.Now(),
		})
		m.processing = false
		m.execTimer.Active = false
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *ModernChatModel) addMessage(msg ChatMessage) {
	m.virtualList.messages = append(m.virtualList.messages, msg)
	m.virtualList.totalMessages = len(m.virtualList.messages)

	// Invalidate cache for new message
	m.invalidateCache()

	// Clean up old messages if we exceed the limit
	m.cleanupOldMessages()

	// Update visible range to show latest messages
	m.updateVisibleRange()

	// Mark that viewport needs updating
	m.viewportNeedsUpdate = true
}

// cleanupOldMessages removes old messages to prevent memory overflow
func (m *ModernChatModel) cleanupOldMessages() {
	// Only cleanup every 30 seconds to avoid frequent operations
	if time.Since(m.virtualList.lastCleanup) < 30*time.Second {
		return
	}

	total := len(m.virtualList.messages)
	if total <= m.virtualList.maxMessages {
		return
	}

	// Remove oldest messages, keeping the maxMessages most recent
	excessMessages := total - m.virtualList.maxMessages
	m.virtualList.messages = m.virtualList.messages[excessMessages:]
	m.virtualList.totalMessages = len(m.virtualList.messages)
	m.virtualList.lastCleanup = time.Now()

	// Add a system message to indicate cleanup occurred
	cleanupMsg := ChatMessage{
		Type:      "system",
		ChunkType: "memory_cleanup",
		Content:   fmt.Sprintf("🧠 Memory cleanup: removed %d older messages", excessMessages),
		Time:      time.Now(),
	}
	m.virtualList.messages = append([]ChatMessage{cleanupMsg}, m.virtualList.messages...)
	m.virtualList.totalMessages = len(m.virtualList.messages)
}

// updateVisibleRange calculates which messages should be visible
func (m *ModernChatModel) updateVisibleRange() {
	total := len(m.virtualList.messages)
	if total == 0 {
		m.virtualList.visibleStart = 0
		m.virtualList.visibleEnd = 0
		return
	}

	// Show the last N messages (buffer size)
	start := total - m.virtualList.bufferSize
	if start < 0 {
		start = 0
	}

	m.virtualList.visibleStart = start
	m.virtualList.visibleEnd = total
}

// updateViewportContent updates the viewport content with visible messages and scrolls appropriately
func (m *ModernChatModel) updateViewportContent() {
	if m.viewportNeedsUpdate {
		content := m.renderVisibleMessages()
		m.viewport.SetContent(content)

		// Auto-scroll to bottom for new messages, but check if user is at top for loading
		if m.viewport.AtTop() && m.virtualList.hasMore && !m.virtualList.isLoading {
			// User scrolled to top, trigger loading more messages
			cmd := m.loadPreviousMessages()
			if cmd != nil {
				m.program.Send(cmd())
			}
		} else {
			// Auto-scroll to bottom for new messages
			m.viewport.GotoBottom()
		}

		m.viewportNeedsUpdate = false
	}
}

// loadPreviousMessages loads more historical messages from session storage
func (m *ModernChatModel) loadPreviousMessages() tea.Cmd {
	if m.virtualList.isLoading {
		return nil
	}

	m.virtualList.isLoading = true

	return func() tea.Msg {
		// Load historical messages from session manager
		var historicalMessages []ChatMessage

		if sessionManager := m.agent.GetSessionManager(); sessionManager != nil {
			// Get current session ID
			sessionID, err := m.agent.GetSessionID()
			if err == nil && sessionID != "" {
				// Try to get session history
				if sessionHistory := m.agent.GetSessionHistory(); sessionHistory != nil {
					// Convert session messages to chat messages
					for _, sessionMsg := range sessionHistory {
						// Skip messages that are already loaded
						alreadyLoaded := false
						for _, loadedMsg := range m.virtualList.messages {
							if loadedMsg.Content == sessionMsg.Content &&
								loadedMsg.Time.Equal(sessionMsg.Timestamp) {
								alreadyLoaded = true
								break
							}
						}

						if !alreadyLoaded {
							chatMsg := ChatMessage{
								Type:      sessionMsg.Role, // "user" or "assistant"
								ChunkType: "session_history",
								Content:   sessionMsg.Content,
								Time:      sessionMsg.Timestamp,
							}
							historicalMessages = append(historicalMessages, chatMsg)
						}
					}
				}
			}
		}

		// Simulate loading delay for better UX
		time.Sleep(300 * time.Millisecond)

		return messagesLoadedMsg{messages: historicalMessages}
	}
}

// renderVisibleMessages renders only visible messages for virtual scrolling
func (m *ModernChatModel) renderVisibleMessages() string {
	var parts []string

	// Show loading indicator if we're loading more messages
	if m.virtualList.isLoading && m.virtualList.hasMore {
		parts = append(parts, systemMsgStyle.Render("📥 Loading more messages..."))
		parts = append(parts, "")
	}

	// Show search mode indicator
	if m.searchMode {
		searchStatus := fmt.Sprintf("🔍 Search: %s", m.searchQuery)
		if len(m.searchResults) > 0 {
			searchStatus += fmt.Sprintf(" (%d results)", len(m.searchResults))
		}
		parts = append(parts, systemMsgStyle.Render(searchStatus))
		parts = append(parts, "")
	}

	// Only render visible messages
	visibleMessages := m.virtualList.messages[m.virtualList.visibleStart:m.virtualList.visibleEnd]

	for i, msg := range visibleMessages {
		// Skip empty assistant messages
		if msg.Type == "assistant" && strings.TrimSpace(msg.Content) == "" {
			continue
		}

		// Add spacing between messages
		if i > 0 {
			parts = append(parts, "")
		}

		// Use cached rendering for better performance
		msgIndex := m.virtualList.visibleStart + i
		isSearchResult := m.isSearchResult(msgIndex)
		styledContent := m.renderMessageWithCache(msgIndex, msg, isSearchResult)

		parts = append(parts, styledContent)
	}

	// Add input box after last message if not at bottom
	if !m.inputAtBottom && len(parts) > 0 {
		parts = append(parts, "")
		parts = append(parts, m.renderInlineInput())
	}

	return strings.Join(parts, "\n")
}

// renderInlineInput renders the input box inline with messages
func (m *ModernChatModel) renderInlineInput() string {
	return inputStyle.Render(m.textarea.View())
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

// startScrollAnimation starts smooth scrolling animation
func (m *ModernChatModel) startScrollAnimation() tea.Cmd {
	if !m.scrollAnimation {
		return nil
	}
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return scrollAnimationMsg{}
	})
}

// startSearchMode enters search mode
func (m *ModernChatModel) startSearchMode() {
	m.searchMode = true
	m.searchQuery = ""
	m.searchResults = make([]int, 0)
	m.currentSearchIndex = -1
	m.addMessage(ChatMessage{
		Type:      "system",
		ChunkType: "system",
		Content:   "🔍 Search mode: Type to search, Enter to find next, Esc to exit",
		Time:      time.Now(),
	})
}

// performSearch searches for messages containing the query
func (m *ModernChatModel) performSearch(query string) {
	m.searchQuery = query
	m.searchResults = make([]int, 0)

	if query == "" {
		return
	}

	// Search through all messages
	for i, msg := range m.virtualList.messages {
		if strings.Contains(strings.ToLower(msg.Content), strings.ToLower(query)) {
			m.searchResults = append(m.searchResults, i)
		}
	}

	m.currentSearchIndex = -1
	if len(m.searchResults) > 0 {
		m.goToNextSearchResult()
	}
}

// goToNextSearchResult navigates to the next search result
func (m *ModernChatModel) goToNextSearchResult() {
	if len(m.searchResults) == 0 {
		return
	}

	m.currentSearchIndex++
	if m.currentSearchIndex >= len(m.searchResults) {
		m.currentSearchIndex = 0
	}

	// Scroll to the message
	msgIndex := m.searchResults[m.currentSearchIndex]
	m.scrollToMessage(msgIndex)

	// Update status
	m.addMessage(ChatMessage{
		Type:      "system",
		ChunkType: "search_result",
		Content:   fmt.Sprintf("🔍 Result %d/%d: \"%s\"", m.currentSearchIndex+1, len(m.searchResults), m.searchQuery),
		Time:      time.Now(),
	})
}

// scrollToMessage scrolls to show a specific message
func (m *ModernChatModel) scrollToMessage(messageIndex int) {
	// Update visible range to include the target message
	if messageIndex < m.virtualList.visibleStart {
		// Message is above visible range
		m.virtualList.visibleStart = messageIndex
		m.virtualList.visibleEnd = messageIndex + m.virtualList.bufferSize
		if m.virtualList.visibleEnd >= len(m.virtualList.messages) {
			m.virtualList.visibleEnd = len(m.virtualList.messages)
		}
	} else if messageIndex >= m.virtualList.visibleEnd {
		// Message is below visible range
		m.virtualList.visibleEnd = messageIndex + 1
		m.virtualList.visibleStart = m.virtualList.visibleEnd - m.virtualList.bufferSize
		if m.virtualList.visibleStart < 0 {
			m.virtualList.visibleStart = 0
		}
	}

	m.viewportNeedsUpdate = true
}

// exitSearchMode exits search mode
func (m *ModernChatModel) exitSearchMode() {
	m.searchMode = false
	m.searchQuery = ""
	m.searchResults = make([]int, 0)
	m.currentSearchIndex = -1
	m.addMessage(ChatMessage{
		Type:      "system",
		ChunkType: "system",
		Content:   "🔍 Search mode exited",
		Time:      time.Now(),
	})
}

// exportConversation exports the conversation history to a file
func (m *ModernChatModel) exportConversation() {
	go func() {
		// Generate filename with timestamp
		timestamp := time.Now().Format("2006-01-02_15-04-05")
		filename := fmt.Sprintf("alex_conversation_%s.md", timestamp)

		// Create markdown content
		var content strings.Builder
		content.WriteString("# Alex Code Agent Conversation\n\n")
		content.WriteString(fmt.Sprintf("Exported: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
		content.WriteString(fmt.Sprintf("Total Messages: %d\n\n", len(m.virtualList.messages)))
		content.WriteString("---\n\n")

		// Export all messages
		for i, msg := range m.virtualList.messages {
			// Skip system messages for cleaner export
			if msg.Type == "system" && msg.ChunkType != "user_input" {
				continue
			}

			content.WriteString(fmt.Sprintf("## Message %d\n\n", i+1))
			content.WriteString(fmt.Sprintf("**Type:** %s\n\n", msg.Type))
			content.WriteString(fmt.Sprintf("**Time:** %s\n\n", msg.Time.Format("2006-01-02 15:04:05")))

			switch msg.Type {
			case "user":
				content.WriteString("**User:**\n\n")
			case "assistant":
				content.WriteString("**Assistant:**\n\n")
			default:
				// Capitalize first letter manually
				msgType := msg.Type
				if len(msgType) > 0 {
					msgType = strings.ToUpper(msgType[:1]) + msgType[1:]
				}
				content.WriteString(fmt.Sprintf("**%s:**\n\n", msgType))
			}

			content.WriteString(fmt.Sprintf("%s\n\n", msg.Content))
			content.WriteString("---\n\n")
		}

		// Write to file
		err := os.WriteFile(filename, []byte(content.String()), 0644)
		if err != nil {
			m.program.Send(tea.Msg(fmt.Sprintf("❌ Export failed: %v", err)))
			return
		}

		// Notify user of successful export
		m.addMessage(ChatMessage{
			Type:      "system",
			ChunkType: "export_success",
			Content:   fmt.Sprintf("💾 Conversation exported to: %s", filename),
			Time:      time.Now(),
		})
		m.viewportNeedsUpdate = true
	}()
}

// isSearchResult checks if a message index is a search result
func (m *ModernChatModel) isSearchResult(messageIndex int) bool {
	for _, resultIndex := range m.searchResults {
		if resultIndex == messageIndex {
			return true
		}
	}
	return false
}

// invalidateCache clears the rendered message cache
func (m *ModernChatModel) invalidateCache() {
	m.renderedCache = make(map[int]string)
	m.lastCacheUpdate = time.Now()
}


// renderMessageWithCache renders a message with caching
func (m *ModernChatModel) renderMessageWithCache(msgIndex int, msg ChatMessage, isSearchResult bool) string {
	// Check cache first
	if cached, exists := m.renderedCache[msgIndex]; exists {
		return cached
	}

	// Render the message
	var styledContent string
	switch msg.Type {
	case "user":
		prefix := "🗨️ "
		if isSearchResult {
			prefix = "🔍🗨️ "
		}
		styledContent = userMsgStyle.Render(prefix) + msg.Content
	case "assistant":
		// Process content to ensure proper formatting for TUI display
		processedContent := m.processContentForTUI(msg.Content)
		prefix := "🤖 "
		if isSearchResult {
			prefix = "🔍🤖 "
		}
		// Apply tool output styling to tool results while keeping regular assistant text green
		styledContent = assistantMsgStyle.Render(prefix) + m.styleAssistantContent(processedContent, msg.ChunkType)
	case "system":
		var prefix string
		switch msg.ChunkType {
		case "search_result":
			prefix = "🔍 "
		case "export_success":
			prefix = "💾 "
		case "memory_cleanup":
			prefix = "🧠 "
		default:
			prefix = "ℹ️ "
		}
		styledContent = systemMsgStyle.Render(prefix + msg.Content)
	case "processing":
		styledContent = processingStyle.Render("⚡ " + msg.Content)
	case "error":
		styledContent = errorMsgStyle.Render("❌ " + msg.Content)
	default:
		styledContent = msg.Content
	}

	// Cache the result
	m.renderedCache[msgIndex] = styledContent

	return styledContent
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
						content = "\n\n✨ " + chunk.Content
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
					// Update token count from real usage data using chunk fields like cobra_cli.go
					if chunk.TotalTokensUsed > 0 {
						m.program.Send(tokenUpdateMsg{count: chunk.TotalTokensUsed})
					} else if chunk.TokensUsed > 0 {
						m.program.Send(tokenUpdateMsg{count: chunk.TokensUsed})
					}
					return // Don't display token usage in chat
				case "error":
					// Error will be handled separately
				}

				if !strings.HasPrefix(content, "\n") {
					content = "\n" + content
				}
				// Send streaming update immediately
				if content != "" {
					m.program.Send(streamChunkMsg{content: content, chunkType: chunk.Type})
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

	// Header
	header := headerStyle.Render("Alex Code Agent")

	// Calculate heights based on input position
	headerHeight := 2 // Header + spacing
	processingHeight := 0
	if m.processing {
		processingHeight = 1
	}

	// Input height calculation depends on position
	inputHeight := 0
	if m.inputAtBottom {
		inputHeight = m.textarea.Height() + 2 // textarea + border
	}

	shortcutsHeight := 1
	fixedHeight := headerHeight + processingHeight + inputHeight + shortcutsHeight

	// Calculate available height for messages
	availableHeight := m.height - fixedHeight
	if availableHeight < 5 {
		availableHeight = 5
	}

	// Update viewport size to match available space
	m.viewport.Height = availableHeight

	// Update viewport content only when needed to prevent flickering
	m.updateViewportContent()

	var parts []string
	parts = append(parts, header, "")

	// Always use viewport for consistent rendering - avoids flickering from mode switching
	parts = append(parts, m.viewport.View())
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

	// Input area - only show at bottom if inputAtBottom is true
	if m.inputAtBottom {
		inputArea := inputStyle.Render(m.textarea.View())
		parts = append(parts, inputArea)
	}

	// Add shortcuts hint
	shortcutsHint := systemMsgStyle.Render("  Enter: send • /: search • Ctrl+I: input pos • Ctrl+E: export • PgUp/PgDn/Home/End: navigate")
	parts = append(parts, shortcutsHint)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

// This function is no longer needed as we use viewport for scrolling

// styleAssistantContent applies appropriate styling to assistant content
// Uses ChunkType to determine styling: tool outputs are rendered in gray, regular content in green
func (m *ModernChatModel) styleAssistantContent(content string, chunkType string) string {
	// Check if this is tool-related content based on ChunkType
	isToolOutput := chunkType == "tool_result" || chunkType == "tool_start" || chunkType == "tool_error"

	lines := strings.Split(content, "\n")
	var styledLines []string

	for _, line := range lines {

		if isToolOutput {
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

		// Handle long lines by wrapping them (use rune count for proper Unicode handling)
		if utf8.RuneCountInString(trimmed) > availableWidth {
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

// wrapLine wraps a long line into multiple lines based on available width with proper Unicode support
func (m *ModernChatModel) wrapLine(line string, maxWidth int) []string {
	// Use rune count for proper Unicode handling
	if utf8.RuneCountInString(line) <= maxWidth {
		return []string{line}
	}

	var wrappedLines []string
	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{line}
	}

	currentLine := ""
	for _, word := range words {
		// Calculate rune lengths for proper Unicode handling
		currentLineLen := utf8.RuneCountInString(currentLine)
		wordLen := utf8.RuneCountInString(word)

		// If adding this word would exceed the limit
		if currentLineLen+wordLen+1 > maxWidth {
			if currentLine != "" {
				wrappedLines = append(wrappedLines, currentLine)
				currentLine = word
			} else {
				// Single word is too long, break it carefully at rune boundaries
				if wordLen > maxWidth {
					wordRunes := []rune(word)
					for len(wordRunes) > maxWidth {
						wrappedLines = append(wrappedLines, string(wordRunes[:maxWidth]))
						wordRunes = wordRunes[maxWidth:]
					}
					if len(wordRunes) > 0 {
						currentLine = string(wordRunes)
					}
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

// cleanupKimiCache cleans up Kimi cache for TUI mode
func (m *ModernChatModel) cleanupKimiCache() {
	if m.agent == nil {
		return
	}

	// Get current session ID
	sessionID, _ := m.agent.GetSessionID()
	if sessionID == "" {
		return
	}

	// Use the generic cleanup function from llm package
	if err := llm.CleanupKimiCacheForSession(sessionID, m.config.GetLLMConfig()); err != nil {
		log.Printf("[DEBUG] TUI: Failed to cleanup Kimi cache: %v", err)
	}
}

// Run the modern TUI
func runModernTUI(agent *agent.ReactAgent, config *config.Manager) error {
	model := NewModernChatModel(agent, config)

	// Add initial system message
	model.addMessage(ChatMessage{
		Type:      "system",
		ChunkType: "system",
		Content:   "Press Ctrl+C to exit",
		Time:      time.Now(),
	})

	// Try to create program with different options for different environments
	var program *tea.Program
	if isTTYAvailable() {
		program = tea.NewProgram(
			model,
			tea.WithAltScreen(), // Use alt screen for proper TUI experience
		)
	} else {
		// Fallback for environments without proper TTY
		program = tea.NewProgram(
			model,
			// No alt screen, try basic mode
		)
	}

	// Set the program reference for streaming callbacks
	model.program = program

	_, err := program.Run()
	if err != nil {
		// If TUI fails, provide helpful error message
		return fmt.Errorf("TUI initialization failed (no TTY available): %w\n\nTry running without TUI:\n  alex \"your prompt here\"", err)
	}
	return err
}

// isTTYAvailable checks if TTY is available for TUI
func isTTYAvailable() bool {
	// Check if we can access /dev/tty
	if _, err := os.Open("/dev/tty"); err != nil {
		return false
	}
	return true
}
