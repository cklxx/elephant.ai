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
	"alex/internal/llm"
)

// StreamBuffer manages infinite data stream with optimized memory usage
type StreamBuffer struct {
	items           []ChatMessage
	capacity        int
	size            int
	head            int  // Start of valid data
	tail            int  // End of valid data
	totalProcessed  int  // Total items ever processed
	isCircular      bool // Whether buffer has wrapped around
}

func newStreamBuffer(capacity int) *StreamBuffer {
	return &StreamBuffer{
		items:          make([]ChatMessage, capacity),
		capacity:       capacity,
		size:           0,
		head:           0,
		tail:           0,
		totalProcessed: 0,
		isCircular:     false,
	}
}

func (sb *StreamBuffer) append(item ChatMessage) {
	if sb.size < sb.capacity {
		// Buffer not full yet
		sb.items[sb.tail] = item
		sb.tail = (sb.tail + 1) % sb.capacity
		sb.size++
	} else {
		// Buffer is full, overwrite oldest
		sb.items[sb.tail] = item
		sb.tail = (sb.tail + 1) % sb.capacity
		sb.head = (sb.head + 1) % sb.capacity
		sb.isCircular = true
	}
	sb.totalProcessed++
}

func (sb *StreamBuffer) getVisible(start, count int) []ChatMessage {
	if sb.size == 0 || start >= sb.size {
		return []ChatMessage{}
	}

	end := start + count
	if end > sb.size {
		end = sb.size
	}

	result := make([]ChatMessage, 0, end-start)
	for i := start; i < end; i++ {
		idx := (sb.head + i) % sb.capacity
		result = append(result, sb.items[idx])
	}
	return result
}

func (sb *StreamBuffer) len() int {
	return sb.size
}

// VirtualViewport manages efficient rendering of visible content only
type VirtualViewport struct {
	buffer         *StreamBuffer
	viewportHeight int
	scrollOffset   int
	preloadBuffer  int
	itemHeight     int // Average lines per message
}

func newVirtualViewport(bufferCapacity int) *VirtualViewport {
	return &VirtualViewport{
		buffer:         newStreamBuffer(bufferCapacity),
		viewportHeight: 20,
		scrollOffset:   0,
		preloadBuffer:  10,
		itemHeight:     3,
	}
}

func (vv *VirtualViewport) addMessage(msg ChatMessage) {
	vv.buffer.append(msg)
	// Auto-scroll to bottom for new messages
	vv.scrollToBottom()
}

func (vv *VirtualViewport) scrollToBottom() {
	maxOffset := vv.buffer.len() - (vv.viewportHeight / vv.itemHeight)
	if maxOffset < 0 {
		maxOffset = 0
	}
	vv.scrollOffset = maxOffset
}

func (vv *VirtualViewport) scrollUp(lines int) {
	vv.scrollOffset -= lines
	if vv.scrollOffset < 0 {
		vv.scrollOffset = 0
	}
}

func (vv *VirtualViewport) scrollDown(lines int) {
	maxOffset := vv.buffer.len() - (vv.viewportHeight / vv.itemHeight)
	if maxOffset < 0 {
		maxOffset = 0
	}
	vv.scrollOffset += lines
	if vv.scrollOffset > maxOffset {
		vv.scrollOffset = maxOffset
	}
}

func (vv *VirtualViewport) getVisibleMessages() []ChatMessage {
	itemsPerView := max(1, vv.viewportHeight/vv.itemHeight)
	start := max(0, vv.scrollOffset-vv.preloadBuffer)
	count := itemsPerView + (2 * vv.preloadBuffer)
	return vv.buffer.getVisible(start, count)
}

func (vv *VirtualViewport) updateViewportHeight(height int) {
	vv.viewportHeight = height
}

// StreamTUIModel - Optimized model for infinite scroll
type StreamTUIModel struct {
	textarea       textarea.Model
	viewport       viewport.Model
	virtualView    *VirtualViewport
	processing     bool
	agent          *agent.ReactAgent
	config         *config.Manager
	width          int
	height         int
	ready          bool
	currentInput   string
	execTimer      ExecutionTimer
	program        *tea.Program
	currentMessage *ChatMessage
	tokenCount     int
	inputAtBottom  bool
}

// Message types for streaming
type (
	streamStartMsg     struct{ input string }
	streamChunkMsg     struct{ content, chunkType string }
	streamContentMsg   struct{ content string }
	streamCompleteMsg  struct{}
	processingDoneMsg  struct{}
	errorOccurredMsg   struct{ err error }
	tickerMsg          struct{}
	tokenUpdateMsg     struct{ count int }
	forceInitMsg       struct{}
)

// ChatMessage represents a message in the stream
type ChatMessage struct {
	Type      string // "user", "assistant", "system", "processing", "error"
	ChunkType string // "llm_content", "tool_result", "status", etc.
	Content   string
	Time      time.Time
}

// ExecutionTimer tracks processing time
type ExecutionTimer struct {
	StartTime time.Time
	Duration  time.Duration
	Active    bool
}

// Color scheme and styles
var (
	primaryColor    = lipgloss.Color("#7C3AED")
	successColor    = lipgloss.Color("#10B981")
	warningColor    = lipgloss.Color("#F59E0B")
	errorColor      = lipgloss.Color("#EF4444")
	mutedColor      = lipgloss.Color("#6B7280")
	toolOutputColor = lipgloss.Color("#6B7280")

	headerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1)

	userMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#06B6D4")).
			Bold(true)

	assistantMsgStyle = lipgloss.NewStyle().
				Foreground(successColor)

	toolOutputStyle = lipgloss.NewStyle().
			Foreground(toolOutputColor)

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

func NewStreamTUIModel(agent *agent.ReactAgent, config *config.Manager) *StreamTUIModel {
	// Configure textarea
	ta := textarea.New()
	ta.Placeholder = ""
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 2000
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(true)

	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))
	ta.BlurredStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("0"))

	// Configure viewport
	vp := viewport.New(80, 24)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	return &StreamTUIModel{
		textarea:      ta,
		viewport:      vp,
		virtualView:   newVirtualViewport(10000), // 10K message buffer
		agent:         agent,
		config:        config,
		ready:         false,
		inputAtBottom: true, // Default to bottom input
	}
}

func (m *StreamTUIModel) Init() tea.Cmd {
	m.initializeDefaultDimensions()
	return tea.Batch(textarea.Blink, m.startTicker(), func() tea.Msg { return forceInitMsg{} })
}

func (m *StreamTUIModel) initializeDefaultDimensions() {
	defaultWidth, defaultHeight := 80, 24
	
	m.textarea.SetWidth(defaultWidth - 4)
	m.viewport.Width = defaultWidth
	m.viewport.Height = 20
	m.width = defaultWidth
	m.height = defaultHeight
	m.virtualView.updateViewportHeight(20)
	m.ready = true
}

func (m *StreamTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tiCmd, vpCmd tea.Cmd
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	m.updateTextareaHeight()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.handleWindowResize(msg)
		
	case tea.KeyMsg:
		return m.handleKeyInput(msg)
		
	case streamStartMsg:
		return m.handleStreamStart(msg)
		
	case streamChunkMsg:
		return m.handleStreamChunk(msg)
		
	case streamContentMsg:
		return m.handleStreamContent(msg)
		
	case streamCompleteMsg:
		return m.handleStreamComplete()
		
	case processingDoneMsg:
		m.processing = false
		m.execTimer.Active = false
		
	case tokenUpdateMsg:
		m.tokenCount = msg.count
		
	case errorOccurredMsg:
		return m.handleError(msg)
		
	case tickerMsg:
		return m.handleTicker()
		
	case forceInitMsg:
		if !m.ready {
			m.initializeDefaultDimensions()
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *StreamTUIModel) handleWindowResize(msg tea.WindowSizeMsg) {
	padding := 4
	textareaWidth := max(20, msg.Width-padding)
	
	m.textarea.SetWidth(textareaWidth)
	m.viewport.Width = msg.Width
	m.width = msg.Width
	m.height = msg.Height
	
	// Update virtual viewport height
	availableHeight := m.calculateViewportHeight()
	m.viewport.Height = availableHeight
	m.virtualView.updateViewportHeight(availableHeight)
}

func (m *StreamTUIModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlC:
		m.cleanupKimiCache()
		return m, tea.Quit
		
	case msg.Type == tea.KeyEsc:
		if m.processing {
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
		m.inputAtBottom = !m.inputAtBottom
		pos := "follows last message"
		if m.inputAtBottom {
			pos = "bottom"
		}
		m.addMessage(ChatMessage{
			Type:      "system",
			ChunkType: "system",
			Content:   "📝 Input moved to " + pos,
			Time:      time.Now(),
		})
		return m, nil
		
	case msg.String() == "pgup":
		m.virtualView.scrollUp(5)
		m.updateViewportContent()
		return m, nil
		
	case msg.String() == "pgdown":
		m.virtualView.scrollDown(5)
		m.updateViewportContent()
		return m, nil
		
	case msg.String() == "home":
		m.virtualView.scrollOffset = 0
		m.updateViewportContent()
		return m, nil
		
	case msg.String() == "end":
		m.virtualView.scrollToBottom()
		m.updateViewportContent()
		return m, nil
		
	case msg.Type == tea.KeyEnter && !msg.Alt:
		return m.handleSubmit()
	}
	
	return m, nil
}

func (m *StreamTUIModel) handleSubmit() (tea.Model, tea.Cmd) {
	if !m.processing && strings.TrimSpace(m.textarea.Value()) != "" {
		input := strings.TrimSpace(m.textarea.Value())
		m.currentInput = input
		m.textarea.Reset()

		m.addMessage(ChatMessage{
			Type:      "user",
			ChunkType: "user_input",
			Content:   input,
			Time:      time.Now(),
		})

		m.processing = true
		m.tokenCount = 0
		m.execTimer = ExecutionTimer{
			StartTime: time.Now(),
			Active:    true,
		}

		return m, tea.Batch(m.processUserInput(input), m.startTicker())
	}
	return m, nil
}

func (m *StreamTUIModel) handleStreamStart(_ streamStartMsg) (tea.Model, tea.Cmd) {
	assistantMsg := ChatMessage{
		Type:      "assistant",
		ChunkType: "llm_content",
		Content:   "",
		Time:      time.Now(),
	}
	m.addMessage(assistantMsg)
	// Get reference to the actual last message in the buffer
	if m.virtualView.buffer.len() > 0 {
		lastIdx := (m.virtualView.buffer.head + m.virtualView.buffer.size - 1) % m.virtualView.buffer.capacity
		m.currentMessage = &m.virtualView.buffer.items[lastIdx]
	}
	return m, nil
}

func (m *StreamTUIModel) handleStreamChunk(msg streamChunkMsg) (tea.Model, tea.Cmd) {
	if m.currentMessage != nil {
		m.currentMessage.Content += msg.content
		if msg.chunkType != "" {
			m.currentMessage.ChunkType = msg.chunkType
		}
		m.updateViewportContent()
	}
	return m, nil
}

func (m *StreamTUIModel) handleStreamContent(msg streamContentMsg) (tea.Model, tea.Cmd) {
	if m.currentMessage != nil {
		m.currentMessage.Content += msg.content
		m.currentMessage.ChunkType = "llm_content"
		m.updateViewportContent()
	}
	return m, nil
}

func (m *StreamTUIModel) handleStreamComplete() (tea.Model, tea.Cmd) {
	if m.currentMessage != nil && (m.execTimer.Active || !m.execTimer.StartTime.IsZero()) {
		duration := time.Since(m.execTimer.StartTime)
		executionInfo := fmt.Sprintf("\n\n⏱️ Execution time: %v", duration.Truncate(10*time.Millisecond))
		if m.tokenCount > 0 {
			executionInfo += fmt.Sprintf(" • 🪙 %d tokens", m.tokenCount)
		}
		m.currentMessage.Content += executionInfo
	}
	m.currentMessage = nil
	return m, func() tea.Msg { return processingDoneMsg{} }
}

func (m *StreamTUIModel) handleError(msg errorOccurredMsg) (tea.Model, tea.Cmd) {
	errorContent := fmt.Sprintf("Error: %v", msg.err)
	if m.execTimer.Active || !m.execTimer.StartTime.IsZero() {
		duration := time.Since(m.execTimer.StartTime)
		executionInfo := fmt.Sprintf("\n⏱️ Execution time: %v", duration.Truncate(10*time.Millisecond))
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
	return m, nil
}

func (m *StreamTUIModel) handleTicker() (tea.Model, tea.Cmd) {
	if m.execTimer.Active {
		m.execTimer.Duration = time.Since(m.execTimer.StartTime)
		return m, m.startTicker()
	}
	return m, m.startTicker()
}

func (m *StreamTUIModel) addMessage(msg ChatMessage) {
	m.virtualView.addMessage(msg)
	m.updateViewportContent()
}

func (m *StreamTUIModel) updateViewportContent() {
	content := m.renderVisibleMessages()
	m.viewport.SetContent(content)
	// Always stay at bottom for new messages unless user explicitly scrolled up
	if m.virtualView.scrollOffset >= m.virtualView.buffer.len()-(m.virtualView.viewportHeight/m.virtualView.itemHeight) {
		m.viewport.GotoBottom()
	}
}

func (m *StreamTUIModel) renderVisibleMessages() string {
	var parts []string
	visibleMessages := m.virtualView.getVisibleMessages()

	for i, msg := range visibleMessages {
		if msg.Type == "assistant" && strings.TrimSpace(msg.Content) == "" {
			continue
		}

		if i > 0 {
			parts = append(parts, "")
		}

		styledContent := m.renderMessage(msg)
		parts = append(parts, styledContent)
	}

	// Add input inline if not at bottom
	if !m.inputAtBottom && len(parts) > 0 {
		parts = append(parts, "", inputStyle.Render(m.textarea.View()))
	}

	return strings.Join(parts, "\n")
}

func (m *StreamTUIModel) renderMessage(msg ChatMessage) string {
	switch msg.Type {
	case "user":
		return userMsgStyle.Render("🗨️ ") + msg.Content
	case "assistant":
		content := m.processContentForTUI(msg.Content)
		return assistantMsgStyle.Render("🤖 ") + m.styleAssistantContent(content, msg.ChunkType)
	case "system":
		prefix := "ℹ️ "
		if strings.Contains(msg.ChunkType, "search") {
			prefix = "🔍 "
		} else if strings.Contains(msg.ChunkType, "export") {
			prefix = "💾 "
		}
		return systemMsgStyle.Render(prefix + msg.Content)
	case "processing":
		return processingStyle.Render("⚡ " + msg.Content)
	case "error":
		return errorMsgStyle.Render("❌ " + msg.Content)
	default:
		return msg.Content
	}
}

func (m *StreamTUIModel) styleAssistantContent(content string, chunkType string) string {
	isToolOutput := chunkType == "tool_result" || chunkType == "tool_start" || chunkType == "tool_error"
	
	lines := strings.Split(content, "\n")
	var styledLines []string

	for _, line := range lines {
		if isToolOutput {
			styledLines = append(styledLines, toolOutputStyle.Render(line))
		} else {
			styledLines = append(styledLines, assistantMsgStyle.Render(line))
		}
	}

	return strings.Join(styledLines, "\n")
}

func (m *StreamTUIModel) processContentForTUI(content string) string {
	content = strings.ReplaceAll(content, "\\n", "\n")
	
	lines := strings.Split(content, "\n")
	var processedLines []string
	
	availableWidth := max(40, m.width-8)
	
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if utf8.RuneCountInString(trimmed) > availableWidth {
			wrapped := m.wrapLine(trimmed, availableWidth)
			processedLines = append(processedLines, wrapped...)
		} else {
			processedLines = append(processedLines, trimmed)
		}
	}
	
	result := strings.Join(processedLines, "\n")
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}
	
	return result
}

func (m *StreamTUIModel) wrapLine(line string, maxWidth int) []string {
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
		currentLineLen := utf8.RuneCountInString(currentLine)
		wordLen := utf8.RuneCountInString(word)

		if currentLineLen+wordLen+1 > maxWidth {
			if currentLine != "" {
				wrappedLines = append(wrappedLines, currentLine)
				currentLine = word
			} else {
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

func (m *StreamTUIModel) updateTextareaHeight() {
	content := m.textarea.Value()
	lines := strings.Split(content, "\n")
	lineCount := max(1, min(len(lines), 10))
	
	if m.textarea.Height() != lineCount {
		m.textarea.SetHeight(lineCount)
	}
}

func (m *StreamTUIModel) calculateViewportHeight() int {
	headerHeight := 2
	processingHeight := 0
	if m.processing {
		processingHeight = 1
	}
	
	inputHeight := 0
	if m.inputAtBottom {
		inputHeight = m.textarea.Height() + 2
	}
	
	shortcutsHeight := 1
	fixedHeight := headerHeight + processingHeight + inputHeight + shortcutsHeight
	
	availableHeight := max(5, m.height-fixedHeight)
	return availableHeight
}

func (m *StreamTUIModel) startTicker() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickerMsg{}
	})
}

func (m *StreamTUIModel) processUserInput(input string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()

		go func() {
			streamCallback := func(chunk agent.StreamChunk) {
				var content string
				switch chunk.Type {
				case "status", "iteration":
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
					m.program.Send(streamContentMsg{content: chunk.Content})
					return
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
					if chunk.TotalTokensUsed > 0 {
						m.program.Send(tokenUpdateMsg{count: chunk.TotalTokensUsed})
					} else if chunk.TokensUsed > 0 {
						m.program.Send(tokenUpdateMsg{count: chunk.TokensUsed})
					}
					return
				}

				if content != "" && !strings.HasPrefix(content, "\n") {
					content = "\n" + content
				}
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

		return streamStartMsg{input: input}
	}
}

func (m *StreamTUIModel) View() string {
	if !m.ready {
		return "Initializing Alex Code Agent..."
	}

	// Header
	header := headerStyle.Render("Alex Code Agent")

	// Update viewport content
	m.updateViewportContent()

	var parts []string
	parts = append(parts, header, "")
	parts = append(parts, m.viewport.View())
	parts = append(parts, "")

	// Processing status
	if m.processing {
		elapsed := m.execTimer.Duration.Truncate(time.Second)
		tokenDisplay := fmt.Sprintf("%d", m.tokenCount)
		
		mainText := processingStyle.Render("Wibbling… ")
		grayInfo := lipgloss.NewStyle().Foreground(mutedColor).Render(fmt.Sprintf("(%v · %s tokens)", elapsed, tokenDisplay))
		parts = append(parts, mainText+grayInfo)
	}

	// Input area
	if m.inputAtBottom {
		parts = append(parts, inputStyle.Render(m.textarea.View()))
	}

	// Shortcuts
	shortcuts := systemMsgStyle.Render("  Enter: send • Ctrl+I: input pos • PgUp/PgDn/Home/End: navigate")
	parts = append(parts, shortcuts)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *StreamTUIModel) cleanupKimiCache() {
	if m.agent == nil {
		return
	}

	sessionID, _ := m.agent.GetSessionID()
	if sessionID == "" {
		return
	}

	if err := llm.CleanupKimiCacheForSession(sessionID, m.config.GetLLMConfig()); err != nil {
		log.Printf("[DEBUG] TUI: Failed to cleanup Kimi cache: %v", err)
	}
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatToolOutput(title, content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= 1 {
		return fmt.Sprintf("%s%s", title, content)
	}

	indent := "   "
	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s%s\n", title, lines[0]))

	for i := 1; i < len(lines); i++ {
		result.WriteString(fmt.Sprintf("%s%s\n", indent, lines[i]))
	}

	return strings.TrimSuffix(result.String(), "\n")
}

// Run the stream TUI
func runModernTUI(agent *agent.ReactAgent, config *config.Manager) error {
	model := NewStreamTUIModel(agent, config)

	// Add initial system message
	model.addMessage(ChatMessage{
		Type:      "system",
		ChunkType: "system",
		Content:   "Press Ctrl+C to exit",
		Time:      time.Now(),
	})

	var program *tea.Program
	if isTTYAvailable() {
		program = tea.NewProgram(model, tea.WithAltScreen())
	} else {
		program = tea.NewProgram(model)
	}

	model.program = program

	_, err := program.Run()
	if err != nil {
		return fmt.Errorf("TUI initialization failed (no TTY available): %w\n\nTry running without TUI:\n  alex \"your prompt here\"", err)
	}
	return err
}

func isTTYAvailable() bool {
	if _, err := os.Open("/dev/tty"); err != nil {
		return false
	}
	return true
}