package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	"alex/internal/async"
	"alex/internal/logging"
	"alex/internal/output"
	"alex/internal/tools/builtin/orchestration"
	"alex/internal/tools/builtin/shared"
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
	lastCompletion                  *domain.WorkflowResultFinalEvent
	streamedContent                 bool
	mdBuffer                        *markdownStreamBuffer
	lastStreamChunkEndedWithNewline bool
	startedAt                       time.Time
	firstTokenLogged                bool
	streamWriter                    *streamWriter
}

var ErrForceExit = errors.New("force exit requested by user")

const (
	// markdownBufferThreshold controls how much streamed markdown we buffer before
	// emitting a partial fragment. Keep this small so CLI/TUI get fast first-byte
	// output even when the model streams without newlines.
	markdownBufferThreshold = 1
	// markdownMaxFlushDelay bounds how long we wait to show partial output after
	// the last flush, even if the buffer is still small.
	markdownMaxFlushDelay = 0
)

type markdownChunk struct {
	content      string
	completeLine bool
}

type markdownStreamBuffer struct {
	builder     strings.Builder
	maxBuffered int
	maxDelay    time.Duration
	lastFlush   time.Time
	flushedOnce bool
	rawMode     bool
}

func newMarkdownStreamBuffer() *markdownStreamBuffer {
	return &markdownStreamBuffer{
		maxBuffered: markdownBufferThreshold,
		maxDelay:    markdownMaxFlushDelay,
	}
}

func (b *markdownStreamBuffer) Append(delta string) []markdownChunk {
	if delta == "" {
		return nil
	}

	b.builder.WriteString(delta)
	data := b.builder.String()
	var chunks []markdownChunk
	now := time.Now()

	for {
		idx := strings.IndexByte(data, '\n')
		if idx == -1 {
			break
		}

		chunk := data[:idx+1]
		chunks = append(chunks, markdownChunk{content: chunk, completeLine: !b.rawMode})
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
		b.flushedOnce = true
		b.lastFlush = now
		return chunks
	}

	// Ensure first token shows up immediately even if the model doesn't emit
	// newlines for a while.
	if !b.flushedOnce && data != "" {
		b.builder.Reset()
		b.flushedOnce = true
		b.lastFlush = now
		b.rawMode = true
		return []markdownChunk{{content: data, completeLine: false}}
	}

	if b.maxBuffered > 0 && len(data) >= b.maxBuffered {
		chunk := data
		b.builder.Reset()
		b.flushedOnce = true
		b.lastFlush = now
		b.rawMode = true
		return []markdownChunk{{content: chunk, completeLine: false}}
	}

	if b.maxDelay > 0 && !b.lastFlush.IsZero() && now.Sub(b.lastFlush) >= b.maxDelay && data != "" {
		chunk := data
		b.builder.Reset()
		b.lastFlush = now
		b.rawMode = true
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
	handler := &StreamingOutputHandler{
		container:       container,
		renderer:        output.NewCLIRenderer(verbose),
		activeTools:     make(map[string]ToolInfo),
		subagentDisplay: NewSubagentDisplay(),
		verbose:         verbose,
		out:             os.Stdout,
		mdBuffer:        newMarkdownStreamBuffer(),
	}
	handler.streamWriter = newStreamWriter(handler.out)
	return handler
}

// SetOutputWriter allows overriding the output writer, primarily for testing
func (h *StreamingOutputHandler) SetOutputWriter(w io.Writer) {
	if w == nil {
		h.out = os.Stdout
		if h.streamWriter != nil {
			h.streamWriter.out = h.out
		}
		return
	}
	h.out = w
	if h.streamWriter != nil {
		h.streamWriter.out = h.out
	}
}

// RunTaskWithStreamOutput executes a task with inline streaming output
func RunTaskWithStreamOutput(container *Container, task string, sessionID string) error {
	// Start execution with stream handler
	baseCtx := cliBaseContext()
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
	ctx = shared.WithApprover(ctx, cliApproverForSession(sessionID))
	ctx = shared.WithAutoApprove(ctx, false)

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
		LogID:        ids.LogID,
	}
	ctx = types.WithOutputContext(ctx, coreOutCtx)

	handler := NewStreamingOutputHandler(container, verbose)
	handler.ctx = ctx // Store context for OutputContext lookup
	handler.startedAt = time.Now()
	handler.streamWriter = newStreamWriter(handler.out)
	handler.printTaskStart(task)

	// Create event bridge
	bridge := NewStreamEventBridge(handler)

	// Add listener to context so subagent can forward events
	ctx = shared.WithParentListener(ctx, bridge)

	// Handle interrupts for graceful cancellation
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	done := make(chan struct{})
	defer close(done)

	var forceExit atomic.Bool

	logger := logging.FromContext(ctx, logging.NewComponentLogger("CLIStreamOutput"))
	async.Go(logger, "cli.signal-handler", func() {
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
	})

	// Execute task with streaming via listener
	domainResult, err := container.AgentCoordinator.ExecuteTask(ctx, task, sessionID, bridge)
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

// OnEvent implements agent.EventListener
func (b *StreamEventBridge) OnEvent(event agent.AgentEvent) {
	// Check if this is a wrapped subtask event
	if subtaskEvent, ok := event.(*orchestration.SubtaskEvent); ok {
		// Handle subtask-specific tracking
		b.handler.handleSubtaskEvent(subtaskEvent)
		return
	}

	// Handle normalized workflow envelopes (new event contract)
	if env, ok := event.(*domain.WorkflowEventEnvelope); ok {
		b.handleEnvelopeEvent(env)
		return
	}

	// Handle regular events
	switch e := event.(type) {
	case *domain.WorkflowNodeStartedEvent:
		b.handler.onIterationStart(e)
	case *domain.WorkflowNodeOutputSummaryEvent:
		b.handler.onThinkComplete(e)
	case *domain.WorkflowNodeOutputDeltaEvent:
		b.handler.onAssistantMessage(e)
	case *domain.WorkflowToolStartedEvent:
		b.handler.onToolCallStart(e)
	case *domain.WorkflowToolCompletedEvent:
		b.handler.onToolCallComplete(e)
	case *domain.WorkflowNodeFailedEvent:
		b.handler.onError(e)
	case *domain.WorkflowResultFinalEvent:
		b.handler.onTaskComplete(e)
	}
}

func (h *StreamingOutputHandler) onIterationStart(event *domain.WorkflowNodeStartedEvent) {
	// Silent - don't print iteration headers in simple mode
}

func (h *StreamingOutputHandler) onThinkComplete(event *domain.WorkflowNodeOutputSummaryEvent) {
	// Silent - don't print analysis output
	// Analysis is internal reasoning, not user-facing output
}

func (h *StreamingOutputHandler) onAssistantMessage(event *domain.WorkflowNodeOutputDeltaEvent) {
	h.streamedContent = true
	if event.Delta != "" {
		for _, chunk := range h.mdBuffer.Append(event.Delta) {
			if chunk.content == "" {
				continue
			}
			if !h.firstTokenLogged && strings.TrimSpace(chunk.content) != "" {
				if _, ok := os.LookupEnv("ALEX_CLI_LATENCY"); ok {
					elapsed := time.Since(h.startedAt)
					_, _ = fmt.Fprintf(os.Stderr, "[latency] first_token_ms=%.2f\n", float64(elapsed.Microseconds())/1000.0)
				}
				h.firstTokenLogged = true
			}
			rendered := h.renderer.RenderMarkdownStreamChunk(chunk.content, chunk.completeLine)
			h.streamWriter.WriteChunk(rendered)
			h.lastStreamChunkEndedWithNewline = strings.HasSuffix(rendered, "\n")
		}
	}
	if event.Final {
		trailing := h.mdBuffer.FlushAll()
		if trailing != "" {
			rendered := h.renderer.RenderMarkdownStreamChunk(trailing, false)
			h.streamWriter.WriteChunk(rendered)
			if strings.HasSuffix(rendered, "\n") {
				h.lastStreamChunkEndedWithNewline = true
			} else {
				h.lastStreamChunkEndedWithNewline = false
				h.streamWriter.WriteChunk("\n")
				h.lastStreamChunkEndedWithNewline = true
			}
		} else if !h.lastStreamChunkEndedWithNewline {
			h.streamWriter.WriteChunk("\n")
			h.lastStreamChunkEndedWithNewline = true
		}
		h.renderer.ResetMarkdownStreamState()
		h.streamWriter.Flush()
	}
}

func (h *StreamingOutputHandler) onToolCallStart(event *domain.WorkflowToolStartedEvent) {
	h.streamWriter.Flush()
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

func (h *StreamingOutputHandler) onToolCallComplete(event *domain.WorkflowToolCompletedEvent) {
	info, exists := h.activeTools[event.CallID]
	if !exists {
		return
	}
	h.streamWriter.Flush()

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

func (h *StreamingOutputHandler) onError(event *domain.WorkflowNodeFailedEvent) {
	h.streamWriter.Flush()
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

func (h *StreamingOutputHandler) onTaskComplete(event *domain.WorkflowResultFinalEvent) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastCompletion = event
}

func (h *StreamingOutputHandler) printTaskStart(task string) {
	h.streamWriter.Flush()
	ids := id.IDsFromContext(h.ctx)
	outCtx := &types.OutputContext{
		Level:        types.LevelCore,
		AgentID:      "core",
		Verbose:      h.verbose,
		SessionID:    ids.SessionID,
		TaskID:       ids.TaskID,
		ParentTaskID: ids.ParentTaskID,
	}

	rendered := h.renderer.RenderTaskStart(outCtx, task)
	h.write(rendered)
}

func (h *StreamingOutputHandler) printCompletion(result *agent.TaskResult) {
	h.streamWriter.Flush()
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

func (h *StreamingOutputHandler) printInterruptRequested() {
	h.write("\n⏹️  Interrupt requested – attempting graceful shutdown (press Ctrl+C again to force exit)\n")
}

func (h *StreamingOutputHandler) printForcedExit() {
	h.write("\n⏹️  Force exit requested – terminating immediately.\n")
}

func (h *StreamingOutputHandler) printCancellation(event *domain.WorkflowResultFinalEvent) {
	summary := "⚠️ Task interrupted"
	if event != nil {
		summary = fmt.Sprintf("⚠️ Task interrupted | %d iteration(s) | %d tokens", event.TotalIterations, event.TotalTokens)
	}
	h.write("\n" + summary + "\n")
}

func (h *StreamingOutputHandler) consumeTaskCompletion() *domain.WorkflowResultFinalEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	event := h.lastCompletion
	h.lastCompletion = nil
	return event
}

// handleSubtaskEvent handles events from subtasks with simple line-by-line output
func (h *StreamingOutputHandler) handleSubtaskEvent(subtaskEvent *orchestration.SubtaskEvent) {
	h.streamWriter.Flush()
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
	type flusher interface{ Flush() error }
	if f, ok := h.out.(flusher); ok {
		_ = f.Flush()
	}
}

type streamWriter struct {
	out       io.Writer
	maxRunes  int
	maxDelay  time.Duration
	lastFlush time.Time
	runes     int
	builder   strings.Builder
}

func newStreamWriter(out io.Writer) *streamWriter {
	return &streamWriter{
		out:      out,
		maxRunes: 8,
		maxDelay: 12 * time.Millisecond,
	}
}

func (w *streamWriter) WriteChunk(chunk string) {
	if chunk == "" {
		return
	}

	for _, r := range chunk {
		w.builder.WriteRune(r)
		w.runes++
		if r == '\n' || w.runes >= w.maxRunes || (w.maxDelay > 0 && !w.lastFlush.IsZero() && time.Since(w.lastFlush) >= w.maxDelay) {
			w.flush()
		}
	}
}

func (w *streamWriter) Flush() {
	w.flush()
}

func (w *streamWriter) flush() {
	if w.runes == 0 {
		return
	}
	if _, err := fmt.Fprint(w.out, w.builder.String()); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "stream output write error: %v\n", err)
	}
	w.builder.Reset()
	w.runes = 0
	w.lastFlush = time.Now()
}

func (b *StreamEventBridge) handleEnvelopeEvent(env *domain.WorkflowEventEnvelope) {
	if env == nil {
		return
	}

	switch env.Event {
	case "workflow.node.started":
		if evt := envelopeToNodeStarted(env); evt != nil {
			b.handler.onIterationStart(evt)
		}
	case "workflow.node.output.summary":
		if evt := envelopeToNodeOutputSummary(env); evt != nil {
			b.handler.onThinkComplete(evt)
		}
	case "workflow.node.output.delta":
		if evt := envelopeToNodeOutputDelta(env); evt != nil {
			b.handler.onAssistantMessage(evt)
		}
	case "workflow.tool.started":
		if evt := envelopeToToolStarted(env); evt != nil {
			b.handler.onToolCallStart(evt)
		}
	case "workflow.tool.completed":
		if evt := envelopeToToolCompleted(env); evt != nil {
			b.handler.onToolCallComplete(evt)
		}
	case "workflow.node.failed":
		if evt := envelopeToNodeFailed(env); evt != nil {
			b.handler.onError(evt)
		}
	case "workflow.result.final":
		if evt := envelopeToResultFinal(env); evt != nil {
			b.handler.onTaskComplete(evt)
		}
	}
}

func envelopeToNodeStarted(env *domain.WorkflowEventEnvelope) *domain.WorkflowNodeStartedEvent {
	return &domain.WorkflowNodeStartedEvent{
		BaseEvent:       envelopeBase(env),
		Iteration:       payloadInt(env.Payload, "iteration"),
		TotalIters:      payloadInt(env.Payload, "total_iters"),
		StepIndex:       payloadInt(env.Payload, "step_index"),
		StepDescription: payloadString(env.Payload, "step_description"),
		Workflow:        nil,
	}
}

func envelopeToNodeOutputSummary(env *domain.WorkflowEventEnvelope) *domain.WorkflowNodeOutputSummaryEvent {
	return &domain.WorkflowNodeOutputSummaryEvent{
		BaseEvent:     envelopeBase(env),
		Iteration:     payloadInt(env.Payload, "iteration"),
		Content:       payloadString(env.Payload, "content"),
		ToolCallCount: payloadInt(env.Payload, "tool_call_count"),
	}
}

func envelopeToNodeOutputDelta(env *domain.WorkflowEventEnvelope) *domain.WorkflowNodeOutputDeltaEvent {
	return &domain.WorkflowNodeOutputDeltaEvent{
		BaseEvent:    envelopeBase(env),
		Iteration:    payloadInt(env.Payload, "iteration"),
		MessageCount: payloadInt(env.Payload, "message_count"),
		Delta:        payloadString(env.Payload, "delta"),
		Final:        payloadBool(env.Payload, "final"),
		CreatedAt:    payloadTime(env.Payload, "created_at"),
		SourceModel:  payloadString(env.Payload, "source_model"),
	}
}

func envelopeToToolStarted(env *domain.WorkflowEventEnvelope) *domain.WorkflowToolStartedEvent {
	callID := env.NodeID
	if callID == "" {
		callID = payloadString(env.Payload, "call_id")
	}
	return &domain.WorkflowToolStartedEvent{
		BaseEvent: envelopeBase(env),
		Iteration: payloadInt(env.Payload, "iteration"),
		CallID:    callID,
		ToolName:  payloadString(env.Payload, "tool_name"),
		Arguments: payloadArgs(env.Payload, "arguments"),
	}
}

func envelopeToToolCompleted(env *domain.WorkflowEventEnvelope) *domain.WorkflowToolCompletedEvent {
	callID := env.NodeID
	if callID == "" {
		callID = payloadString(env.Payload, "call_id")
	}
	var errVal error
	if msg := payloadString(env.Payload, "error"); msg != "" {
		errVal = errors.New(msg)
	}

	return &domain.WorkflowToolCompletedEvent{
		BaseEvent:   envelopeBase(env),
		CallID:      callID,
		ToolName:    payloadString(env.Payload, "tool_name"),
		Result:      payloadString(env.Payload, "result"),
		Error:       errVal,
		Duration:    time.Duration(payloadInt64(env.Payload, "duration")) * time.Millisecond,
		Metadata:    payloadMap(env.Payload, "metadata"),
		Attachments: payloadAttachments(env.Payload, "attachments"),
	}
}

func envelopeToNodeFailed(env *domain.WorkflowEventEnvelope) *domain.WorkflowNodeFailedEvent {
	var errVal error
	if msg := payloadString(env.Payload, "error"); msg != "" {
		errVal = errors.New(msg)
	}

	return &domain.WorkflowNodeFailedEvent{
		BaseEvent:   envelopeBase(env),
		Iteration:   payloadInt(env.Payload, "iteration"),
		Phase:       payloadString(env.Payload, "phase"),
		Error:       errVal,
		Recoverable: payloadBool(env.Payload, "recoverable"),
	}
}

func envelopeToResultFinal(env *domain.WorkflowEventEnvelope) *domain.WorkflowResultFinalEvent {
	return &domain.WorkflowResultFinalEvent{
		BaseEvent:       envelopeBase(env),
		FinalAnswer:     payloadString(env.Payload, "final_answer"),
		TotalIterations: payloadInt(env.Payload, "total_iterations"),
		TotalTokens:     payloadInt(env.Payload, "total_tokens"),
		StopReason:      payloadString(env.Payload, "stop_reason"),
		Duration:        time.Duration(payloadInt64(env.Payload, "duration")) * time.Millisecond,
		IsStreaming:     payloadBool(env.Payload, "is_streaming"),
		StreamFinished:  payloadBool(env.Payload, "stream_finished"),
		Attachments:     payloadAttachments(env.Payload, "attachments"),
	}
}

func envelopeBase(env *domain.WorkflowEventEnvelope) domain.BaseEvent {
	if env == nil {
		return domain.NewBaseEvent(types.LevelCore, "", "", "", time.Now())
	}
	ts := env.Timestamp()
	if ts.IsZero() {
		ts = time.Now()
	}
	return domain.NewBaseEvent(env.GetAgentLevel(), env.GetSessionID(), env.GetTaskID(), env.GetParentTaskID(), ts)
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	val, ok := payload[key]
	if !ok || val == nil {
		return ""
	}
	switch v := val.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case []byte:
		return string(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return fmt.Sprint(v)
	}
}

func payloadBool(payload map[string]any, key string) bool {
	if payload == nil {
		return false
	}
	val, ok := payload[key]
	if !ok || val == nil {
		return false
	}
	switch v := val.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return err == nil && parsed
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func payloadInt(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	val, ok := payload[key]
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return 0
}

func payloadInt64(payload map[string]any, key string) int64 {
	if payload == nil {
		return 0
	}
	val, ok := payload[key]
	if !ok || val == nil {
		return 0
	}
	switch v := val.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

func payloadTime(payload map[string]any, key string) time.Time {
	if payload == nil {
		return time.Time{}
	}
	val, ok := payload[key]
	if !ok || val == nil {
		return time.Time{}
	}
	switch v := val.(type) {
	case time.Time:
		return v
	case string:
		if t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(v)); err == nil {
			return t
		}
	}
	return time.Time{}
}

func payloadArgs(payload map[string]any, key string) map[string]interface{} {
	if payload == nil {
		return nil
	}
	val, ok := payload[key]
	if !ok || val == nil {
		return nil
	}
	if args, ok := val.(map[string]interface{}); ok {
		return args
	}
	if args, ok := val.(map[string]any); ok {
		return args
	}
	return nil
}

func payloadMap(payload map[string]any, key string) map[string]any {
	if payload == nil {
		return nil
	}
	val, ok := payload[key]
	if !ok || val == nil {
		return nil
	}
	if m, ok := val.(map[string]any); ok {
		return m
	}
	if m, ok := val.(map[string]interface{}); ok {
		return m
	}
	return nil
}

func payloadAttachments(payload map[string]any, key string) map[string]ports.Attachment {
	if payload == nil {
		return nil
	}
	val, ok := payload[key]
	if !ok || val == nil {
		return nil
	}
	if attachments, ok := val.(map[string]ports.Attachment); ok {
		return attachments
	}
	if generic, ok := val.(map[string]any); ok && len(generic) > 0 {
		result := make(map[string]ports.Attachment)
		for k, v := range generic {
			if att, ok := v.(ports.Attachment); ok {
				result[k] = att
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return nil
}
