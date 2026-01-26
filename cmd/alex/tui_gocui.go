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
	"alex/internal/tools/builtin/orchestration"
	"alex/internal/tools/builtin/shared"
	id "alex/internal/utils/id"

	"github.com/charmbracelet/lipgloss"
	"github.com/jroimartin/gocui"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

const (
	gocuiOutputView = "output"
	gocuiStatusView = "status"
	gocuiPromptView = "prompt"
	gocuiInputView  = "input"
)

type gocuiChatUI struct {
	container *Container
	gui       *gocui.Gui

	outputView *gocui.View
	statusView *gocui.View
	promptView *gocui.View
	inputView  *gocui.View

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

func RunGocui(container *Container) error {
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

	gui, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		baseCancel()
		return err
	}
	defer gui.Close()

	ui := newGocuiChatUI(container, baseCtx, baseCancel, gui)
	gui.SetManagerFunc(ui.layout)

	if err := ui.bindKeys(); err != nil {
		baseCancel()
		return err
	}

	if err := gui.MainLoop(); err != nil && !errors.Is(err, gocui.ErrQuit) {
		baseCancel()
		return err
	}
	baseCancel()
	return nil
}

func newGocuiChatUI(container *Container, baseCtx context.Context, baseCancel context.CancelFunc, gui *gocui.Gui) *gocuiChatUI {
	return &gocuiChatUI{
		container:   container,
		gui:         gui,
		baseCtx:     baseCtx,
		baseCancel:  baseCancel,
		startTime:   time.Now(),
		renderer:    output.NewCLIRendererWithMarkdown(container.Runtime.Verbose, plainMarkdownRenderer{}),
		activeTools: make(map[string]ToolInfo),
		subagents:   NewSubagentDisplay(),
		mdBuffer:    newMarkdownStreamBuffer(),
	}
}

func (ui *gocuiChatUI) bindKeys() error {
	if ui.gui == nil {
		return fmt.Errorf("gui is nil")
	}

	if err := ui.gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, ui.onCtrlC); err != nil {
		return err
	}
	if err := ui.gui.SetKeybinding("", gocui.KeyCtrlL, gocui.ModNone, ui.onClear); err != nil {
		return err
	}
	if err := ui.gui.SetKeybinding(gocuiInputView, gocui.KeyEnter, gocui.ModNone, ui.onSubmit); err != nil {
		return err
	}

	return nil
}

func (ui *gocuiChatUI) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	if maxX < 10 || maxY < 6 {
		return nil
	}

	if v, err := g.SetView(gocuiOutputView, 0, 0, maxX-1, maxY-4); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Autoscroll = true
		v.Wrap = true
		v.Frame = false
		ui.outputView = v
		ui.appendSystemCard()
	}

	if v, err := g.SetView(gocuiStatusView, 0, maxY-3, maxX-1, maxY-3); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Frame = false
		ui.statusView = v
	}

	if v, err := g.SetView(gocuiPromptView, 0, maxY-2, 1, maxY-2); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Frame = false
		v.Editable = false
		v.Wrap = false
		fmt.Fprint(v, "❯ ")
		ui.promptView = v
	}

	if v, err := g.SetView(gocuiInputView, 2, maxY-2, maxX-1, maxY-2); err != nil {
		if !errors.Is(err, gocui.ErrUnknownView) {
			return err
		}
		v.Frame = false
		v.Editable = true
		v.Wrap = false
		v.Autoscroll = false
		v.Editor = gocui.DefaultEditor
		ui.inputView = v
		if _, err := g.SetCurrentView(gocuiInputView); err != nil {
			return err
		}
	}

	return nil
}

func (ui *gocuiChatUI) onCtrlC(g *gocui.Gui, _ *gocui.View) error {
	if ui.running && ui.cancelCurrentTurn != nil {
		ui.setStatusLine(styleSystem.Render("⏹️ cancel requested…"))
		ui.cancelCurrentTurn()
		return nil
	}
	ui.shutdown()
	return gocui.ErrQuit
}

func (ui *gocuiChatUI) onClear(_ *gocui.Gui, _ *gocui.View) error {
	ui.clearTranscript()
	ui.setStatusLine(styleGray.Render("cleared"))
	return nil
}

func (ui *gocuiChatUI) onSubmit(g *gocui.Gui, v *gocui.View) error {
	if ui.running {
		ui.setStatusLine(styleGray.Render("busy (Ctrl+C to cancel)"))
		return nil
	}
	if v == nil {
		return nil
	}

	input := strings.TrimSpace(v.Buffer())
	v.Clear()
	_ = v.SetCursor(0, 0)
	_ = v.SetOrigin(0, 0)

	cmd := parseUserCommand(input)
	switch cmd.kind {
	case commandEmpty:
		return nil
	case commandQuit:
		ui.shutdown()
		return gocui.ErrQuit
	case commandClear:
		ui.clearTranscript()
		ui.setStatusLine(styleGray.Render("cleared"))
		return nil
	case commandRun:
		ui.appendUserMessage(cmd.task)
		ui.streamedCurrentTurn = false
		ui.assistantHeaderPrinted = false
		ui.running = true
		ui.setStatusLine(styleGray.Render("running…"))

		taskCtx, cancel := context.WithCancel(ui.baseCtx)
		ui.cancelCurrentTurn = cancel
		ui.startTask(taskCtx, cmd.task)
		return nil
	default:
		return nil
	}
}

func (ui *gocuiChatUI) startTask(ctx context.Context, task string) {
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

		listener := newGocuiListener(ctx, func(e agent.AgentEvent) {
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

func (ui *gocuiChatUI) finishTask(result *agent.TaskResult, err error) {
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

func (ui *gocuiChatUI) handleAgentEvent(event agent.AgentEvent) {
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

func (ui *gocuiChatUI) onAssistantMessage(event *domain.WorkflowNodeOutputDeltaEvent) {
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

func (ui *gocuiChatUI) onToolCallStart(event *domain.WorkflowToolStartedEvent) {
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

func (ui *gocuiChatUI) onToolCallComplete(event *domain.WorkflowToolCompletedEvent) {
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

func (ui *gocuiChatUI) appendSystemCard() {
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
		width, _ := ui.outputView.Size()
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

func (ui *gocuiChatUI) appendUserMessage(content string) {
	header := styleBold.Render(styleGreen.Render("You"))
	body := lipgloss.NewStyle().PaddingLeft(2).Render(content)
	ui.appendLine(header)
	ui.appendLine(body)
	ui.appendLine("")
}

func (ui *gocuiChatUI) ensureAgentHeader() {
	if ui.assistantHeaderPrinted {
		return
	}
	ui.assistantHeaderPrinted = true
	header := styleBold.Render(styleBoldCyan.Render(tuiAgentName))
	ui.appendLine(header)
}

func (ui *gocuiChatUI) appendAgentLine(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	ui.ensureAgentHeader()
	ui.appendLine(indentBlock(line, tuiAgentIndent))
}

func (ui *gocuiChatUI) appendAgentRaw(content string) {
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

func (ui *gocuiChatUI) appendAgentStreamRaw(content string) {
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

func (ui *gocuiChatUI) flushStreamBuffer() {
	if ui.streamBuffer.Len() == 0 {
		return
	}
	line := indentBlock(ui.streamBuffer.String(), tuiAgentIndent)
	ui.appendLine(strings.TrimSuffix(line, "\n"))
	ui.streamBuffer.Reset()
	ui.lastStreamChunkEndedWithNewline = true
}

func (ui *gocuiChatUI) appendLine(content string) {
	content = strings.TrimSuffix(content, "\n")
	ui.appendRaw(content + "\n")
}

func (ui *gocuiChatUI) appendRaw(content string) {
	if content == "" || ui.outputView == nil {
		return
	}
	fmt.Fprint(ui.outputView, content)
}

func (ui *gocuiChatUI) clearTranscript() {
	if ui.outputView == nil {
		return
	}
	ui.outputView.Clear()
	ui.assistantHeaderPrinted = false
	ui.streamBuffer.Reset()
	ui.appendSystemCard()
}

func (ui *gocuiChatUI) setStatusLine(line string) {
	ui.statusLine = line
	if ui.statusView == nil {
		return
	}
	ui.statusView.Clear()
	fmt.Fprint(ui.statusView, line)
}

func (ui *gocuiChatUI) shutdown() {
	if ui.cancelCurrentTurn != nil {
		ui.cancelCurrentTurn()
		ui.cancelCurrentTurn = nil
	}
	if ui.baseCancel != nil {
		ui.baseCancel()
	}
}

func (ui *gocuiChatUI) queue(update func()) {
	if ui.gui == nil || update == nil {
		return
	}
	ui.gui.Update(func(*gocui.Gui) error {
		update()
		return nil
	})
}

type gocuiListener struct {
	ctx     context.Context
	onEvent func(agent.AgentEvent)
}

func newGocuiListener(ctx context.Context, onEvent func(agent.AgentEvent)) *gocuiListener {
	return &gocuiListener{ctx: ctx, onEvent: onEvent}
}

func (l *gocuiListener) OnEvent(event agent.AgentEvent) {
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

type plainMarkdownRenderer struct{}

func (plainMarkdownRenderer) Render(content string) (string, error) {
	return content, nil
}
