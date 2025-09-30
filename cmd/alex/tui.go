package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// TUI model following Bubble Tea's Elm architecture
type tuiModel struct {
	container   *Container
	sessionID   string
	viewport    viewport.Model
	textarea    textarea.Model
	messages    []string // Message history
	processing  bool
	width       int
	height      int
	ready       bool
	err         error
}

// Styles - minimal like Claude Code
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))
)

// Messages for Bubble Tea
type (
	taskCompleteMsg struct {
		answer string
		stats  string
	}
	taskErrorMsg error
)

func newTUIModel(container *Container, sessionID string) tuiModel {
	ta := textarea.New()
	ta.Placeholder = "Type your task and press Enter..."
	ta.Focus()
	ta.CharLimit = 4000
	ta.SetHeight(1)
	ta.ShowLineNumbers = false

	vp := viewport.New(80, 20)
	vp.HighPerformanceRendering = true

	// Welcome message
	welcome := []string{
		titleStyle.Render("ALEX - AI Code Agent"),
		dimStyle.Render(fmt.Sprintf("Session: %s", sessionID)),
		"",
	}

	return tuiModel{
		container: container,
		sessionID: sessionID,
		textarea:  ta,
		viewport:  vp,
		messages:  welcome,
	}
}

func (m tuiModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEnter:
			if !m.processing && m.textarea.Value() != "" {
				input := strings.TrimSpace(m.textarea.Value())
				m.textarea.Reset()
				m.processing = true

				// Add user input to messages
				m.messages = append(m.messages, "", fmt.Sprintf("> %s", input), "")
				m.updateViewport()

				return m, m.executeTask(input)
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 2
		footerHeight := 4 // input + status
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight - footerHeight

		if !m.ready {
			m.ready = true
			m.textarea.SetWidth(msg.Width - 4)
		} else {
			m.textarea.SetWidth(msg.Width - 4)
		}

		m.updateViewport()

	case taskCompleteMsg:
		m.processing = false
		if msg.answer != "" {
			// Render markdown
			rendered := renderMarkdownTUI(msg.answer, m.width)
			m.messages = append(m.messages, rendered)
		}
		m.messages = append(m.messages, "", dimStyle.Render(msg.stats), "")
		m.updateViewport()

	case taskErrorMsg:
		m.processing = false
		m.messages = append(m.messages, fmt.Sprintf("Error: %v", msg), "")
		m.updateViewport()
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m tuiModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Main viewport
	vpView := m.viewport.View()

	// Input area
	inputView := inputStyle.Render(m.textarea.View())

	// Status bar
	var status string
	if m.processing {
		status = statusStyle.Render("⚡ Processing... | Press Ctrl+C to quit")
	} else {
		status = statusStyle.Render("✓ Ready | Press Ctrl+C to quit")
	}

	// Vertical layout
	return lipgloss.JoinVertical(
		lipgloss.Left,
		vpView,
		strings.Repeat("─", m.width),
		inputView,
		status,
	)
}

func (m *tuiModel) updateViewport() {
	content := strings.Join(m.messages, "\n")
	m.viewport.SetContent(content)
	m.viewport.GotoBottom()
}

func (m tuiModel) executeTask(input string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		result, err := m.container.Coordinator.ExecuteTask(ctx, input, m.sessionID)
		if err != nil {
			return taskErrorMsg(err)
		}

		stats := fmt.Sprintf("✓ Completed in %d iterations, %d tokens", result.Iterations, result.TokensUsed)
		return taskCompleteMsg{
			answer: result.Answer,
			stats:  stats,
		}
	}
}

// renderMarkdownTUI renders markdown with glamour
func renderMarkdownTUI(content string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width-4),
	)
	if err != nil {
		return content
	}

	rendered, err := r.Render(content)
	if err != nil {
		return content
	}

	return strings.TrimSpace(rendered)
}

// RunTUI starts the Bubble Tea TUI
func RunTUI(container *Container) error {
	// Create session
	ctx := context.Background()
	session, err := container.SessionStore.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	p := tea.NewProgram(
		newTUIModel(container, session.ID),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err = p.Run()
	return err
}
