package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	"alex/internal/output"
	"alex/internal/tools/builtin/orchestration"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"

	"github.com/charmbracelet/lipgloss"
	"github.com/gdamore/tcell/v2"
	"github.com/muesli/termenv"
	"github.com/rivo/tview"
	"golang.org/x/term"
)

const (
	tuiAgentName   = "alex"
	tuiAgentIndent = "  "
)

type plainMarkdownRenderer struct{}

func (plainMarkdownRenderer) Render(content string) (string, error) {
	return content, nil
}

type tviewChatUI struct {
	container *Container

	app        *tview.Application
	outputView *tview.TextView
	statusView *tview.TextView
	inputField *tview.InputField
	layout     *tview.Flex

	outputWriter io.Writer

	baseCtx    context.Context
	baseCancel context.CancelFunc

	sessionID string
	startTime time.Time

	renderer *output.CLIRenderer

	running           bool
	cancelCurrentTurn context.CancelFunc
	statusLine        string

	activeTools map[string]ToolInfo
	subagents   *SubagentDisplay

	mdBuffer                        *markdownStreamBuffer
	streamedCurrentTurn             bool
	assistantHeaderPrinted          bool
	lastStreamChunkEndedWithNewline bool
	streamBuffer                    strings.Builder
}

func RunTUIView(container *Container) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}

	if !term.IsTerminal(int(os.Stdout.Fd())) || !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("not running in a TTY")
	}

	lipgloss.SetColorProfile(termenv.Ascii)

	baseCtx := context.Background()
	coreOutCtx := &types.OutputContext{
		Level:   types.LevelCore,
		AgentID: "core",
		Verbose: container.Runtime.Verbose,
	}
	baseCtx = types.WithOutputContext(baseCtx, coreOutCtx)
	baseCtx, baseCancel := context.WithCancel(baseCtx)

	ui := newTUIView(container, baseCtx, baseCancel)
	if err := ui.run(); err != nil {
		baseCancel()
		return err
	}
	baseCancel()
	return nil
}

func newTUIView(container *Container, baseCtx context.Context, baseCancel context.CancelFunc) *tviewChatUI {
	app := tview.NewApplication()

	outputView := tview.NewTextView().
		SetDynamicColors(false).
		SetWrap(true).
		SetWordWrap(true)
	outputView.SetScrollable(true)

	statusView := tview.NewTextView().SetWrap(false)

	inputField := tview.NewInputField().
		SetLabel("❯ ").
		SetFieldWidth(0)
	inputField.SetLabelColor(tcell.ColorGreen)

	footer := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(statusView, 1, 0, false).
		AddItem(inputField, 1, 0, true)

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(outputView, 0, 1, false).
		AddItem(footer, 2, 0, true)

	ui := &tviewChatUI{
		container:    container,
		app:          app,
		outputView:   outputView,
		statusView:   statusView,
		inputField:   inputField,
		layout:       layout,
		outputWriter: outputView,
		baseCtx:      baseCtx,
		baseCancel:   baseCancel,
		startTime:    time.Now(),
		renderer:     output.NewCLIRendererWithMarkdown(container.Runtime.Verbose, plainMarkdownRenderer{}),
		activeTools:  make(map[string]ToolInfo),
		subagents:    NewSubagentDisplay(),
		mdBuffer:     newMarkdownStreamBuffer(),
	}

	inputField.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		ui.onSubmit()
	})

	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return ui.handleGlobalKey(event)
	})

	app.SetRoot(layout, true).SetFocus(inputField)

	app.QueueUpdateDraw(func() {
		ui.appendSystemCard()
	})

	return ui
}

func (ui *tviewChatUI) run() error {
	if ui.app == nil {
		return fmt.Errorf("tview application missing")
	}
	return ui.app.Run()
}

func (ui *tviewChatUI) handleGlobalKey(event *tcell.EventKey) *tcell.EventKey {
	if event == nil {
		return event
	}

	switch event.Key() {
	case tcell.KeyCtrlC:
		ui.onCancelOrExit()
		return nil
	case tcell.KeyCtrlL:
		ui.clearTranscript()
		ui.setStatusLine(styleGray.Render("cleared"))
		return nil
	default:
		return event
	}
}

func (ui *tviewChatUI) onCancelOrExit() {
	if ui.running && ui.cancelCurrentTurn != nil {
		ui.setStatusLine(styleSystem.Render("⏹️ cancel requested…"))
		ui.cancelCurrentTurn()
		return
	}
	ui.shutdown()
	if ui.app != nil {
		ui.app.Stop()
	}
}

func (ui *tviewChatUI) onSubmit() {
	if ui.running {
		ui.setStatusLine(styleGray.Render("busy (Ctrl+C to cancel)"))
		return
	}

	task := ui.inputField.GetText()
	cmd := parseUserCommand(task)

	switch cmd.kind {
	case commandEmpty:
		return
	case commandQuit:
		ui.shutdown()
		if ui.app != nil {
			ui.app.Stop()
		}
		return
	case commandClear:
		ui.clearTranscript()
		ui.inputField.SetText("")
		ui.setStatusLine(styleGray.Render("cleared"))
		return
	case commandRun:
		ui.inputField.SetText("")
		ui.appendUserMessage(cmd.task)
		ui.streamedCurrentTurn = false
		ui.assistantHeaderPrinted = false
		ui.running = true
		ui.setStatusLine(styleGray.Render("running…"))

		ctx, cancel := context.WithCancel(ui.baseCtx)
		ui.cancelCurrentTurn = cancel
		ui.startTask(ctx, cmd.task)
		return
	default:
		return
	}
}

func (ui *tviewChatUI) startTask(ctx context.Context, task string) {
	go func() {
		sessionID := ui.sessionID
		if sessionID == "" {
			session, err := ui.container.AgentCoordinator.GetSession(ctx, "")
			if err != nil {
				ui.queue(func() {
					ui.finishTask(nil, fmt.Errorf("create session: %w", err))
				})
				return
			}
			sessionID = session.ID
			ui.queue(func() {
				ui.sessionID = sessionID
			})
		}

		taskCtx := id.WithSessionID(ctx, sessionID)
		taskCtx = id.WithTaskID(taskCtx, id.NewTaskID())
		taskCtx = shared.WithApprover(taskCtx, cliApproverForSession(sessionID))
		taskCtx = shared.WithAutoApprove(taskCtx, false)

		listener := newTUIViewListener(ctx, func(e agent.AgentEvent) {
			ui.queue(func() {
				ui.handleAgentEvent(e)
			})
		})
		taskCtx = shared.WithParentListener(taskCtx, listener)

		result, err := ui.container.AgentCoordinator.ExecuteTask(taskCtx, task, sessionID, listener)
		ui.queue(func() {
			ui.finishTask(result, err)
		})
	}()
}

func (ui *tviewChatUI) finishTask(result *agent.TaskResult, err error) {
	ui.running = false
	ui.flushStreamBuffer()
	if ui.cancelCurrentTurn != nil {
		ui.cancelCurrentTurn()
	}
	ui.cancelCurrentTurn = nil

	if err != nil {
		if errors.Is(err, context.Canceled) {
			ui.setStatusLine(styleSystem.Render("⏹️ canceled"))
			return
		}
		ui.appendAgentLine(styleError.Render(fmt.Sprintf("✗ Error: %v", err)))
		ui.setStatusLine(styleError.Render("✗ failed"))
		return
	}

	if result != nil {
		ui.setStatusLine(styleGray.Render(fmt.Sprintf("✓ Done • %d iterations • %d tokens", result.Iterations, result.TokensUsed)))
		if !ui.streamedCurrentTurn && strings.TrimSpace(result.Answer) != "" {
			ui.appendAgentRaw(ui.renderer.RenderMarkdownStreamChunk(result.Answer, true))
		}
	}
}

func (ui *tviewChatUI) handleAgentEvent(event agent.AgentEvent) {
	if event == nil {
		return
	}

	if subtaskEvent, ok := event.(*orchestration.SubtaskEvent); ok {
		ui.flushStreamBuffer()
		lines := ui.subagents.Handle(normalizeSubtaskEvent(subtaskEvent))
		for _, line := range lines {
			ui.appendAgentRaw(line)
		}
		return
	}

	if env, ok := event.(*domain.WorkflowEventEnvelope); ok {
		switch env.Event {
		case "workflow.node.output.delta":
			if evt := envelopeToNodeOutputDelta(env); evt != nil {
				ui.onAssistantMessage(evt)
			}
		case "workflow.tool.started":
			if evt := envelopeToToolStarted(env); evt != nil {
				ui.onToolCallStart(evt)
			}
		case "workflow.tool.completed":
			if evt := envelopeToToolCompleted(env); evt != nil {
				ui.onToolCallComplete(evt)
			}
		case "workflow.node.failed":
			if evt := envelopeToNodeFailed(env); evt != nil {
				ui.flushStreamBuffer()
				ui.appendAgentLine(styleError.Render(fmt.Sprintf("✗ Error: %v", evt.Error)))
			}
		}
	}
}

func (ui *tviewChatUI) onAssistantMessage(event *domain.WorkflowNodeOutputDeltaEvent) {
	if event == nil {
		return
	}

	if strings.TrimSpace(event.Delta) != "" || event.Final {
		ui.streamedCurrentTurn = true
	}

	if event.Delta != "" {
		for _, chunk := range ui.mdBuffer.Append(event.Delta) {
			if chunk.content == "" {
				continue
			}
			rendered := ui.renderer.RenderMarkdownStreamChunk(chunk.content, chunk.completeLine)
			ui.appendAgentStreamRaw(rendered)
			ui.lastStreamChunkEndedWithNewline = strings.HasSuffix(rendered, "\n")
		}
	}

	if event.Final {
		trailing := ui.mdBuffer.FlushAll()
		if trailing != "" {
			rendered := ui.renderer.RenderMarkdownStreamChunk(trailing, false)
			ui.appendAgentStreamRaw(rendered)
			if strings.HasSuffix(rendered, "\n") {
				ui.lastStreamChunkEndedWithNewline = true
			} else {
				ui.lastStreamChunkEndedWithNewline = false
				ui.appendAgentStreamRaw("\n")
				ui.lastStreamChunkEndedWithNewline = true
			}
		} else if !ui.lastStreamChunkEndedWithNewline {
			ui.appendAgentStreamRaw("\n")
			ui.lastStreamChunkEndedWithNewline = true
		}
		ui.renderer.ResetMarkdownStreamState()
	}
}

func (ui *tviewChatUI) onToolCallStart(event *domain.WorkflowToolStartedEvent) {
	if event == nil {
		return
	}

	ui.flushStreamBuffer()
	if ui.streamedCurrentTurn && !ui.lastStreamChunkEndedWithNewline {
		ui.appendRaw("\n")
		ui.lastStreamChunkEndedWithNewline = true
	}

	ui.activeTools[event.CallID] = ToolInfo{
		Name:      event.ToolName,
		StartTime: event.Timestamp(),
	}

	outCtx := &types.OutputContext{
		Level:        event.GetAgentLevel(),
		Category:     output.CategorizeToolName(event.ToolName),
		AgentID:      string(event.GetAgentLevel()),
		Verbose:      ui.container.Runtime.Verbose,
		SessionID:    event.GetSessionID(),
		TaskID:       event.GetTaskID(),
		ParentTaskID: event.GetParentTaskID(),
	}

	rendered := ui.renderer.RenderToolCallStart(outCtx, event.ToolName, event.Arguments)
	ui.appendAgentRaw(rendered)
}

func (ui *tviewChatUI) onToolCallComplete(event *domain.WorkflowToolCompletedEvent) {
	if event == nil {
		return
	}

	ui.flushStreamBuffer()
	info, exists := ui.activeTools[event.CallID]
	if !exists {
		return
	}

	outCtx := &types.OutputContext{
		Level:        event.GetAgentLevel(),
		Category:     output.CategorizeToolName(info.Name),
		AgentID:      string(event.GetAgentLevel()),
		Verbose:      ui.container.Runtime.Verbose,
		SessionID:    event.GetSessionID(),
		TaskID:       event.GetTaskID(),
		ParentTaskID: event.GetParentTaskID(),
	}

	duration := time.Since(info.StartTime)
	rendered := ui.renderer.RenderToolCallComplete(outCtx, info.Name, event.Result, event.Error, duration)
	ui.appendAgentRaw(rendered)

	delete(ui.activeTools, event.CallID)
}

func (ui *tviewChatUI) appendSystemCard() {
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
	if ui.outputView != nil {
		_, _, width, _ := ui.outputView.GetInnerRect()
		if width > 0 {
			cardWidth = minInt(100, width-4)
			if cardWidth < 48 {
				cardWidth = 48
			}
		}
	}

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("14")).
		Padding(0, 1).
		Width(cardWidth)

	title := fmt.Sprintf("%s %s", styleBold.Render(styleGreen.Render(tuiAgentName)), styleGray.Render("— interactive"))
	ui.appendLine(card.Render(title + "\n" + strings.Join(lines, "\n")))
}

func (ui *tviewChatUI) appendUserMessage(content string) {
	header := styleBold.Render(styleGreen.Render("You"))
	body := lipgloss.NewStyle().PaddingLeft(2).Render(content)
	ui.appendLine(header)
	ui.appendLine(body)
	ui.appendLine("")
}

func (ui *tviewChatUI) ensureAgentHeader() {
	if ui.assistantHeaderPrinted {
		return
	}
	ui.assistantHeaderPrinted = true
	header := styleBold.Render(styleBoldCyan.Render(tuiAgentName))
	ui.appendLine(header)
}

func (ui *tviewChatUI) appendAgentLine(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	ui.ensureAgentHeader()
	ui.appendLine(indentBlock(line, tuiAgentIndent))
}

func (ui *tviewChatUI) appendAgentRaw(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}

	if !ui.assistantHeaderPrinted {
		content = strings.TrimLeft(content, "\n")
		if strings.TrimSpace(content) == "" {
			return
		}
	}

	ui.ensureAgentHeader()
	ui.appendRaw(indentBlock(content, tuiAgentIndent))
}

func (ui *tviewChatUI) appendAgentStreamRaw(content string) {
	if content == "" {
		return
	}

	if !ui.assistantHeaderPrinted {
		content = strings.TrimLeft(content, "\n")
		if content == "" {
			return
		}
	}

	ui.ensureAgentHeader()

	for _, r := range content {
		if r == '\n' {
			line := indentBlock(ui.streamBuffer.String(), tuiAgentIndent)
			ui.appendLine(strings.TrimSuffix(line, "\n"))
			ui.streamBuffer.Reset()
		} else {
			ui.streamBuffer.WriteRune(r)
		}
	}
}

func (ui *tviewChatUI) flushStreamBuffer() {
	if ui.streamBuffer.Len() == 0 {
		return
	}
	line := indentBlock(ui.streamBuffer.String(), tuiAgentIndent)
	ui.appendLine(strings.TrimSuffix(line, "\n"))
	ui.streamBuffer.Reset()
	ui.lastStreamChunkEndedWithNewline = true
}

func (ui *tviewChatUI) appendLine(content string) {
	content = strings.TrimSuffix(content, "\n")
	ui.appendRaw(content + "\n")
}

func (ui *tviewChatUI) appendRaw(content string) {
	if content == "" || ui.outputWriter == nil {
		return
	}
	fmt.Fprint(ui.outputWriter, content)
	if ui.outputView != nil {
		ui.outputView.ScrollToEnd()
	}
}

func (ui *tviewChatUI) clearTranscript() {
	if ui.outputView == nil {
		return
	}
	ui.outputView.Clear()
	ui.assistantHeaderPrinted = false
	ui.streamBuffer.Reset()
	ui.appendSystemCard()
}

func (ui *tviewChatUI) setStatusLine(line string) {
	ui.statusLine = line
	if ui.statusView == nil {
		return
	}
	ui.statusView.SetText(line)
}

func (ui *tviewChatUI) shutdown() {
	if ui.cancelCurrentTurn != nil {
		ui.cancelCurrentTurn()
		ui.cancelCurrentTurn = nil
	}
	if ui.baseCancel != nil {
		ui.baseCancel()
	}
}

func (ui *tviewChatUI) queue(update func()) {
	if ui.app == nil || update == nil {
		return
	}
	ui.app.QueueUpdateDraw(update)
}

type tviewListener struct {
	ctx     context.Context
	onEvent func(agent.AgentEvent)
}

func newTUIViewListener(ctx context.Context, onEvent func(agent.AgentEvent)) *tviewListener {
	return &tviewListener{ctx: ctx, onEvent: onEvent}
}

func (l *tviewListener) OnEvent(event agent.AgentEvent) {
	if event == nil || l.onEvent == nil {
		return
	}
	if l.ctx != nil && l.ctx.Err() != nil {
		return
	}
	l.onEvent(event)
}

func normalizeSubtaskEvent(event *orchestration.SubtaskEvent) *orchestration.SubtaskEvent {
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
