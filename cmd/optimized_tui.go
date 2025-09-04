package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
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

// ExecutionTimer tracks processing time
type ExecutionTimer struct {
	StartTime time.Time
	Duration  time.Duration
	Active    bool
}

// Message types for the optimized TUI
type (
	streamStartMsg    struct{ input string }
	streamContentMsg  struct{ content string }
	streamCompleteMsg struct{}
	errorOccurredMsg  struct{ err error }
	renderTickMsg     struct{}
	forceInitMsg      struct{}
)

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

// OptimizedMessage represents a message with cached rendering info
type OptimizedMessage struct {
	ID          string
	Type        string // "user", "assistant", "system", "processing", "error"
	ChunkType   string
	Content     string
	Time        time.Time
	// Caching fields
	RenderedContent string    // Cached rendered content
	RenderHash      uint64    // Hash of content + style for cache invalidation
	LastRendered    time.Time // When content was last rendered
	LineCount       int       // Number of lines this message occupies
	IsDirty         bool      // Whether content needs re-rendering
}

// IncrementalBuffer manages messages with smart caching and incremental updates
type IncrementalBuffer struct {
	messages       []*OptimizedMessage
	capacity       int
	totalMessages  int64 // Total messages ever added
	renderCache    map[string]string // Cache for expensive renders
	visibleRange   VisibleRange
	mu             sync.RWMutex
	messagePool    sync.Pool // Pool for message reuse
}

type VisibleRange struct {
	Start     int
	End       int
	TopOffset int // Pixel offset from top of first visible message
}

func NewIncrementalBuffer(capacity int) *IncrementalBuffer {
	return &IncrementalBuffer{
		messages:     make([]*OptimizedMessage, 0, capacity),
		capacity:     capacity,
		renderCache:  make(map[string]string),
		messagePool: sync.Pool{
			New: func() interface{} {
				return &OptimizedMessage{}
			},
		},
	}
}

func (ib *IncrementalBuffer) AddMessage(msgType, chunkType, content string) *OptimizedMessage {
	ib.mu.Lock()
	defer ib.mu.Unlock()

	// Get message from pool or create new
	msg := ib.messagePool.Get().(*OptimizedMessage)
	*msg = OptimizedMessage{
		ID:        fmt.Sprintf("msg_%d", ib.totalMessages),
		Type:      msgType,
		ChunkType: chunkType,
		Content:   content,
		Time:      time.Now(),
		IsDirty:   true,
	}

	// Handle buffer overflow with circular buffer
	if len(ib.messages) >= ib.capacity {
		// Return oldest message to pool
		oldest := ib.messages[0]
		ib.messagePool.Put(oldest)
		// Shift messages
		ib.messages = ib.messages[1:]
	}

	ib.messages = append(ib.messages, msg)
	ib.totalMessages++

	return msg
}

func (ib *IncrementalBuffer) UpdateMessage(msg *OptimizedMessage, content string) {
	ib.mu.Lock()
	defer ib.mu.Unlock()

	if msg.Content != content {
		msg.Content = content
		msg.IsDirty = true
		// Invalidate render cache for this message
		if msg.RenderedContent != "" {
			msg.RenderedContent = ""
			msg.RenderHash = 0
		}
	}
}

func (ib *IncrementalBuffer) GetVisibleMessages(viewportHeight, scrollOffset int) ([]*OptimizedMessage, bool) {
	ib.mu.RLock()
	defer ib.mu.RUnlock()

	if len(ib.messages) == 0 {
		return nil, false
	}

	// Calculate visible range based on estimated line heights
	avgLineHeight := 3 // Estimated lines per message
	startIdx := max(0, scrollOffset/avgLineHeight)
	endIdx := min(len(ib.messages), startIdx+viewportHeight/avgLineHeight+2) // +2 for buffer

	// Update visible range
	newRange := VisibleRange{
		Start: startIdx,
		End:   endIdx,
	}

	// Check if visible range changed
	rangeChanged := newRange.Start != ib.visibleRange.Start || newRange.End != ib.visibleRange.End
	ib.visibleRange = newRange

	return ib.messages[startIdx:endIdx], rangeChanged
}

// SmartRenderer handles intelligent rendering with caching
type SmartRenderer struct {
	width            int
	contentCache     map[string]string
	styleCache       map[string]lipgloss.Style
	lastRenderTime   time.Time
	renderThrottle   time.Duration
	mu               sync.RWMutex
}

func NewSmartRenderer() *SmartRenderer {
	return &SmartRenderer{
		contentCache:   make(map[string]string),
		styleCache:     make(map[string]lipgloss.Style),
		renderThrottle: 16 * time.Millisecond, // ~60fps
	}
}

func (sr *SmartRenderer) RenderMessage(msg *OptimizedMessage, width int) string {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Check if we need to throttle rendering
	if time.Since(sr.lastRenderTime) < sr.renderThrottle {
		if msg.RenderedContent != "" {
			return msg.RenderedContent
		}
	}

	// Generate cache key
	cacheKey := fmt.Sprintf("%s_%s_%d_%d", msg.ID, msg.Type, width, len(msg.Content))

	// Check cache first
	if !msg.IsDirty && msg.RenderedContent != "" {
		return msg.RenderedContent
	}

	// Check global cache
	if cached, exists := sr.contentCache[cacheKey]; exists && !msg.IsDirty {
		msg.RenderedContent = cached
		msg.IsDirty = false
		return cached
	}

	// Render content
	rendered := sr.renderMessageContent(msg, width)
	
	// Cache the result
	msg.RenderedContent = rendered
	msg.IsDirty = false
	msg.LastRendered = time.Now()
	sr.contentCache[cacheKey] = rendered
	sr.lastRenderTime = time.Now()

	// Calculate line count for layout
	msg.LineCount = strings.Count(rendered, "\n") + 1

	return rendered
}

func (sr *SmartRenderer) renderMessageContent(msg *OptimizedMessage, width int) string {
	// Efficient content processing with word wrapping
	processedContent := sr.processContent(msg.Content, width-8) // Account for padding

	// Apply styling based on message type
	switch msg.Type {
	case "user":
		return userMsgStyle.Render("🗨️ ") + processedContent
	case "assistant":
		prefix := assistantMsgStyle.Render("🤖 ")
		if msg.ChunkType == "tool_result" || msg.ChunkType == "tool_start" {
			return prefix + toolOutputStyle.Render(processedContent)
		}
		return prefix + assistantMsgStyle.Render(processedContent)
	case "system":
		prefix := "ℹ️ "
		if strings.Contains(msg.ChunkType, "search") {
			prefix = "🔍 "
		}
		return systemMsgStyle.Render(prefix + processedContent)
	case "processing":
		return processingStyle.Render("⚡ " + processedContent)
	case "error":
		return errorMsgStyle.Render("❌ " + processedContent)
	default:
		return processedContent
	}
}

func (sr *SmartRenderer) processContent(content string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80
	}

	lines := strings.Split(content, "\n")
	processedLines := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		if utf8.RuneCountInString(trimmed) <= maxWidth {
			processedLines = append(processedLines, trimmed)
		} else {
			// Efficient word wrapping
			wrapped := sr.wrapLine(trimmed, maxWidth)
			processedLines = append(processedLines, wrapped...)
		}
	}

	return strings.Join(processedLines, "\n")
}

func (sr *SmartRenderer) wrapLine(line string, maxWidth int) []string {
	if utf8.RuneCountInString(line) <= maxWidth {
		return []string{line}
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{line}
	}

	wrapped := make([]string, 0, 2)
	currentLine := strings.Builder{}
	currentLine.Grow(maxWidth + 10) // Pre-allocate

	for _, word := range words {
		wordLen := utf8.RuneCountInString(word)
		currentLen := utf8.RuneCountInString(currentLine.String())

		if currentLen+wordLen+1 > maxWidth && currentLen > 0 {
			wrapped = append(wrapped, currentLine.String())
			currentLine.Reset()
			currentLine.WriteString(word)
		} else {
			if currentLen > 0 {
				currentLine.WriteString(" ")
			}
			currentLine.WriteString(word)
		}
	}

	if currentLine.Len() > 0 {
		wrapped = append(wrapped, currentLine.String())
	}

	return wrapped
}

// OptimizedTUIModel - Improved model with smart rendering
type OptimizedTUIModel struct {
	// Core components
	textarea     textarea.Model
	viewport     viewport.Model
	buffer       *IncrementalBuffer
	renderer     *SmartRenderer
	
	// State management
	processing   bool
	agent        *agent.ReactAgent
	config       *config.Manager
	width        int
	height       int
	ready        bool
	
	// Performance tracking
	execTimer    ExecutionTimer
	program      *tea.Program
	currentMsg   *OptimizedMessage
	tokenCount   int
	
	// Rendering optimization
	renderPending   bool
	inputAtBottom   bool
	
	// Input handling
	inputQueue      []string
	currentInput    string
}

func NewOptimizedTUIModel(agent *agent.ReactAgent, config *config.Manager) *OptimizedTUIModel {
	// Configure textarea with optimizations
	ta := textarea.New()
	ta.Placeholder = ""
	ta.Focus()
	ta.Prompt = "> "
	ta.CharLimit = 4000 // Increased limit
	ta.SetHeight(1)
	ta.ShowLineNumbers = false
	ta.KeyMap.InsertNewline.SetEnabled(true)

	// Configure viewport with high performance mode
	vp := viewport.New(80, 24)
	vp.MouseWheelEnabled = true
	vp.MouseWheelDelta = 3

	return &OptimizedTUIModel{
		textarea:      ta,
		viewport:      vp,
		buffer:        NewIncrementalBuffer(50000), // Increased capacity
		renderer:      NewSmartRenderer(),
		agent:         agent,
		config:        config,
		ready:         false,
		inputAtBottom: true,
		inputQueue:    make([]string, 0, 10),
	}
}

func (m *OptimizedTUIModel) Init() tea.Cmd {
	m.initializeDefaultDimensions()
	return tea.Batch(
		textarea.Blink, 
		m.startRenderTicker(),
		func() tea.Msg { return forceInitMsg{} },
	)
}

func (m *OptimizedTUIModel) initializeDefaultDimensions() {
	defaultWidth, defaultHeight := 80, 24
	m.updateDimensions(defaultWidth, defaultHeight)
	m.ready = true
}

func (m *OptimizedTUIModel) updateDimensions(width, height int) {
	m.width = width
	m.height = height
	m.textarea.SetWidth(max(20, width-4))
	m.viewport.Width = width
	m.viewport.Height = m.calculateViewportHeight()
	m.renderer.width = width
}

func (m *OptimizedTUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var tiCmd, vpCmd tea.Cmd
	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.updateDimensions(msg.Width, msg.Height)
		m.scheduleRender()

	case tea.KeyMsg:
		return m.handleKeyInput(msg)

	case streamContentMsg:
		return m.handleStreamContent(msg)

	case renderTickMsg:
		if m.renderPending {
			m.renderView()
			m.renderPending = false
		}
		return m, m.startRenderTicker()

	case forceInitMsg:
		if !m.ready {
			m.initializeDefaultDimensions()
		}
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *OptimizedTUIModel) handleStreamContent(msg streamContentMsg) (tea.Model, tea.Cmd) {
	if m.currentMsg != nil {
		m.buffer.UpdateMessage(m.currentMsg, m.currentMsg.Content+msg.content)
		m.scheduleRender()
	}
	return m, nil
}

func (m *OptimizedTUIModel) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Type == tea.KeyCtrlC:
		m.cleanupKimiCache()
		return m, tea.Quit

	case msg.Type == tea.KeyEnter && !msg.Alt:
		return m.handleSubmit()

	case msg.String() == "pgup":
		m.viewport.ScrollUp(5)
		m.scheduleRender()
		return m, nil

	case msg.String() == "pgdown":
		m.viewport.ScrollDown(5)
		m.scheduleRender()
		return m, nil
	}

	return m, nil
}

func (m *OptimizedTUIModel) handleSubmit() (tea.Model, tea.Cmd) {
	input := strings.TrimSpace(m.textarea.Value())
	if input == "" {
		return m, nil
	}

	m.textarea.Reset()
	
	// Add user message
	_ = m.buffer.AddMessage("user", "user_input", input)
	m.scheduleRender()

	if !m.processing {
		return m.startProcessing(input)
	}

	// Queue message if processing
	m.inputQueue = append(m.inputQueue, input)
	_ = m.buffer.AddMessage("system", "system", 
		fmt.Sprintf("📬 Message queued (%d in queue)", len(m.inputQueue)))
	m.scheduleRender()

	return m, nil
}

func (m *OptimizedTUIModel) startProcessing(input string) (tea.Model, tea.Cmd) {
	m.processing = true
	m.currentInput = input
	m.tokenCount = 0
	m.execTimer = ExecutionTimer{
		StartTime: time.Now(),
		Active:    true,
	}

	// Create assistant message for streaming
	m.currentMsg = m.buffer.AddMessage("assistant", "llm_content", "")
	m.scheduleRender()

	return m, m.processUserInput(input)
}

func (m *OptimizedTUIModel) scheduleRender() {
	m.renderPending = true
}

func (m *OptimizedTUIModel) renderView() {
	visibleMessages, rangeChanged := m.buffer.GetVisibleMessages(
		m.viewport.Height, 
		m.viewport.YOffset,
	)

	if !rangeChanged && !m.hasContentChanged(visibleMessages) {
		return // Skip render if nothing changed
	}

	// Render visible messages efficiently
	var contentParts []string
	for i, msg := range visibleMessages {
		if i > 0 {
			contentParts = append(contentParts, "")
		}
		rendered := m.renderer.RenderMessage(msg, m.width)
		contentParts = append(contentParts, rendered)
	}

	content := strings.Join(contentParts, "\n")
	m.viewport.SetContent(content)
}

func (m *OptimizedTUIModel) hasContentChanged(messages []*OptimizedMessage) bool {
	for _, msg := range messages {
		if msg.IsDirty {
			return true
		}
	}
	return false
}

func (m *OptimizedTUIModel) calculateViewportHeight() int {
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

	return max(5, m.height-fixedHeight)
}

// Render ticker for smooth updates
func (m *OptimizedTUIModel) startRenderTicker() tea.Cmd {
	return tea.Tick(16*time.Millisecond, func(t time.Time) tea.Msg {
		return renderTickMsg{}
	})
}

func (m *OptimizedTUIModel) processUserInput(input string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		
		go func() {
			streamCallback := func(chunk agent.StreamChunk) {
				switch chunk.Type {
				case "llm_content":
					m.program.Send(streamContentMsg{content: chunk.Content})
				case "complete":
					m.program.Send(streamCompleteMsg{})
				}
			}

			err := m.agent.ProcessMessageStream(ctx, input, m.config.GetConfig(), streamCallback)
			if err != nil {
				m.program.Send(errorOccurredMsg{err: err})
			}
		}()

		return streamStartMsg{input: input}
	}
}

func (m *OptimizedTUIModel) View() string {
	if !m.ready {
		return "Initializing Optimized Alex Agent..."
	}

	// Ensure view is rendered
	if m.renderPending {
		m.renderView()
		m.renderPending = false
	}

	header := headerStyle.Render("🚀 Alex Agent (Optimized)")
	var parts []string
	parts = append(parts, header, "")
	parts = append(parts, m.viewport.View())

	// Processing status
	if m.processing {
		elapsed := m.execTimer.Duration.Truncate(time.Second)
		status := processingStyle.Render(fmt.Sprintf("Processing… (%v • %d tokens)", elapsed, m.tokenCount))
		parts = append(parts, "", status)
	}

	// Input area
	if m.inputAtBottom {
		parts = append(parts, "", inputStyle.Render(m.textarea.View()))
	}

	// Shortcuts
	shortcuts := systemMsgStyle.Render("Enter: send • PgUp/PgDn: scroll • Ctrl+C: exit")
	parts = append(parts, shortcuts)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m *OptimizedTUIModel) cleanupKimiCache() {
	if m.agent == nil {
		return
	}

	sessionID, _ := m.agent.GetSessionID()
	if sessionID == "" {
		return
	}

	if err := llm.CleanupKimiCacheForSession(sessionID, m.config.GetLLMConfig()); err != nil {
		log.Printf("Warning: Kimi cache cleanup failed during TUI shutdown: %v", err)
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

func isTTYAvailable() bool {
	if _, err := os.Open("/dev/tty"); err != nil {
		return false
	}
	return true
}

// RunOptimizedTUI launches the optimized TUI
func RunOptimizedTUI(agent *agent.ReactAgent, config *config.Manager) error {
	model := NewOptimizedTUIModel(agent, config)

	// Add welcome message
	model.buffer.AddMessage("system", "system", "🚀 Welcome to Optimized Alex Agent!")

	var program *tea.Program
	if isTTYAvailable() {
		program = tea.NewProgram(model, tea.WithAltScreen())
	} else {
		program = tea.NewProgram(model)
	}

	model.program = program

	_, err := program.Run()
	return err
}