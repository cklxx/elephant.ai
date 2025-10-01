package tui_chat

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

// ChatTUI is the main Bubble Tea model for interactive chat
type ChatTUI struct {
	// Components
	viewport viewport.Model
	textarea textarea.Model

	// State
	messages     []Message
	messageCache map[string]cachedMessage
	width        int
	height       int
	ready        bool

	// Rendering
	renderer *glamour.TermRenderer

	// Integration
	coordinator *app.AgentCoordinator
	program     *tea.Program
	sessionID   string
	taskRunning bool

	// Metrics
	currentIter int
	totalIters  int
	tokensUsed  int
}

// NewChatTUI creates a new chat TUI model
func NewChatTUI(coordinator *app.AgentCoordinator, sessionID string) *ChatTUI {
	ta := textarea.New()
	ta.Placeholder = "Type your message... (Enter to send, Shift+Enter for newline)"
	ta.Focus()
	ta.CharLimit = -1
	ta.ShowLineNumbers = false

	return &ChatTUI{
		textarea:     ta,
		messages:     []Message{},
		messageCache: make(map[string]cachedMessage),
		coordinator:  coordinator,
		sessionID:    sessionID,
		ready:        false,
	}
}

// SetProgram sets the tea.Program reference (called after creation)
func (m *ChatTUI) SetProgram(p *tea.Program) {
	m.program = p
}

// Init initializes the model
func (m ChatTUI) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles messages
func (m ChatTUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		return m.handleResize(msg)

	// Agent events from EventBridge
	case app.IterationStartMsg:
		m.currentIter = msg.Iteration
		m.totalIters = msg.TotalIters
		return m, nil

	case app.ThinkingMsg:
		// Optionally show thinking indicator
		return m, nil

	case app.ThinkCompleteMsg:
		// Optionally show thought summary
		// Could add system message here if msg.Content is not empty
		return m, nil

	case app.ToolCallStartMsg:
		m.addToolMessage(msg)
		return m, nil

	case app.ToolCallCompleteMsg:
		m.updateToolMessage(msg)
		return m, nil

	case app.TaskCompleteMsg:
		m.addAssistantMessage(msg.FinalAnswer)
		m.taskRunning = false
		m.tokensUsed = msg.TotalTokens
		return m, nil

	case app.ErrorMsg:
		m.addErrorMessage(msg.Error.Error())
		m.taskRunning = false
		return m, nil
	}

	// Update textarea
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m ChatTUI) View() string {
	if !m.ready {
		return "Initializing ALEX Chat..."
	}

	return lipgloss.JoinVertical(
		lipgloss.Top,
		m.renderHeader(),
		m.viewport.View(),
		m.textarea.View(),
		m.renderHelp(),
	)
}

// handleKeyPress processes keyboard input
func (m ChatTUI) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		return m, tea.Quit

	case tea.KeyEnter:
		// Just Enter sends (Shift+Enter is handled by textarea automatically)
		return m.sendMessage()
	}

	// Pass to textarea for normal input
	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// handleResize processes window size changes
func (m ChatTUI) handleResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height

	// Calculate component sizes
	headerHeight := 3
	helpHeight := 2
	textareaHeight := 4

	viewportHeight := msg.Height - headerHeight - helpHeight - textareaHeight

	// Resize viewport
	m.viewport.Width = msg.Width
	m.viewport.Height = viewportHeight

	// Resize textarea
	m.textarea.SetWidth(msg.Width - 4)
	m.textarea.SetHeight(textareaHeight)

	// Invalidate cache if width changed
	oldRenderer := m.renderer
	if m.renderer == nil || m.width != msg.Width {
		m.messageCache = make(map[string]cachedMessage)
		var err error
		m.renderer, err = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(msg.Width-8),
		)
		if err != nil {
			m.renderer = oldRenderer // Keep old renderer on error
		}
	}

	// Re-render messages
	m.updateViewport()

	if !m.ready {
		m.ready = true
		// Add welcome message
		m.addSystemMessage("Welcome to ALEX Chat! Type your message and press Enter to start.")
	}

	return m, nil
}

// sendMessage sends the user's message and executes task
func (m *ChatTUI) sendMessage() (tea.Model, tea.Cmd) {
	content := strings.TrimSpace(m.textarea.Value())
	if content == "" {
		return m, nil
	}

	// Add user message to UI
	m.addUserMessage(content)

	// Clear input
	m.textarea.Reset()

	// Mark task as running
	m.taskRunning = true

	// Execute task in background
	return m, m.executeTask(content)
}

// executeTask runs the coordinator in a goroutine
func (m *ChatTUI) executeTask(task string) tea.Cmd {
	return func() tea.Msg {
		// Execute in background goroutine
		go func() {
			ctx := context.Background()

			// Execute task with TUI event streaming
			_, err := m.coordinator.ExecuteTaskWithTUI(
				ctx,
				task,
				m.sessionID,
				m.program,
			)

			if err != nil {
				// Send error message
				m.program.Send(app.ErrorMsg{
					Timestamp: time.Now(),
					Phase:     "execution",
					Error:     err,
				})
			}
		}()

		return nil // Return immediately
	}
}

// renderHeader renders the header bar
func (m ChatTUI) renderHeader() string {
	modelName := "gpt-4" // TODO: get from config

	status := "Ready"
	if m.taskRunning {
		status = fmt.Sprintf("Running (Iter %d/%d)", m.currentIter, m.totalIters)
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("15")). // Bright white
		Background(lipgloss.Color("12")). // Bright blue
		Padding(0, 1)

	return headerStyle.Render(
		fmt.Sprintf("ALEX Chat | Model: %s | %s | Tokens: %d",
			modelName,
			status,
			m.tokensUsed,
		),
	)
}

// renderHelp renders the help footer
func (m ChatTUI) renderHelp() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Bold(true)

	return lipgloss.JoinHorizontal(lipgloss.Left,
		helpStyle.Render("Press "),
		keyStyle.Render("Enter"),
		helpStyle.Render(" to send • "),
		keyStyle.Render("Shift+Enter"),
		helpStyle.Render(" for newline • "),
		keyStyle.Render("Ctrl+C"),
		helpStyle.Render(" to quit"),
	)
}
