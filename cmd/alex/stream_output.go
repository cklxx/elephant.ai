package main

import (
	"alex/internal/agent/types"
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/output"
	"alex/internal/tools/builtin"
	id "alex/internal/utils/id"
)

// ToolInfo stores information about an active tool call
type ToolInfo struct {
	Name      string
	StartTime time.Time
}

// StreamingOutputHandler handles streaming output to terminal using unified renderers
type StreamingOutputHandler struct {
	container *Container
	renderer  *output.CLIRenderer
	ctx       context.Context // Context from coordinator for OutputContext
	out       io.Writer

	// State
	activeTools     map[string]ToolInfo
	subagentDisplay *SubagentDisplay
	verbose         bool
}

func NewStreamingOutputHandler(container *Container, verbose bool) *StreamingOutputHandler {
	return &StreamingOutputHandler{
		container:       container,
		renderer:        output.NewCLIRenderer(verbose),
		activeTools:     make(map[string]ToolInfo),
		subagentDisplay: NewSubagentDisplay(),
		verbose:         verbose,
		out:             os.Stdout,
	}
}

// SetOutputWriter allows overriding the output writer, primarily for testing
func (h *StreamingOutputHandler) SetOutputWriter(w io.Writer) {
	if w == nil {
		h.out = os.Stdout
		return
	}
	h.out = w
}

// RunTaskWithStreamOutput executes a task with inline streaming output
func RunTaskWithStreamOutput(container *Container, task string, sessionID string) error {
	// Start execution with stream handler
	ctx := context.Background()

	if sessionID == "" {
		session, err := container.SessionStore.Create(ctx)
		if err != nil {
			return fmt.Errorf("create session: %w", err)
		}
		sessionID = session.ID
	}

	ctx = id.WithSessionID(ctx, sessionID)
	ctx = id.WithTaskID(ctx, id.NewTaskID())

	verbose := container.Runtime.Verbose

	// Set core agent output context
	ids := id.IDsFromContext(ctx)
	coreOutCtx := &types.OutputContext{
		Level:        types.LevelCore,
		AgentID:      "core",
		Verbose:      verbose,
		SessionID:    ids.SessionID,
		TaskID:       ids.TaskID,
		ParentTaskID: ids.ParentTaskID,
	}
	ctx = types.WithOutputContext(ctx, coreOutCtx)

	handler := NewStreamingOutputHandler(container, verbose)
	handler.ctx = ctx // Store context for OutputContext lookup

	// Announce execution context for easy correlation in the terminal
	contextLine := fmt.Sprintf("Session: %s · Task: %s", ids.SessionID, ids.TaskID)
	if ids.ParentTaskID != "" {
		contextLine += fmt.Sprintf(" · Parent: %s", ids.ParentTaskID)
	}
	if _, err := fmt.Fprintln(handler.out, contextLine); err != nil {
		return fmt.Errorf("write execution context: %w", err)
	}
	if _, err := fmt.Fprintln(handler.out); err != nil {
		return fmt.Errorf("write execution spacing: %w", err)
	}

	// Create event bridge
	bridge := NewStreamEventBridge(handler)

	// Add listener to context so subagent can forward events
	ctx = builtin.WithParentListener(ctx, bridge)

	// Execute task with streaming via listener
	domainResult, err := container.Coordinator.ExecuteTask(ctx, task, sessionID, bridge)
	if err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}

	// Print completion summary
	handler.printCompletion(domainResult)

	return nil
}

// StreamEventBridge converts domain events to stream output
type StreamEventBridge struct {
	handler *StreamingOutputHandler
}

func NewStreamEventBridge(handler *StreamingOutputHandler) *StreamEventBridge {
	return &StreamEventBridge{handler: handler}
}

// OnEvent implements ports.EventListener
func (b *StreamEventBridge) OnEvent(event ports.AgentEvent) {
	// Check if this is a wrapped subtask event
	if subtaskEvent, ok := event.(*builtin.SubtaskEvent); ok {
		// Handle subtask-specific tracking
		b.handler.handleSubtaskEvent(subtaskEvent)
		return
	}

	// Handle regular events
	switch e := event.(type) {
	case *domain.TaskAnalysisEvent:
		b.handler.onTaskAnalysis(e)
	case *domain.IterationStartEvent:
		b.handler.onIterationStart(e)
	case *domain.ThinkingEvent:
		b.handler.onThinking(e)
	case *domain.ThinkCompleteEvent:
		b.handler.onThinkComplete(e)
	case *domain.ToolCallStartEvent:
		b.handler.onToolCallStart(e)
	case *domain.ToolCallCompleteEvent:
		b.handler.onToolCallComplete(e)
	case *domain.ErrorEvent:
		b.handler.onError(e)
	}
}

// Event handlers

func (h *StreamingOutputHandler) onTaskAnalysis(event *domain.TaskAnalysisEvent) {
	// Use agent level from event (events now carry their source info)
	outCtx := &types.OutputContext{
		Level:        event.GetAgentLevel(),
		AgentID:      string(event.GetAgentLevel()),
		Verbose:      h.verbose,
		SessionID:    event.GetSessionID(),
		TaskID:       event.GetTaskID(),
		ParentTaskID: event.GetParentTaskID(),
	}
	rendered := h.renderer.RenderTaskAnalysis(outCtx, event)
	h.write(rendered)
}

func (h *StreamingOutputHandler) onIterationStart(event *domain.IterationStartEvent) {
	// Silent - don't print iteration headers in simple mode
}

func (h *StreamingOutputHandler) onThinking(event *domain.ThinkingEvent) {
	// Silent - analysis is shown in think complete
}

func (h *StreamingOutputHandler) onThinkComplete(event *domain.ThinkCompleteEvent) {
	// Silent - don't print analysis output
	// Analysis is internal reasoning, not user-facing output
}

func (h *StreamingOutputHandler) onToolCallStart(event *domain.ToolCallStartEvent) {
	h.activeTools[event.CallID] = ToolInfo{
		Name:      event.ToolName,
		StartTime: event.Timestamp(),
	}

	// Use agent level from event
	outCtx := &types.OutputContext{
		Level:        event.GetAgentLevel(),
		Category:     output.CategorizeToolName(event.ToolName),
		AgentID:      string(event.GetAgentLevel()),
		Verbose:      h.verbose,
		SessionID:    event.GetSessionID(),
		TaskID:       event.GetTaskID(),
		ParentTaskID: event.GetParentTaskID(),
	}

	rendered := h.renderer.RenderToolCallStart(outCtx, event.ToolName, event.Arguments)
	h.write(rendered)
}

func (h *StreamingOutputHandler) onToolCallComplete(event *domain.ToolCallCompleteEvent) {
	info, exists := h.activeTools[event.CallID]
	if !exists {
		return
	}

	// Use agent level from event
	outCtx := &types.OutputContext{
		Level:        event.GetAgentLevel(),
		Category:     output.CategorizeToolName(info.Name),
		AgentID:      string(event.GetAgentLevel()),
		Verbose:      h.verbose,
		SessionID:    event.GetSessionID(),
		TaskID:       event.GetTaskID(),
		ParentTaskID: event.GetParentTaskID(),
	}

	duration := time.Since(info.StartTime)
	rendered := h.renderer.RenderToolCallComplete(outCtx, info.Name, event.Result, event.Error, duration)
	h.write(rendered)

	delete(h.activeTools, event.CallID)
}

func (h *StreamingOutputHandler) onError(event *domain.ErrorEvent) {
	outCtx := &types.OutputContext{
		Level:        event.GetAgentLevel(),
		AgentID:      string(event.GetAgentLevel()),
		Verbose:      h.verbose,
		SessionID:    event.GetSessionID(),
		TaskID:       event.GetTaskID(),
		ParentTaskID: event.GetParentTaskID(),
	}
	rendered := h.renderer.RenderError(outCtx, event.Phase, event.Error)
	h.write(rendered)
}

func (h *StreamingOutputHandler) printCompletion(result *ports.TaskResult) {
	outCtx := &types.OutputContext{
		Level:        types.LevelCore,
		AgentID:      "core",
		Verbose:      h.verbose,
		SessionID:    result.SessionID,
		TaskID:       result.TaskID,
		ParentTaskID: result.ParentTaskID,
	}
	rendered := h.renderer.RenderTaskComplete(outCtx, (*domain.TaskResult)(result))
	h.write(rendered)
}

// handleSubtaskEvent handles events from subtasks with simple line-by-line output
func (h *StreamingOutputHandler) handleSubtaskEvent(subtaskEvent *builtin.SubtaskEvent) {
	lines := h.subagentDisplay.Handle(subtaskEvent)
	for _, line := range lines {
		h.write(line)
	}
}

func (h *StreamingOutputHandler) write(rendered string) {
	if rendered == "" {
		return
	}
	if _, err := fmt.Fprint(h.out, rendered); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "stream output write error: %v\n", err)
	}
}
