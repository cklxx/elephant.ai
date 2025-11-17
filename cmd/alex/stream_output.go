package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/types"
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
	activeTools                     map[string]ToolInfo
	subagentDisplay                 *SubagentDisplay
	verbose                         bool
	mu                              sync.Mutex
	lastCompletion                  *domain.TaskCompleteEvent
	streamedContent                 bool
	mdBuffer                        *markdownStreamBuffer
	lastStreamChunkEndedWithNewline bool
}

var ErrForceExit = errors.New("force exit requested by user")

const markdownBufferThreshold = 4096

type markdownChunk struct {
	content      string
	completeLine bool
}

type markdownStreamBuffer struct {
	builder     strings.Builder
	maxBuffered int
}

func newMarkdownStreamBuffer() *markdownStreamBuffer {
	return &markdownStreamBuffer{maxBuffered: markdownBufferThreshold}
}

func (b *markdownStreamBuffer) Append(delta string) []markdownChunk {
	if delta == "" {
		return nil
	}

	b.builder.WriteString(delta)
	data := b.builder.String()
	var chunks []markdownChunk

	for {
		idx := strings.IndexByte(data, '\n')
		if idx == -1 {
			break
		}

		chunk := data[:idx+1]
		chunks = append(chunks, markdownChunk{content: chunk, completeLine: true})
		if idx+1 >= len(data) {
			data = ""
			break
		}
		data = data[idx+1:]
	}

	if len(chunks) > 0 {
		b.builder.Reset()
		if len(data) > 0 {
			b.builder.WriteString(data)
		}
		return chunks
	}

	if b.maxBuffered > 0 && len(data) >= b.maxBuffered {
		chunk := data
		b.builder.Reset()
		return []markdownChunk{{content: chunk, completeLine: false}}
	}

	return nil
}

func (b *markdownStreamBuffer) FlushAll() string {
	if b.builder.Len() == 0 {
		return ""
	}

	trailing := b.builder.String()
	b.builder.Reset()
	return trailing
}

func NewStreamingOutputHandler(container *Container, verbose bool) *StreamingOutputHandler {
	return &StreamingOutputHandler{
		container:       container,
		renderer:        output.NewCLIRenderer(verbose),
		activeTools:     make(map[string]ToolInfo),
		subagentDisplay: NewSubagentDisplay(),
		verbose:         verbose,
		out:             os.Stdout,
		mdBuffer:        newMarkdownStreamBuffer(),
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
	baseCtx := context.Background()
	ctx, cancel := context.WithCancel(baseCtx)
	defer cancel()

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

	// Create event bridge
	bridge := NewStreamEventBridge(handler)

	// Add listener to context so subagent can forward events
	ctx = builtin.WithParentListener(ctx, bridge)

	// Handle interrupts for graceful cancellation
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	done := make(chan struct{})
	defer close(done)

	var forceExit atomic.Bool

	go func() {
		interrupted := false
		for {
			select {
			case <-signals:
				if !interrupted {
					interrupted = true
					handler.printInterruptRequested()
					cancel()
				} else {
					handler.printForcedExit()
					forceExit.Store(true)
					cancel()
				}
			case <-done:
				return
			}
		}
	}()

	// Execute task with streaming via listener
	domainResult, err := container.Coordinator.ExecuteTask(ctx, task, sessionID, bridge)
	if err != nil {
		if forceExit.Load() {
			handler.consumeTaskCompletion()
			return ErrForceExit
		}
		if errors.Is(err, context.Canceled) {
			completion := handler.consumeTaskCompletion()
			handler.printCancellation(completion)
			return nil
		}
		return fmt.Errorf("task execution failed: %w", err)
	}

	// Print completion summary
	handler.printCompletion(domainResult)
	handler.consumeTaskCompletion()

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
	case *domain.AssistantMessageEvent:
		b.handler.onAssistantMessage(e)
	case *domain.ToolCallStartEvent:
		b.handler.onToolCallStart(e)
	case *domain.ToolCallCompleteEvent:
		b.handler.onToolCallComplete(e)
	case *domain.ErrorEvent:
		b.handler.onError(e)
	case *domain.TaskCompleteEvent:
		b.handler.onTaskComplete(e)
	case *domain.AutoReviewEvent:
		b.handler.onAutoReview(e)
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

func (h *StreamingOutputHandler) onAssistantMessage(event *domain.AssistantMessageEvent) {
	h.streamedContent = true
	if event.Delta != "" {
		for _, chunk := range h.mdBuffer.Append(event.Delta) {
			if chunk.content == "" {
				continue
			}
			rendered := h.renderer.RenderMarkdownStreamChunk(chunk.content, chunk.completeLine)
			h.write(rendered)
			h.lastStreamChunkEndedWithNewline = strings.HasSuffix(rendered, "\n")
		}
	}
	if event.Final {
		trailing := h.mdBuffer.FlushAll()
		if trailing != "" {
			rendered := h.renderer.RenderMarkdownStreamChunk(trailing, false)
			h.write(rendered)
			if strings.HasSuffix(rendered, "\n") {
				h.lastStreamChunkEndedWithNewline = true
			} else {
				h.lastStreamChunkEndedWithNewline = false
				h.write("\n")
				h.lastStreamChunkEndedWithNewline = true
			}
		} else if !h.lastStreamChunkEndedWithNewline {
			h.write("\n")
			h.lastStreamChunkEndedWithNewline = true
		}
	}
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

func (h *StreamingOutputHandler) onTaskComplete(event *domain.TaskCompleteEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastCompletion = event
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
	resultCopy := *result
	if h.streamedContent {
		resultCopy.Answer = ""
	}
	rendered := h.renderer.RenderTaskComplete(outCtx, (*domain.TaskResult)(&resultCopy))
	h.write(rendered)
	h.streamedContent = false
}

func (h *StreamingOutputHandler) onAutoReview(event *domain.AutoReviewEvent) {
	if event == nil {
		return
	}
	if summary := h.renderer.RenderAutoReviewSummary(event.Summary); summary != "" {
		h.write(summary)
	}
}

func (h *StreamingOutputHandler) printInterruptRequested() {
	h.write("\n⏹️  Interrupt requested – attempting graceful shutdown (press Ctrl+C again to force exit)\n")
}

func (h *StreamingOutputHandler) printForcedExit() {
	h.write("\n⏹️  Force exit requested – terminating immediately.\n")
}

func (h *StreamingOutputHandler) printCancellation(event *domain.TaskCompleteEvent) {
	summary := "⚠️ Task interrupted"
	if event != nil {
		summary = fmt.Sprintf("⚠️ Task interrupted | %d iteration(s) | %d tokens", event.TotalIterations, event.TotalTokens)
	}
	h.write("\n" + summary + "\n")
}

func (h *StreamingOutputHandler) consumeTaskCompletion() *domain.TaskCompleteEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	event := h.lastCompletion
	h.lastCompletion = nil
	return event
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
