package tui_chat

import (
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/app"

	"github.com/charmbracelet/lipgloss"
)

// addUserMessage adds a user message to the chat
func (m *ChatTUI) addUserMessage(content string) {
	msg := Message{
		ID:        generateID(),
		Role:      "user",
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)
	m.updateViewport()
}

// addAssistantMessage adds an assistant message to the chat
func (m *ChatTUI) addAssistantMessage(content string) {
	msg := Message{
		ID:        generateID(),
		Role:      "assistant",
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)
	m.updateViewport()
}

// addSystemMessage adds a system message to the chat
func (m *ChatTUI) addSystemMessage(content string) {
	msg := Message{
		ID:        generateID(),
		Role:      "system",
		Content:   content,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)
	m.updateViewport()
}

// addErrorMessage adds an error message to the chat
func (m *ChatTUI) addErrorMessage(errMsg string) {
	msg := Message{
		ID:        generateID(),
		Role:      "system",
		Content:   "❌ Error: " + errMsg,
		Timestamp: time.Now(),
	}
	m.messages = append(m.messages, msg)
	m.updateViewport()
}

// addToolMessage adds a tool execution message
func (m *ChatTUI) addToolMessage(event app.ToolCallStartMsg) {
	msg := Message{
		ID:        event.CallID,
		Role:      "tool",
		Timestamp: event.Timestamp,
		ToolCall: &ToolCallInfo{
			ID:        event.CallID,
			Name:      event.ToolName,
			Arguments: event.Arguments,
			Status:    ToolRunning,
			StartTime: event.Timestamp,
		},
	}
	m.messages = append(m.messages, msg)
	m.updateViewport()
}

// updateToolMessage updates a tool message with results
func (m *ChatTUI) updateToolMessage(event app.ToolCallCompleteMsg) {
	for i := range m.messages {
		if m.messages[i].ID == event.CallID && m.messages[i].ToolCall != nil {
			m.messages[i].ToolCall.Result = event.Result
			m.messages[i].ToolCall.Error = event.Error
			m.messages[i].ToolCall.Duration = event.Duration

			if event.Error != nil {
				m.messages[i].ToolCall.Status = ToolError
			} else {
				m.messages[i].ToolCall.Status = ToolSuccess
			}

			// Invalidate cache for this message
			delete(m.messageCache, event.CallID)
			break
		}
	}
	m.updateViewport()
}

// updateViewport re-renders all messages and updates viewport
func (m *ChatTUI) updateViewport() {
	var rendered []string

	for _, msg := range m.messages {
		// Check cache
		if cached, ok := m.messageCache[msg.ID]; ok && cached.width == m.width {
			rendered = append(rendered, cached.content)
			continue
		}

		// Render message
		content := m.renderMessage(msg)

		// Cache it
		m.messageCache[msg.ID] = cachedMessage{
			width:   m.width,
			content: content,
		}

		rendered = append(rendered, content)
	}

	// Update viewport content
	fullContent := strings.Join(rendered, "\n\n")
	m.viewport.SetContent(fullContent)
	m.viewport.GotoBottom() // Auto-scroll to latest message
}

// renderMessage renders a single message based on its role
func (m *ChatTUI) renderMessage(msg Message) string {
	switch msg.Role {
	case "user":
		return m.renderUserMessage(msg)
	case "assistant":
		return m.renderAssistantMessage(msg)
	case "tool":
		return m.renderToolMessage(msg)
	case "system":
		return m.renderSystemMessage(msg)
	}
	return ""
}

// renderUserMessage renders a user message
func (m *ChatTUI) renderUserMessage(msg Message) string {
	style := lipgloss.NewStyle().
		BorderLeft(true).
		BorderForeground(lipgloss.Color("6")). // Cyan
		PaddingLeft(1).
		Foreground(lipgloss.Color("15")) // Bright white

	return style.Render("You: " + msg.Content)
}

// renderAssistantMessage renders an assistant message with markdown
func (m *ChatTUI) renderAssistantMessage(msg Message) string {
	// Render markdown
	var rendered string
	if m.renderer != nil {
		r, err := m.renderer.Render(msg.Content)
		if err != nil {
			rendered = msg.Content // Fallback to plain text
		} else {
			rendered = strings.TrimSpace(r)
		}
	} else {
		rendered = msg.Content
	}

	style := lipgloss.NewStyle().
		BorderLeft(true).
		BorderForeground(lipgloss.Color("12")). // Bright blue
		PaddingLeft(1)

	return style.Render(rendered)
}

// renderSystemMessage renders a system message
func (m *ChatTUI) renderSystemMessage(msg Message) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8")). // Gray
		Italic(true)

	return style.Render(msg.Content)
}

// renderToolMessage renders a tool execution message
func (m *ChatTUI) renderToolMessage(msg Message) string {
	if msg.ToolCall == nil {
		return ""
	}

	tc := msg.ToolCall
	icon := getToolIcon(tc.Name)

	var borderColor lipgloss.Color
	var text string

	switch tc.Status {
	case ToolRunning:
		borderColor = lipgloss.Color("11") // Yellow
		text = fmt.Sprintf("%s %s ...", icon, tc.Name)

	case ToolSuccess:
		borderColor = lipgloss.Color("2") // Green
		preview := createToolPreview(tc.Name, tc.Result)
		text = fmt.Sprintf("✓ %s %s: %s (%s)", icon, tc.Name, preview, tc.Duration)

	case ToolError:
		borderColor = lipgloss.Color("9") // Red
		text = fmt.Sprintf("✗ %s %s: %v", icon, tc.Name, tc.Error)

	default:
		borderColor = lipgloss.Color("8") // Gray
		text = fmt.Sprintf("%s %s (pending)", icon, tc.Name)
	}

	style := lipgloss.NewStyle().
		BorderLeft(true).
		BorderStyle(lipgloss.ThickBorder()).
		BorderForeground(borderColor).
		PaddingLeft(1)

	return style.Render(text)
}
