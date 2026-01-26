package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	"alex/internal/output"
	"alex/internal/tools/builtin"
	id "alex/internal/utils/id"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

const (
	tuiAgentName   = "alex"
	tuiAgentIndent = "  "
	typewriterTickDelay = 6 * time.Millisecond
)

type tuiAgentEventMsg struct {
	event agent.AgentEvent
}

type tuiTaskCompleteMsg struct {
	result *agent.TaskResult
	err    error
}

type tuiSessionReadyMsg struct {
	sessionID string
}

type tuiTypewriterTickMsg struct{}

type bubbleChatUI struct {
	container *Container

	baseCtx    context.Context
	baseCancel context.CancelFunc
	program    *tea.Program

	sessionID string
	startTime time.Time

	width  int
	height int

	viewport viewport.Model
	input    textinput.Model

	renderer *output.CLIRenderer

	transcript strings.Builder
	follow     bool
	imeMode    bool
	imeBuffer  []rune

	running             bool
	cancelCurrentTurn   context.CancelFunc
	streamedCurrentTurn bool

	activeTools map[string]ToolInfo
	subagents   *SubagentDisplay

	mdBuffer                        *markdownStreamBuffer
	lastStreamChunkEndedWithNewline bool
	assistantHeaderPrinted          bool

	statusLine string
	typewriterQueue  []rune
	typewriterActive bool
}

func RunBubbleChatUI(container *Container) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("not running in a TTY")
	}

	baseCtx := context.Background()
	coreOutCtx := &types.OutputContext{
		Level:   types.LevelCore,
		AgentID: "core",
		Verbose: container.Runtime.Verbose,
	}
	baseCtx = types.WithOutputContext(baseCtx, coreOutCtx)
	baseCtx, baseCancel := context.WithCancel(baseCtx)

	model := newBubbleChatUI(container, baseCtx, baseCancel)
	program := tea.NewProgram(model, tea.WithAltScreen())
	model.program = program
	_, err := program.Run()
	baseCancel()
	return err
}

func newBubbleChatUI(container *Container, baseCtx context.Context, baseCancel context.CancelFunc) *bubbleChatUI {
	vp := viewport.New(0, 0)
	vp.MouseWheelEnabled = true
	vp.KeyMap.PageDown.SetEnabled(true)
	vp.KeyMap.PageUp.SetEnabled(true)
	vp.KeyMap.HalfPageDown.SetEnabled(true)
	vp.KeyMap.HalfPageUp.SetEnabled(true)

	input := textinput.New()
	input.Prompt = styleBoldGreen.Render("❯ ")
	input.Placeholder = "Type a message…"
	input.Focus()

	return &bubbleChatUI{
		container:   container,
		baseCtx:     baseCtx,
		baseCancel:  baseCancel,
		startTime:   time.Now(),
		viewport:    vp,
		input:       input,
		renderer:    output.NewCLIRenderer(container.Runtime.Verbose),
		follow:      container.Runtime.FollowTranscript,
		imeMode:     shouldUseIMEInput(runtimeEnvLookup()),
		activeTools: make(map[string]ToolInfo),
		subagents:   NewSubagentDisplay(),
		mdBuffer:    newMarkdownStreamBuffer(),
	}
}

func (m *bubbleChatUI) Init() tea.Cmd {
	m.appendSystemCard()
	return nil
}

func (m *bubbleChatUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.onWindowSize(msg)
		return m, nil

	case tea.KeyMsg:
		return m.onKey(msg)

	case tuiSessionReadyMsg:
		m.sessionID = msg.sessionID
		return m, nil

	case tuiAgentEventMsg:
		return m, m.handleAgentEvent(msg.event)

	case tuiTaskCompleteMsg:
		m.flushTypewriter()
		m.running = false
		if m.cancelCurrentTurn != nil {
			m.cancelCurrentTurn()
		}
		m.cancelCurrentTurn = nil

		if msg.err != nil {
			if errors.Is(msg.err, context.Canceled) {
				m.statusLine = styleSystem.Render("⏹️ canceled")
				return m, nil
			}
			m.appendAgentLine(styleError.Render(fmt.Sprintf("✗ Error: %v", msg.err)))
			m.statusLine = styleError.Render("✗ failed")
			return m, nil
		}

		if msg.result != nil {
			m.statusLine = styleGray.Render(fmt.Sprintf("✓ Done • %d iterations • %d tokens", msg.result.Iterations, msg.result.TokensUsed))
			if !m.streamedCurrentTurn && strings.TrimSpace(msg.result.Answer) != "" {
				m.appendAgentRaw(m.renderer.RenderMarkdownStreamChunk(msg.result.Answer, true))
			}
		}
		return m, nil

	case tuiTypewriterTickMsg:
		return m, m.onTypewriterTick()
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *bubbleChatUI) View() string {
	header := m.renderHeader()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		m.viewport.View(),
		footer,
	)
}

func (m *bubbleChatUI) renderHeader() string {
	title := styleBold.Render(styleGreen.Render(tuiAgentName))

	parts := []string{title}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		parts = append(parts, styleGray.Render(filepathBase(cwd)))
	}
	if branch := currentGitBranch(); branch != "" {
		parts = append(parts, styleGray.Render("git:"+branch))
	}
	if m.sessionID != "" {
		parts = append(parts, styleGray.Render("session:"+shortSessionID(m.sessionID)))
	}
	if m.running {
		parts = append(parts, styleYellow.Render("running"))
	}

	line := strings.Join(parts, styleGray.Render(" • "))
	return lipgloss.NewStyle().
		Padding(0, 1).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("8")).
		Render(line)
}

func (m *bubbleChatUI) renderFooter() string {
	hints := []string{
		"Enter send",
		"Ctrl+C cancel/quit",
		"Ctrl+L clear",
		"PgUp/PgDn scroll",
		"End follow",
	}

	if m.statusLine == "" {
		m.statusLine = styleGray.Render(strings.Join(hints, " • "))
	}

	inputLine := m.input.View()
	status := m.statusLine

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().
			Padding(0, 1).
			Border(lipgloss.NormalBorder(), true, false, false, false).
			BorderForeground(lipgloss.Color("8")).
			Render(inputLine),
		lipgloss.NewStyle().Padding(0, 1).Render(status),
	)
}

func (m *bubbleChatUI) onWindowSize(msg tea.WindowSizeMsg) {
	m.width = msg.Width
	m.height = msg.Height

	// Header/footer render with borders, so they take more than one row.
	headerHeight := 2
	footerHeight := 3
	bodyHeight := msg.Height - headerHeight - footerHeight
	if bodyHeight < 1 {
		bodyHeight = 1
	}

	m.viewport.Width = msg.Width
	m.viewport.Height = bodyHeight
	m.input.Width = maxInt(20, msg.Width-4)
}

func (m *bubbleChatUI) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		if m.running && m.cancelCurrentTurn != nil {
			m.statusLine = styleSystem.Render("⏹️ cancel requested…")
			m.cancelCurrentTurn()
			return m, nil
		}
		m.shutdown()
		return m, tea.Quit

	case "ctrl+l":
		m.transcript.Reset()
		m.viewport.SetContent("")
		m.appendSystemCard()
		m.statusLine = styleGray.Render("cleared")
		return m, nil

	case "end":
		m.follow = true
		m.viewport.GotoBottom()
		return m, nil

	case "pgup", "pgdown", "up", "down":
		m.follow = false
	}

	if msg.Type == tea.KeyEnter {
		return m.onSubmit()
	}

	if m.imeMode {
		updated, handled := applyIMEKey(m.imeBuffer, msg)
		if handled {
			m.imeBuffer = updated
			m.input.SetValue(string(m.imeBuffer))
			m.input.CursorEnd()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	m.input, _ = m.input.Update(msg)
	return m, cmd
}

func (m *bubbleChatUI) onSubmit() (tea.Model, tea.Cmd) {
	if m.running {
		m.statusLine = styleGray.Render("busy (Ctrl+C to cancel)")
		return m, nil
	}

	task := strings.TrimSpace(m.input.Value())
	if task == "" {
		return m, nil
	}

	switch task {
	case "/quit", "/exit":
		m.shutdown()
		return m, tea.Quit
	case "/clear":
		m.transcript.Reset()
		m.viewport.SetContent("")
		m.appendSystemCard()
		m.input.SetValue("")
		m.imeBuffer = nil
		m.statusLine = styleGray.Render("cleared")
		return m, nil
	}

	m.appendUserMessage(task)
	m.input.SetValue("")
	m.imeBuffer = nil
	m.streamedCurrentTurn = false
	m.assistantHeaderPrinted = false
	m.running = true
	m.statusLine = styleGray.Render("running…")

	taskCtx, cancel := context.WithCancel(m.baseCtx)
	m.cancelCurrentTurn = cancel

	sessionID := m.sessionID
	return m, m.runTaskCmd(taskCtx, task, sessionID)
}

func (m *bubbleChatUI) runTaskCmd(ctx context.Context, task string, sessionID string) tea.Cmd {
	return func() tea.Msg {
		if sessionID == "" {
			session, err := m.container.AgentCoordinator.GetSession(ctx, "")
			if err != nil {
				return tuiTaskCompleteMsg{err: fmt.Errorf("create session: %w", err)}
			}
			sessionID = session.ID
			m.send(tuiSessionReadyMsg{sessionID: sessionID})
		}

		taskCtx := id.WithSessionID(ctx, sessionID)
		taskCtx = id.WithTaskID(taskCtx, id.NewTaskID())
		taskCtx = builtin.WithApprover(taskCtx, cliApproverForSession(sessionID))
		taskCtx = builtin.WithAutoApprove(taskCtx, false)

		listener := newBubbleTeaListener(ctx, m.container.Runtime.Verbose, func(e agent.AgentEvent) {
			m.send(tuiAgentEventMsg{event: e})
		})
		taskCtx = builtin.WithParentListener(taskCtx, listener)

		result, err := m.container.AgentCoordinator.ExecuteTask(taskCtx, task, sessionID, listener)
		if err != nil {
			return tuiTaskCompleteMsg{err: err}
		}
		return tuiTaskCompleteMsg{result: result}
	}
}

func (m *bubbleChatUI) send(msg tea.Msg) {
	if msg == nil {
		return
	}
	if m.baseCtx != nil && m.baseCtx.Err() != nil {
		return
	}
	if m.program == nil {
		return
	}
	m.program.Send(msg)
}

func (m *bubbleChatUI) shutdown() {
	if m.cancelCurrentTurn != nil {
		m.cancelCurrentTurn()
		m.cancelCurrentTurn = nil
	}
	if m.baseCancel != nil {
		m.baseCancel()
	}
}

func (m *bubbleChatUI) appendSystemCard() {
	cwd, _ := os.Getwd()
	displayCwd := cwd
	if len(displayCwd) > 60 {
		displayCwd = "..." + displayCwd[len(displayCwd)-57:]
	}
	lines := []string{
		fmt.Sprintf("%s %s", styleGray.Render("cwd:"), displayCwd),
	}
	if branch := currentGitBranch(); branch != "" {
		lines = append(lines, fmt.Sprintf("%s %s", styleGray.Render("git:"), styleGreen.Render(branch)))
	}
	lines = append(lines, styleGray.Render("commands: /quit, /exit, /clear"))

	cardWidth := 76
	if m.width > 0 {
		cardWidth = minInt(100, m.width-4)
		if cardWidth < 48 {
			cardWidth = 48
		}
	}

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Padding(0, 1).
		Width(cardWidth)

	title := fmt.Sprintf("%s %s", styleBold.Render(styleGreen.Render(tuiAgentName)), styleGray.Render("— interactive"))
	m.appendLine(card.Render(title + "\n" + strings.Join(lines, "\n")))
}

func (m *bubbleChatUI) appendUserMessage(content string) {
	header := styleBold.Render(styleGreen.Render("You"))
	body := lipgloss.NewStyle().PaddingLeft(2).Render(content)
	m.appendLine(header)
	m.appendLine(body)
	m.appendLine("")
}

func (m *bubbleChatUI) ensureAgentHeader() {
	if m.assistantHeaderPrinted {
		return
	}
	m.assistantHeaderPrinted = true
	header := styleBold.Render(styleBoldCyan.Render(tuiAgentName))
	m.appendLine(header)
}

func (m *bubbleChatUI) appendAgentLine(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	m.ensureAgentHeader()
	m.appendLine(indentBlock(line, tuiAgentIndent))
}

func (m *bubbleChatUI) appendAgentRaw(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}

	// Avoid an empty spacer line right after the agent header when the renderer
	// happens to emit leading newlines (common with markdown renderers).
	if !m.assistantHeaderPrinted {
		content = strings.TrimLeft(content, "\n")
		if strings.TrimSpace(content) == "" {
			return
		}
	}

	m.ensureAgentHeader()
	m.appendRaw(indentBlock(content, tuiAgentIndent))
}

func (m *bubbleChatUI) appendAgentStreamRaw(content string) {
	if content == "" {
		return
	}

	// Avoid an empty spacer line right after the agent header when the renderer
	// happens to emit leading newlines (common with markdown renderers).
	if !m.assistantHeaderPrinted {
		content = strings.TrimLeft(content, "\n")
		if content == "" {
			return
		}
	}

	m.ensureAgentHeader()
	m.appendRaw(indentBlock(content, tuiAgentIndent))
}

func (m *bubbleChatUI) appendLine(content string) {
	if content == "" {
		m.transcript.WriteString("\n")
	} else {
		if !strings.HasSuffix(content, "\n") {
			m.transcript.WriteString(content)
			m.transcript.WriteString("\n")
		} else {
			m.transcript.WriteString(content)
		}
	}
	m.viewport.SetContent(m.transcript.String())
	if m.follow {
		m.viewport.GotoBottom()
	}
}

func (m *bubbleChatUI) handleAgentEvent(event agent.AgentEvent) tea.Cmd {
	if event == nil {
		return nil
	}

	// Subtask wrapper events.
	if subtaskEvent, ok := event.(*builtin.SubtaskEvent); ok {
		m.flushTypewriter()
		lines := m.subagents.Handle(normalizeSubtaskEvent(subtaskEvent))
		for _, line := range lines {
			m.appendAgentRaw(line)
		}
		return nil
	}

	// New contract: workflow envelopes.
	if env, ok := event.(*domain.WorkflowEventEnvelope); ok {
		return m.handleEnvelopeEvent(env)
	}

	return nil
}

func normalizeSubtaskEvent(event *builtin.SubtaskEvent) *builtin.SubtaskEvent {
	if event == nil || event.OriginalEvent == nil {
		return event
	}

	env, ok := event.OriginalEvent.(*domain.WorkflowEventEnvelope)
	if !ok || env == nil {
		return event
	}

	var converted agent.AgentEvent
	switch env.Event {
	case "workflow.tool.started":
		converted = envelopeToToolStarted(env)
	case "workflow.tool.completed":
		converted = envelopeToToolCompleted(env)
	case "workflow.result.final":
		converted = envelopeToResultFinal(env)
	case "workflow.node.failed":
		converted = envelopeToNodeFailed(env)
	default:
		return event
	}

	if converted == nil {
		return event
	}

	copy := *event
	copy.OriginalEvent = converted
	return &copy
}

func (m *bubbleChatUI) handleEnvelopeEvent(env *domain.WorkflowEventEnvelope) tea.Cmd {
	if env == nil {
		return nil
	}

	switch env.Event {
	case "workflow.node.output.delta":
		if evt := envelopeToNodeOutputDelta(env); evt != nil {
			return m.onAssistantMessage(evt)
		}
	case "workflow.tool.started":
		if evt := envelopeToToolStarted(env); evt != nil {
			m.onToolCallStart(evt)
		}
	case "workflow.tool.completed":
		if evt := envelopeToToolCompleted(env); evt != nil {
			m.onToolCallComplete(evt)
		}
	case "workflow.node.failed":
		if evt := envelopeToNodeFailed(env); evt != nil {
			m.flushTypewriter()
			m.appendAgentLine(styleError.Render(fmt.Sprintf("✗ Error: %v", evt.Error)))
		}
	}

	return nil
}

func (m *bubbleChatUI) onAssistantMessage(event *domain.WorkflowNodeOutputDeltaEvent) tea.Cmd {
	if event == nil {
		return nil
	}

	if strings.TrimSpace(event.Delta) != "" || event.Final {
		m.streamedCurrentTurn = true
	}

	var cmds []tea.Cmd
	if event.Delta != "" {
		for _, chunk := range m.mdBuffer.Append(event.Delta) {
			if chunk.content == "" {
				continue
			}
			rendered := m.renderer.RenderMarkdownStreamChunk(chunk.content, chunk.completeLine)
			if cmd := m.enqueueTypewriter(rendered); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.lastStreamChunkEndedWithNewline = strings.HasSuffix(rendered, "\n")
		}
	}

	if event.Final {
		trailing := m.mdBuffer.FlushAll()
		if trailing != "" {
			rendered := m.renderer.RenderMarkdownStreamChunk(trailing, false)
			if cmd := m.enqueueTypewriter(rendered); cmd != nil {
				cmds = append(cmds, cmd)
			}
			if strings.HasSuffix(rendered, "\n") {
				m.lastStreamChunkEndedWithNewline = true
			} else {
				m.lastStreamChunkEndedWithNewline = false
				if cmd := m.enqueueTypewriter("\n"); cmd != nil {
					cmds = append(cmds, cmd)
				}
				m.lastStreamChunkEndedWithNewline = true
			}
		} else if !m.lastStreamChunkEndedWithNewline {
			if cmd := m.enqueueTypewriter("\n"); cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.lastStreamChunkEndedWithNewline = true
		}
		m.renderer.ResetMarkdownStreamState()
	}

	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

func (m *bubbleChatUI) onToolCallStart(event *domain.WorkflowToolStartedEvent) {
	if event == nil {
		return
	}

	m.flushTypewriter()

	if m.streamedCurrentTurn && !m.lastStreamChunkEndedWithNewline {
		m.appendRaw("\n")
		m.lastStreamChunkEndedWithNewline = true
	}

	m.activeTools[event.CallID] = ToolInfo{
		Name:      event.ToolName,
		StartTime: event.Timestamp(),
	}

	outCtx := &types.OutputContext{
		Level:        event.GetAgentLevel(),
		Category:     output.CategorizeToolName(event.ToolName),
		AgentID:      string(event.GetAgentLevel()),
		Verbose:      m.container.Runtime.Verbose,
		SessionID:    event.GetSessionID(),
		TaskID:       event.GetTaskID(),
		ParentTaskID: event.GetParentTaskID(),
	}

	rendered := m.renderer.RenderToolCallStart(outCtx, event.ToolName, event.Arguments)
	m.appendAgentRaw(rendered)
}

func (m *bubbleChatUI) onToolCallComplete(event *domain.WorkflowToolCompletedEvent) {
	if event == nil {
		return
	}

	m.flushTypewriter()

	info, exists := m.activeTools[event.CallID]
	if !exists {
		return
	}

	outCtx := &types.OutputContext{
		Level:        event.GetAgentLevel(),
		Category:     output.CategorizeToolName(info.Name),
		AgentID:      string(event.GetAgentLevel()),
		Verbose:      m.container.Runtime.Verbose,
		SessionID:    event.GetSessionID(),
		TaskID:       event.GetTaskID(),
		ParentTaskID: event.GetParentTaskID(),
	}

	duration := time.Since(info.StartTime)
	rendered := m.renderer.RenderToolCallComplete(outCtx, info.Name, event.Result, event.Error, duration)
	m.appendAgentRaw(rendered)

	delete(m.activeTools, event.CallID)
}

func (m *bubbleChatUI) appendRaw(content string) {
	if content == "" {
		return
	}
	m.transcript.WriteString(content)
	m.viewport.SetContent(m.transcript.String())
	if m.follow {
		m.viewport.GotoBottom()
	}
}

func (m *bubbleChatUI) enqueueTypewriter(content string) tea.Cmd {
	if content == "" {
		return nil
	}
	m.typewriterQueue = append(m.typewriterQueue, []rune(content)...)
	if !m.typewriterActive {
		m.typewriterActive = true
		return typewriterTick()
	}
	return nil
}

func (m *bubbleChatUI) flushTypewriter() {
	if len(m.typewriterQueue) == 0 {
		m.typewriterActive = false
		return
	}
	m.appendAgentStreamRaw(string(m.typewriterQueue))
	m.typewriterQueue = nil
	m.typewriterActive = false
}

func (m *bubbleChatUI) onTypewriterTick() tea.Cmd {
	if len(m.typewriterQueue) == 0 {
		m.typewriterActive = false
		return nil
	}
	r := m.typewriterQueue[0]
	m.typewriterQueue = m.typewriterQueue[1:]
	m.appendAgentStreamRaw(string(r))
	if len(m.typewriterQueue) == 0 {
		m.typewriterActive = false
		return nil
	}
	return typewriterTick()
}

func typewriterTick() tea.Cmd {
	return tea.Tick(typewriterTickDelay, func(time.Time) tea.Msg {
		return tuiTypewriterTickMsg{}
	})
}

func indentBlock(content string, prefix string) string {
	if content == "" || prefix == "" {
		return content
	}

	hasTrailingNewline := strings.HasSuffix(content, "\n")
	content = strings.TrimSuffix(content, "\n")

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = prefix + line
	}

	out := strings.Join(lines, "\n")
	if hasTrailingNewline {
		out += "\n"
	}
	return out
}

type bubbleTeaListener struct {
	ctx     context.Context
	verbose bool
	onEvent func(agent.AgentEvent)
}

func newBubbleTeaListener(ctx context.Context, verbose bool, onEvent func(agent.AgentEvent)) *bubbleTeaListener {
	return &bubbleTeaListener{
		ctx:     ctx,
		verbose: verbose,
		onEvent: onEvent,
	}
}

func (l *bubbleTeaListener) OnEvent(event agent.AgentEvent) {
	if event == nil || l.onEvent == nil {
		return
	}
	if l.ctx != nil && l.ctx.Err() != nil {
		return
	}
	l.onEvent(event)
}

func filepathBase(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func shortSessionID(sessionID string) string {
	parts := strings.Split(sessionID, "-")
	if len(parts) == 0 {
		return sessionID
	}
	return parts[len(parts)-1]
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
