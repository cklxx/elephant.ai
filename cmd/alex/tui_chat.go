package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/app"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// ═══════════════════════════════════════════════════════════════
// Data Structures
// ═══════════════════════════════════════════════════════════════

// MessageRole represents the role of a message sender
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// ChatMessage represents a single message in the chat history
type ChatMessage struct {
	ID        string
	Role      MessageRole
	Content   string
	Timestamp time.Time
	Metadata  map[string]interface{} // For tool info, tokens, streaming status, etc.
}

// ChatState represents the current state of the chat UI
type ChatState int

const (
	StateIdle ChatState = iota
	StateWaitingForInput
	StateProcessingRequest
	StateStreamingResponse
	StateExecutingTools
	StateError
)

// ToolExecution tracks the state of an active tool call
type ToolExecution struct {
	CallID    string
	Name      string
	Arguments map[string]interface{}
	StartTime time.Time
	Result    string
	Error     error
	Duration  time.Duration
}

// ═══════════════════════════════════════════════════════════════
// Main Model
// ═══════════════════════════════════════════════════════════════

// ChatTUIModel is the comprehensive chat interface model
type ChatTUIModel struct {
	// Core components
	viewport viewport.Model
	textarea textarea.Model
	renderer *glamour.TermRenderer

	// State
	state            ChatState
	messages         []ChatMessage
	streamingMessage *ChatMessage // Currently streaming assistant message
	streamBuffer     *strings.Builder

	// Tool tracking
	activeTools      map[string]ToolExecution
	currentIteration int
	totalIterations  int

	// Context
	container *Container
	sessionID string
	ctx       context.Context
	program   *tea.Program // Reference to the tea.Program for sending events

	// UI dimensions
	width  int
	height int

	// Metadata
	startTime   time.Time
	totalTokens int
	err         error
	ready       bool
}

// NewChatTUIModel creates a new chat TUI model
func NewChatTUIModel(container *Container, sessionID string) ChatTUIModel {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Shift+Enter for newline)"
	ta.Focus()
	ta.Prompt = "┃ "
	ta.CharLimit = 10000
	ta.SetHeight(3)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	// No border - clean display

	return ChatTUIModel{
		viewport:     vp,
		textarea:     ta,
		messages:     make([]ChatMessage, 0),
		activeTools:  make(map[string]ToolExecution, 0),
		container:    container,
		sessionID:    sessionID,
		ctx:          context.Background(),
		state:        StateWaitingForInput,
		startTime:    time.Now(),
		streamBuffer: &strings.Builder{},
	}
}

// ═══════════════════════════════════════════════════════════════
// Bubbletea Interface Implementation
// ═══════════════════════════════════════════════════════════════

func (m ChatTUIModel) Init() tea.Cmd {
	return tea.Batch(
		textarea.Blink,
		func() tea.Msg {
			// Send welcome message via custom message
			return WelcomeMsg{}
		},
	)
}

func (m ChatTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	// ─────────────────────────────────────────────────────────
	// Window & Terminal Events
	// ─────────────────────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate dynamic heights
		headerHeight := 3
		footerHeight := 3
		textareaHeight := 5

		// Update viewport
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - headerHeight - textareaHeight - footerHeight

		// Update textarea
		m.textarea.SetWidth(msg.Width - 4)

		// Initialize renderer
		if m.renderer == nil {
			var err error
			m.renderer, err = glamour.NewTermRenderer(
				glamour.WithAutoStyle(),
				glamour.WithWordWrap(m.viewport.Width-8),
			)
			if err != nil {
				m.err = fmt.Errorf("failed to initialize renderer: %w", err)
			}
		}

		// Mark as ready
		if !m.ready {
			m.ready = true
			m.updateViewportContent()
		}

		return m, nil

	// ─────────────────────────────────────────────────────────
	// Keyboard Events
	// ─────────────────────────────────────────────────────────
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			// Check if Shift+Enter (Alt modifier on some terminals)
			if msg.Alt {
				// Insert newline
				var cmd tea.Cmd
				m.textarea, cmd = m.textarea.Update(msg)
				return m, cmd
			}

			// Send message
			userInput := strings.TrimSpace(m.textarea.Value())
			if userInput == "" {
				return m, nil
			}

			// Add user message ONLY to local display
			// The coordinator will add it to session when executing
			userMsg := ChatMessage{
				ID:        fmt.Sprintf("user-%d", time.Now().UnixNano()),
				Role:      RoleUser,
				Content:   userInput,
				Timestamp: time.Now(),
				Metadata:  make(map[string]interface{}),
			}
			m.messages = append(m.messages, userMsg)

			// Clear textarea
			m.textarea.Reset()

			// Update viewport
			m.updateViewportContent()

			// Change state and execute task
			// NOTE: The coordinator will add this task as a user message to the session
			// and include all previous session history for context
			m.state = StateProcessingRequest
			return m, m.executeTask(userInput, m.program)

		case tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
			// Scroll viewport
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			cmds = append(cmds, cmd)
		}

	// ─────────────────────────────────────────────────────────
	// Agent Events (from coordinator)
	// ─────────────────────────────────────────────────────────
	case app.IterationStartMsg:
		m.currentIteration = msg.Iteration
		m.totalIterations = msg.TotalIters
		m.state = StateStreamingResponse

		// Add iteration marker
		m.addSystemMessage(fmt.Sprintf("─── Iteration %d/%d ───", msg.Iteration, msg.TotalIters))
		return m, nil

	case app.ThinkingMsg:
		m.state = StateStreamingResponse
		// No need to display "Thinking..." as we'll show the actual thought
		return m, nil

	case app.ThinkCompleteMsg:
		if msg.ToolCallCount > 0 {
			m.state = StateExecutingTools
		}

		// Don't display internal thoughts - they're redundant with final answer
		// Only show if there's NO final answer coming (i.e., pure thinking step)
		return m, nil

	case app.ToolCallStartMsg:
		m.state = StateExecutingTools
		m.activeTools[msg.CallID] = ToolExecution{
			CallID:    msg.CallID,
			Name:      msg.ToolName,
			Arguments: msg.Arguments,
			StartTime: msg.Timestamp,
		}

		// Add tool execution message
		argsStr := formatArgs(msg.Arguments)
		toolMsg := fmt.Sprintf("🔧 **%s**(%s)", msg.ToolName, argsStr)
		m.addSystemMessage(toolMsg)
		return m, nil

	case app.ToolCallCompleteMsg:
		exec, exists := m.activeTools[msg.CallID]
		if !exists {
			return m, nil
		}

		exec.Duration = msg.Duration
		exec.Result = msg.Result
		exec.Error = msg.Error

		// Add tool result message
		preview := createToolPreview(exec.Name, msg.Result)
		if msg.Error != nil {
			m.addSystemMessage(fmt.Sprintf("   ✗ Failed: %v (%s)", msg.Error, msg.Duration))
		} else {
			m.addSystemMessage(fmt.Sprintf("   ✓ %s (%s)", preview, msg.Duration))
		}

		delete(m.activeTools, msg.CallID)
		return m, nil

	case app.IterationCompleteMsg:
		m.totalTokens += msg.TokensUsed
		m.addSystemMessage(fmt.Sprintf("   📊 Tokens: %d | Tools: %d", msg.TokensUsed, msg.ToolsRun))
		return m, nil

	case app.TaskCompleteMsg:
		m.state = StateWaitingForInput

		// Add final answer as assistant message
		if msg.FinalAnswer != "" {
			answerMsg := ChatMessage{
				ID:        fmt.Sprintf("answer-%d", time.Now().UnixNano()),
				Role:      RoleAssistant,
				Content:   msg.FinalAnswer,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"type":             "final_answer",
					"total_tokens":     msg.TotalTokens,
					"total_iterations": msg.TotalIterations,
					"duration":         msg.Duration.String(),
				},
			}
			m.messages = append(m.messages, answerMsg)
			m.updateViewportContent()
		}

		// Add completion status
		m.addSystemMessage(fmt.Sprintf("✓ Completed in %d iterations (%s)", msg.TotalIterations, msg.Duration))
		return m, nil

	case app.ErrorMsg:
		m.state = StateError
		m.err = msg.Error
		m.addSystemMessage(fmt.Sprintf("✗ Error: %v", msg.Error))
		return m, nil

	// ─────────────────────────────────────────────────────────
	// Custom Messages
	// ─────────────────────────────────────────────────────────
	case WelcomeMsg:
		// Add welcome message
		welcomeMsg := ChatMessage{
			ID:        "welcome",
			Role:      RoleSystem,
			Content:   "# Welcome to ALEX Chat\n\nALEX is your AI coding agent. Ask me anything about coding, debugging, or system tasks.\n\n**Tips:**\n- Press `Enter` to send a message\n- Press `Shift+Enter` for a new line\n- Press `Ctrl+C` to quit\n- Use `↑↓` to scroll through chat history",
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		}
		m.messages = append(m.messages, welcomeMsg)
		m.updateViewportContent()
		return m, nil

	case SetProgramMsg:
		m.program = msg.Program
		return m, nil

	case StreamChunkMsg:
		// Handle streaming response chunks
		if m.streamingMessage == nil {
			m.streamingMessage = &ChatMessage{
				ID:        fmt.Sprintf("assistant-%d", time.Now().UnixNano()),
				Role:      RoleAssistant,
				Content:   "",
				Timestamp: time.Now(),
				Metadata:  map[string]interface{}{"streaming": true},
			}
			// Initialize streamBuffer if needed
			if m.streamBuffer == nil {
				m.streamBuffer = &strings.Builder{}
			}
		}

		if msg.Done {
			// Finalize streaming message
			if m.streamBuffer != nil {
				m.streamingMessage.Content = m.streamBuffer.String()
			}
			m.streamingMessage.Metadata["streaming"] = false
			m.messages = append(m.messages, *m.streamingMessage)
			m.streamingMessage = nil
			if m.streamBuffer != nil {
				m.streamBuffer.Reset()
			}
			m.updateViewportContent()
			return m, nil
		}

		// Append chunk to buffer
		if m.streamBuffer != nil {
			m.streamBuffer.WriteString(msg.Content)
			m.streamingMessage.Content = m.streamBuffer.String()
		}
		m.updateViewportContent()

		return m, nil
	}

	// Update child components
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m ChatTUIModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// ─────────────────────────────────────────────────────────
	// Header
	// ─────────────────────────────────────────────────────────
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12")).
		Background(lipgloss.Color("235")).
		Padding(0, 1).
		Width(m.width)

	stateStr := m.getStateString()
	elapsed := time.Since(m.startTime).Round(time.Second)
	header := fmt.Sprintf("  ALEX Agent Chat | %s | %s | %d msgs", stateStr, elapsed, len(m.messages))
	headerView := headerStyle.Render(header)

	// ─────────────────────────────────────────────────────────
	// Viewport (messages)
	// ─────────────────────────────────────────────────────────
	viewportView := m.viewport.View()

	// ─────────────────────────────────────────────────────────
	// Textarea (input) - no border for clean look
	// ─────────────────────────────────────────────────────────
	textareaView := m.textarea.View()

	// ─────────────────────────────────────────────────────────
	// Footer
	// ─────────────────────────────────────────────────────────
	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")).
		Padding(0, 1)

	footerContent := m.buildFooter()
	footerView := footerStyle.Render(footerContent)

	// ─────────────────────────────────────────────────────────
	// Compose layout
	// ─────────────────────────────────────────────────────────
	return lipgloss.JoinVertical(
		lipgloss.Left,
		headerView,
		viewportView,
		textareaView,
		footerView,
	)
}

// ═══════════════════════════════════════════════════════════════
// Helper Methods
// ═══════════════════════════════════════════════════════════════

// executeTask runs the agent task in the background
func (m *ChatTUIModel) executeTask(task string, p *tea.Program) tea.Cmd {
	return func() tea.Msg {
		// Execute with TUI event streaming
		result, err := m.container.Coordinator.ExecuteTaskWithTUI(m.ctx, task, m.sessionID, p)
		if err != nil {
			return app.ErrorMsg{
				Timestamp: time.Now(),
				Phase:     "execution",
				Error:     err,
			}
		}

		// Return completion message
		return app.TaskCompleteMsg{
			Timestamp:       time.Now(),
			TotalIterations: result.Iterations,
			TotalTokens:     result.TokensUsed,
			Duration:        time.Since(m.startTime),
			FinalAnswer:     result.Answer,
		}
	}
}

// updateViewportContent re-renders all messages in the viewport
func (m *ChatTUIModel) updateViewportContent() {
	var b strings.Builder

	for i, msg := range m.messages {
		// Add spacing between messages
		if i > 0 {
			b.WriteString("\n\n")
		}

		// Render message based on role
		rendered := m.renderMessage(msg)
		b.WriteString(rendered)
	}

	// Include streaming message if active
	if m.streamingMessage != nil {
		b.WriteString("\n\n")
		rendered := m.renderMessage(*m.streamingMessage)
		b.WriteString(rendered)
		b.WriteString(" ▊") // Cursor indicator
	}

	m.viewport.SetContent(b.String())
	m.viewport.GotoBottom()
}

// renderMessage renders a single message with appropriate styling
func (m *ChatTUIModel) renderMessage(msg ChatMessage) string {
	var b strings.Builder

	// Role indicator
	roleStyle := m.getRoleStyle(msg.Role)
	timestamp := msg.Timestamp.Format("15:04:05")

	rolePrefix := ""
	switch msg.Role {
	case RoleUser:
		rolePrefix = "👤 You"
	case RoleAssistant:
		rolePrefix = "🤖 Alex"
	case RoleSystem:
		rolePrefix = "ℹ️  System"
	case RoleTool:
		rolePrefix = "🔧 Tool"
	}

	header := roleStyle.Render(fmt.Sprintf("%s [%s]", rolePrefix, timestamp))
	b.WriteString(header)
	b.WriteString("\n")

	// Content
	content := msg.Content

	// Render markdown for assistant and system messages
	if msg.Role == RoleAssistant || msg.Role == RoleSystem {
		if m.renderer != nil {
			rendered, err := m.renderer.Render(content)
			if err == nil {
				content = strings.TrimSpace(rendered)
			}
		}
	}

	// Apply content styling
	contentStyle := lipgloss.NewStyle().
		PaddingLeft(2)

	b.WriteString(contentStyle.Render(content))

	return b.String()
}

// getRoleStyle returns the style for a message role
func (m *ChatTUIModel) getRoleStyle(role MessageRole) lipgloss.Style {
	base := lipgloss.NewStyle().Bold(true)

	switch role {
	case RoleUser:
		return base.Foreground(lipgloss.Color("10")) // Green
	case RoleAssistant:
		return base.Foreground(lipgloss.Color("12")) // Blue
	case RoleSystem:
		return base.Foreground(lipgloss.Color("8")) // Gray
	case RoleTool:
		return base.Foreground(lipgloss.Color("11")) // Yellow
	default:
		return base
	}
}

// addSystemMessage adds a system message to the chat
func (m *ChatTUIModel) addSystemMessage(content string) {
	msg := ChatMessage{
		ID:        fmt.Sprintf("system-%d", time.Now().UnixNano()),
		Role:      RoleSystem,
		Content:   content,
		Timestamp: time.Now(),
		Metadata:  make(map[string]interface{}),
	}
	m.messages = append(m.messages, msg)
	m.updateViewportContent()
}

// getStateString returns a human-readable state string
func (m *ChatTUIModel) getStateString() string {
	switch m.state {
	case StateIdle:
		return "Idle"
	case StateWaitingForInput:
		return "Ready"
	case StateProcessingRequest:
		return "Processing..."
	case StateStreamingResponse:
		return "Thinking..."
	case StateExecutingTools:
		return fmt.Sprintf("Running %d tools", len(m.activeTools))
	case StateError:
		return "Error"
	default:
		return "Unknown"
	}
}

// buildFooter constructs the footer with help text and status
func (m *ChatTUIModel) buildFooter() string {
	muted := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

	helpParts := []string{
		key.Render("Enter"),
		muted.Render(" send"),
		muted.Render(" • "),
		key.Render("Shift+Enter"),
		muted.Render(" newline"),
		muted.Render(" • "),
		key.Render("↑↓"),
		muted.Render(" scroll"),
		muted.Render(" • "),
		key.Render("Ctrl+C"),
		muted.Render(" quit"),
	}

	if len(m.activeTools) > 0 {
		helpParts = append(helpParts, muted.Render(fmt.Sprintf(" | %d tools active", len(m.activeTools))))
	}

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		helpParts = append(helpParts, muted.Render(" | "), errStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, helpParts...)
}

// ═══════════════════════════════════════════════════════════════
// Custom Messages
// ═══════════════════════════════════════════════════════════════

// StreamChunkMsg represents a chunk of streaming response
type StreamChunkMsg struct {
	Content string
	Done    bool
}

// WelcomeMsg is sent on initialization to show welcome message
type WelcomeMsg struct{}

// SetProgramMsg is used to inject the program reference
type SetProgramMsg struct {
	Program *tea.Program
}

// ═══════════════════════════════════════════════════════════════
// Entry Point
// ═══════════════════════════════════════════════════════════════

// RunChatTUI starts the interactive chat TUI
func RunChatTUI(container *Container) error {
	// Create a new session
	ctx := context.Background()
	session, err := container.Coordinator.GetSession(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Create chat model
	model := NewChatTUIModel(container, session.ID)

	// Create Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(), // Enable mouse support
	)

	// Inject program reference after creation
	go func() {
		p.Send(SetProgramMsg{Program: p})
	}()

	// Run (blocks until quit)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
