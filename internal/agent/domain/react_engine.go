package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
	materialports "alex/internal/materials/ports"
	"alex/internal/utils/clilatency"
	id "alex/internal/utils/id"
)

// ReactEngine orchestrates the Think-Act-Observe cycle
type ReactEngine struct {
	maxIterations      int
	stopReasons        []string
	logger             ports.Logger
	clock              ports.Clock
	eventListener      EventListener // Optional event listener for TUI
	completion         completionConfig
	attachmentMigrator materialports.Migrator
	workflow           WorkflowTracker
}

type workflowRecorder struct {
	tracker WorkflowTracker
}

type reactWorkflow struct {
	recorder *workflowRecorder
}

type toolCallBatch struct {
	engine               *ReactEngine
	ctx                  context.Context
	state                *TaskState
	iteration            int
	registry             ports.ToolRegistry
	tracker              *reactWorkflow
	attachments          map[string]ports.Attachment
	attachmentIterations map[string]int
	subagentSnapshots    []*ports.TaskState
	calls                []ToolCall
	callNodes            []string
	results              []ToolResult
	attachmentsMu        sync.Mutex
}

func newWorkflowRecorder(tracker WorkflowTracker) *workflowRecorder {
	if tracker == nil {
		return nil
	}
	return &workflowRecorder{tracker: tracker}
}

func (r *workflowRecorder) ensure(nodeID string, input any) string {
	if r == nil || nodeID == "" {
		return ""
	}
	r.tracker.EnsureNode(nodeID, input)
	return nodeID
}

func (r *workflowRecorder) start(nodeID string, input any) {
	if r.ensure(nodeID, input) == "" {
		return
	}
	r.tracker.StartNode(nodeID)
}

func (r *workflowRecorder) complete(nodeID string, output any, err error) {
	if r == nil || nodeID == "" {
		return
	}
	if err != nil {
		r.tracker.CompleteNodeFailure(nodeID, err)
		return
	}
	r.tracker.CompleteNodeSuccess(nodeID, output)
}

type completionConfig struct {
	temperature   float64
	maxTokens     int
	topP          float64
	stopSequences []string
}

func newReactWorkflow(tracker WorkflowTracker) *reactWorkflow {
	return &reactWorkflow{recorder: newWorkflowRecorder(tracker)}
}

func newToolCallBatch(
	engine *ReactEngine,
	ctx context.Context,
	state *TaskState,
	iteration int,
	calls []ToolCall,
	registry ports.ToolRegistry,
	tracker *reactWorkflow,
) *toolCallBatch {
	expanded := make([]ToolCall, len(calls))
	subagentSnapshots := make([]*ports.TaskState, len(calls))
	for i, call := range calls {
		tc := call
		tc.Arguments = engine.expandToolCallArguments(tc.Name, tc.Arguments, state)
		expanded[i] = tc

		if tc.Name == "subagent" {
			subagentSnapshots[i] = buildSubagentStateSnapshot(state, tc)
		}
	}

	var nodes []string
	if tracker != nil {
		nodes = make([]string, len(expanded))
		for i, call := range expanded {
			nodes[i] = tracker.ensureToolCall(iteration, call)
		}
	}

	attachmentsSnapshot, iterationSnapshot := snapshotAttachments(state)

	return &toolCallBatch{
		engine:               engine,
		ctx:                  ctx,
		state:                state,
		iteration:            iteration,
		registry:             registry,
		tracker:              tracker,
		attachments:          attachmentsSnapshot,
		attachmentIterations: iterationSnapshot,
		subagentSnapshots:    subagentSnapshots,
		calls:                expanded,
		callNodes:            nodes,
	}
}

func (rw *reactWorkflow) startContext(task string) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.start(workflowNodeContext, map[string]any{"task": task})
}

func (rw *reactWorkflow) completeContext(output map[string]any) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.complete(workflowNodeContext, output, nil)
}

func (rw *reactWorkflow) startThink(iteration int) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.start(iterationThinkNode(iteration), map[string]any{"iteration": iteration})
}

func (rw *reactWorkflow) startPlan(iteration, requested int) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.start(iterationPlanNode(iteration), map[string]any{"iteration": iteration, "requested_calls": requested})
}

func (rw *reactWorkflow) completePlan(iteration int, planned []ToolCall, err error) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.complete(iterationPlanNode(iteration), workflowPlanOutput(iteration, planned), err)
}

func (rw *reactWorkflow) completeThink(iteration int, thought Message, toolCalls []ToolCall, err error) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.complete(iterationThinkNode(iteration), workflowThinkOutput(iteration, thought, toolCalls), err)
}

func (rw *reactWorkflow) startTools(iteration int, nodeID string, calls int) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.start(nodeID, map[string]any{"iteration": iteration, "tool_calls": calls})
}

func (rw *reactWorkflow) completeTools(iteration int, nodeID string, results []ToolResult, err error) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.complete(nodeID, workflowToolOutput(iteration, results), err)
}

func (rw *reactWorkflow) ensureToolCall(iteration int, call ToolCall) string {
	id := iterationToolCallNode(iteration, call.ID)
	if rw == nil || rw.recorder == nil {
		return id
	}
	return rw.recorder.ensure(id, workflowToolCallInput(iteration, call))
}

func (rw *reactWorkflow) startToolCall(nodeID string) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.start(nodeID, nil)
}

func (rw *reactWorkflow) completeToolCall(nodeID string, iteration int, call ToolCall, result ToolResult, err error) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.complete(nodeID, workflowToolCallOutput(iteration, call, result), err)
}

func (b *toolCallBatch) execute() []ToolResult {
	b.results = make([]ToolResult, len(b.calls))
	for i, call := range b.calls {
		b.runCall(i, call)
	}
	return b.results
}

func (b *toolCallBatch) runCall(idx int, tc ToolCall) {
	tc.SessionID = b.state.SessionID
	tc.TaskID = b.state.TaskID
	tc.ParentTaskID = b.state.ParentTaskID

	nodeID := ""
	if b.tracker != nil {
		nodeID = b.callNodes[idx]
		if nodeID == "" {
			nodeID = b.tracker.ensureToolCall(b.iteration, tc)
		}
		b.tracker.startToolCall(nodeID)
	}

	startTime := b.engine.clock.Now()

	b.engine.logger.Debug("Tool %d: Getting tool '%s' from registry", idx, tc.Name)
	tool, err := b.registry.Get(tc.Name)
	if err != nil {
		missing := fmt.Errorf("tool not found: %s", tc.Name)
		b.finalize(idx, tc, nodeID, ToolResult{Error: missing}, startTime)
		return
	}

	toolCtx := ports.WithAttachmentContext(b.ctx, b.attachments, b.attachmentIterations)
	if tc.Name == "subagent" {
		if snapshot := b.subagentSnapshots[idx]; snapshot != nil {
			toolCtx = ports.WithClonedTaskStateSnapshot(toolCtx, snapshot)
		}
	}

	formattedArgs := formatToolArgumentsForLog(tc.Arguments)
	b.engine.logger.Debug("Tool %d: Executing '%s' with args: %s", idx, tc.Name, formattedArgs)
	result, execErr := tool.Execute(toolCtx, ports.ToolCall(tc))
	if execErr != nil {
		b.finalize(idx, tc, nodeID, ToolResult{Error: execErr}, startTime)
		return
	}

	if result == nil {
		b.finalize(idx, tc, nodeID, ToolResult{Error: fmt.Errorf("tool %s returned no result", tc.Name)}, startTime)
		return
	}

	result.Attachments = b.engine.applyToolAttachmentMutations(
		b.ctx,
		b.state,
		tc,
		result.Attachments,
		result.Metadata,
		&b.attachmentsMu,
	)
	b.engine.applyImportantNotes(b.state, tc, result.Metadata)

	b.finalize(idx, tc, nodeID, *result, startTime)
}

func (b *toolCallBatch) finalize(idx int, tc ToolCall, nodeID string, result ToolResult, startTime time.Time) {
	normalized := b.engine.normalizeToolResult(tc, b.state, result)
	b.results[idx] = normalized

	duration := b.engine.clock.Now().Sub(startTime)
	b.engine.emitWorkflowToolCompletedEvent(b.ctx, b.state, tc, normalized, duration)

	if b.tracker != nil {
		b.tracker.completeToolCall(nodeID, b.iteration, tc, normalized, normalized.Error)
	}
}

func (rw *reactWorkflow) finalize(stopReason string, result *TaskResult, err error) {
	if rw == nil || rw.recorder == nil {
		return
	}
	rw.recorder.start(workflowNodeFinalize, map[string]any{"stop_reason": stopReason})
	rw.recorder.complete(workflowNodeFinalize, workflowFinalizeOutput(result), err)
}

// WorkflowTracker captures the minimal workflow operations the ReAct engine
// needs for debugging and event emission. Implementations are provided by the
// application layer (e.g., agentWorkflow) to avoid domain-level coupling to a
// specific workflow implementation.
type WorkflowTracker interface {
	EnsureNode(id string, input any)
	StartNode(id string)
	CompleteNodeSuccess(id string, output any)
	CompleteNodeFailure(id string, err error)
}

const (
	workflowNodeContext  = "react:context"
	workflowNodeFinalize = "react:finalize"
)

func iterationThinkNode(iteration int) string {
	return fmt.Sprintf("react:iter:%d:think", iteration)
}

func iterationPlanNode(iteration int) string {
	return fmt.Sprintf("react:iter:%d:plan", iteration)
}

func iterationToolsNode(iteration int) string {
	return fmt.Sprintf("react:iter:%d:tools", iteration)
}

func iterationToolCallNode(iteration int, callID string) string {
	return fmt.Sprintf("react:iter:%d:tool:%s", iteration, callID)
}

// CompletionDefaults defines optional overrides for LLM completion behaviour.
type CompletionDefaults struct {
	Temperature   *float64
	MaxTokens     *int
	TopP          *float64
	StopSequences []string
}

// ReactEngineConfig captures the dependencies required to construct a ReactEngine.
type ReactEngineConfig struct {
	MaxIterations      int
	StopReasons        []string
	Logger             ports.Logger
	Clock              ports.Clock
	EventListener      EventListener
	CompletionDefaults CompletionDefaults
	AttachmentMigrator materialports.Migrator
	Workflow           WorkflowTracker
}

// SetEventListener configures event emission for TUI/streaming
func (e *ReactEngine) SetEventListener(listener EventListener) {
	e.eventListener = listener
}

// GetEventListener returns the current event listener (for saving/restoring)
func (e *ReactEngine) GetEventListener() EventListener {
	return e.eventListener
}

// getAgentLevel reads the current agent level from context
func (e *ReactEngine) getAgentLevel(ctx context.Context) ports.AgentLevel {
	if ctx == nil {
		return ports.LevelCore
	}
	outCtx := ports.GetOutputContext(ctx)
	if outCtx == nil {
		return ports.LevelCore
	}
	return outCtx.Level
}

// emitEvent sends event to listener if one is set
func (e *ReactEngine) emitEvent(event AgentEvent) {
	if e.eventListener != nil {
		e.logger.Debug("[emitEvent] Emitting event type=%s, sessionID=%s to listener", event.EventType(), event.GetSessionID())
		e.eventListener.OnEvent(event)
		e.logger.Debug("[emitEvent] Event emitted successfully")
	} else {
		e.logger.Debug("[emitEvent] No listener set, skipping event type=%s", event.EventType())
	}
}

func (e *ReactEngine) newBaseEvent(ctx context.Context, sessionID, taskID, parentTaskID string) BaseEvent {
	return newBaseEventWithIDs(e.getAgentLevel(ctx), sessionID, taskID, parentTaskID, e.clock.Now())
}

// SolveTask is the main ReAct loop - pure business logic
func (e *ReactEngine) SolveTask(
	ctx context.Context,
	task string,
	state *TaskState,
	services Services,
) (*TaskResult, error) {
	runtime := newReactRuntime(e, ctx, task, state, services, func() {
		e.prepareUserTaskContext(ctx, task, state)
	})
	return runtime.run()
}

// RecordUserInput appends the user input to the task state without running ReAct.
func (e *ReactEngine) RecordUserInput(ctx context.Context, task string, state *TaskState) {
	if state == nil || strings.TrimSpace(task) == "" {
		return
	}
	e.prepareUserTaskContext(ctx, task, state)
}

// think sends current state to LLM for reasoning
func (e *ReactEngine) think(
	ctx context.Context,
	state *TaskState,
	services Services,
) (Message, error) {

	tools := services.ToolExecutor.List()
	requestID := id.NewRequestID()
	filteredMessages, excluded := splitMessagesForLLM(state.Messages)

	e.logger.Debug(
		"Preparing LLM request (request_id=%s): messages=%d (filtered=%d, excluded=%d), tools=%d",
		requestID,
		len(state.Messages),
		len(filteredMessages),
		len(excluded),
		len(tools),
	)

	req := ports.CompletionRequest{
		Messages:    filteredMessages,
		Tools:       tools,
		Temperature: e.completion.temperature,
		MaxTokens:   e.completion.maxTokens,
		TopP:        e.completion.topP,
		Metadata: map[string]any{
			"request_id": requestID,
		},
	}

	if len(e.completion.stopSequences) > 0 {
		req.StopSequences = append([]string(nil), e.completion.stopSequences...)
	}

	timestamp := e.clock.Now()
	snapshot := NewWorkflowDiagnosticContextSnapshotEvent(
		e.getAgentLevel(ctx),
		state.SessionID,
		state.TaskID,
		state.ParentTaskID,
		state.Iterations,
		state.Iterations,
		requestID,
		filteredMessages,
		excluded,
		timestamp,
	)
	e.emitEvent(snapshot)
	if services.Context != nil {
		summary := snapshotSummaryFromMessages(state.Messages)
		record := buildContextTurnRecord(state, filteredMessages, timestamp, summary)
		if err := services.Context.RecordTurn(ctx, record); err != nil {
			e.logger.Warn("Failed to persist turn snapshot: %v", err)
		}
	}

	e.logger.Debug("Calling LLM (request_id=%s)...", requestID)

	modelName := ""
	if services.LLM != nil {
		modelName = services.LLM.Model()
	}

	llmCallStarted := time.Now()
	resp, err := services.LLM.Complete(ctx, req)
	clilatency.Printf(
		"[latency] llm_complete_ms=%.2f iteration=%d model=%s request_id=%s\n",
		float64(time.Since(llmCallStarted))/float64(time.Millisecond),
		state.Iterations,
		strings.TrimSpace(modelName),
		requestID,
	)

	if err != nil {
		e.logger.Error("LLM call failed (request_id=%s): %v", requestID, err)
		return Message{}, fmt.Errorf("LLM call failed: %w", err)
	}

	e.emitEvent(&WorkflowNodeOutputDeltaEvent{
		BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		Iteration:   state.Iterations,
		Delta:       resp.Content,
		Final:       true,
		CreatedAt:   e.clock.Now(),
		SourceModel: modelName,
	})

	e.logger.Debug("LLM response received (request_id=%s): content=%d bytes, tool_calls=%d",
		requestID, len(resp.Content), len(resp.ToolCalls))

	return Message{
		Role:      "assistant",
		Content:   resp.Content,
		ToolCalls: resp.ToolCalls,
		Source:    ports.MessageSourceAssistantReply,
	}, nil
}

func (e *ReactEngine) normalizeToolResult(tc ToolCall, state *TaskState, result ToolResult) ToolResult {
	normalized := result
	if normalized.CallID == "" {
		normalized.CallID = tc.ID
	}
	if normalized.SessionID == "" {
		normalized.SessionID = state.SessionID
	}
	if normalized.TaskID == "" {
		normalized.TaskID = state.TaskID
	}
	if normalized.ParentTaskID == "" {
		normalized.ParentTaskID = state.ParentTaskID
	}
	if strings.EqualFold(tc.Name, "a2ui_emit") {
		normalized.Attachments = nil
		if len(normalized.Metadata) > 0 {
			delete(normalized.Metadata, "attachment_mutations")
			delete(normalized.Metadata, "attachments_mutations")
			delete(normalized.Metadata, "attachmentMutations")
			delete(normalized.Metadata, "attachmentsMutations")
			if len(normalized.Metadata) == 0 {
				normalized.Metadata = nil
			}
		}
	}
	return normalized
}

func (e *ReactEngine) emitWorkflowToolCompletedEvent(ctx context.Context, state *TaskState, tc ToolCall, result ToolResult, duration time.Duration) {
	e.emitEvent(&WorkflowToolCompletedEvent{
		BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		CallID:      result.CallID,
		ToolName:    tc.Name,
		Result:      result.Content,
		Error:       result.Error,
		Duration:    duration,
		Metadata:    result.Metadata,
		Attachments: result.Attachments,
	})
}

const (
	toolArgInlineLengthLimit = 256
	toolArgPreviewLength     = 64
	toolResultPreviewRunes   = 280
	maxFeedbackSignals       = 20
)

// parseToolCalls extracts tool calls from assistant message
func (e *ReactEngine) parseToolCalls(msg Message, parser ports.FunctionCallParser) []ToolCall {

	if len(msg.ToolCalls) > 0 {
		e.logger.Debug("Using native tool calls from message: count=%d", len(msg.ToolCalls))
		return msg.ToolCalls
	}

	e.logger.Debug("Parsing tool calls from content: length=%d", len(msg.Content))
	parsed, err := parser.Parse(msg.Content)
	if err != nil {
		e.logger.Warn("Failed to parse tool calls from content: %v", err)
		return nil
	}

	calls := make([]ToolCall, 0, len(parsed))
	calls = append(calls, parsed...)

	e.logger.Debug("Parsed %d tool calls from content", len(calls))
	return calls
}

// buildToolMessages converts tool results into messages sent back to the LLM.
func (e *ReactEngine) buildToolMessages(results []ToolResult) []Message {
	messages := make([]Message, 0, len(results))

	for _, result := range results {
		var content string
		if result.Error != nil {
			content = fmt.Sprintf("Tool %s failed: %v", result.CallID, result.Error)
		} else if trimmed := strings.TrimSpace(result.Content); trimmed != "" {
			content = trimmed
		} else {
			content = fmt.Sprintf("Tool %s completed successfully.", result.CallID)
		}

		content = ensureToolAttachmentReferences(content, result.Attachments)

		msg := Message{
			Role:        "tool",
			Content:     content,
			ToolCallID:  result.CallID,
			ToolResults: []ToolResult{result},
			Source:      ports.MessageSourceToolResult,
		}

		msg.Attachments = normalizeToolAttachments(result.Attachments)

		messages = append(messages, msg)
	}

	return messages
}

// finalize creates the final task result
func (e *ReactEngine) finalize(state *TaskState, stopReason string, duration time.Duration) *TaskResult {

	finalAnswer := strings.TrimSpace(state.FinalAnswer)
	if finalAnswer == "" {
		for i := len(state.Messages) - 1; i >= 0; i-- {
			if state.Messages[i].Role == "assistant" {
				finalAnswer = state.Messages[i].Content
				break
			}
		}
	}

	attachments := resolveContentAttachments(finalAnswer, state)
	finalAnswer = ensureAttachmentPlaceholders(finalAnswer, attachments)

	return &TaskResult{
		Answer:       finalAnswer,
		Messages:     state.Messages,
		Iterations:   state.Iterations,
		TokensUsed:   state.TokenCount,
		StopReason:   stopReason,
		SessionID:    state.SessionID,
		TaskID:       state.TaskID,
		ParentTaskID: state.ParentTaskID,
		Important:    ports.CloneImportantNotes(state.Important),
		Duration:     duration,
	}
}

func (e *ReactEngine) decorateFinalResult(state *TaskState, result *TaskResult) map[string]ports.Attachment {
	if state == nil || result == nil {
		return nil
	}

	attachments := resolveContentAttachments(result.Answer, state)

	result.Answer = ensureAttachmentPlaceholders(result.Answer, attachments)

	a2uiAttachments := collectA2UIAttachments(state)
	if len(a2uiAttachments) == 0 {
		if len(attachments) == 0 {
			return nil
		}
		return attachments
	}

	if attachments == nil {
		attachments = make(map[string]ports.Attachment, len(a2uiAttachments))
	}
	for key, att := range a2uiAttachments {
		if _, ok := attachments[key]; ok {
			continue
		}
		attachments[key] = att
	}
	return attachments
}

func workflowContextOutput(state *TaskState) map[string]any {
	if state == nil {
		return nil
	}

	snapshot := ports.CloneTaskState(state)
	if snapshot == nil {
		return nil
	}

	pending := snapshot.PendingUserAttachments
	if pending == nil && len(snapshot.Attachments) > 0 {
		pending = snapshot.Attachments
	}

	return map[string]any{
		"messages":              snapshot.Messages,
		"attachments":           snapshot.Attachments,
		"pending_attachments":   pending,
		"iteration":             snapshot.Iterations,
		"token_count":           snapshot.TokenCount,
		"attachment_iterations": snapshot.AttachmentIterations,
	}
}

func workflowThinkOutput(iteration int, thought Message, toolCalls []ToolCall) map[string]any {
	output := map[string]any{
		"iteration":  iteration,
		"tool_calls": len(toolCalls),
	}

	if trimmed := strings.TrimSpace(thought.Content); trimmed != "" {
		output["content"] = trimmed
	}
	if len(thought.Attachments) > 0 {
		output["attachments"] = ports.CloneAttachmentMap(thought.Attachments)
	}

	return output
}

func workflowPlanOutput(iteration int, toolCalls []ToolCall) map[string]any {
	output := map[string]any{
		"iteration": iteration,
	}

	if len(toolCalls) > 0 {
		names := make([]string, 0, len(toolCalls))
		for _, call := range toolCalls {
			if call.Name != "" {
				names = append(names, call.Name)
			}
		}
		output["tool_calls"] = len(toolCalls)
		if len(names) > 0 {
			output["tools"] = names
		}
	}

	return output
}

func workflowToolCallInput(iteration int, call ToolCall) map[string]any {
	input := map[string]any{
		"iteration": iteration,
		"call_id":   call.ID,
		"tool":      call.Name,
	}

	if len(call.Arguments) > 0 {
		args := make(map[string]any, len(call.Arguments))
		for k, v := range call.Arguments {
			args[k] = v
		}
		input["arguments"] = args
	}

	return input
}

func workflowToolCallOutput(iteration int, call ToolCall, result ToolResult) map[string]any {
	output := map[string]any{
		"iteration": iteration,
		"call_id":   call.ID,
		"tool":      call.Name,
	}

	cloned := ports.CloneToolResults([]ToolResult{result})
	if len(cloned) > 0 {
		output["result"] = cloned[0]
	}

	return output
}

func workflowToolOutput(iteration int, results []ToolResult) map[string]any {
	output := map[string]any{
		"iteration": iteration,
	}

	if len(results) > 0 {
		output["results"] = ports.CloneToolResults(results)
	}

	successes := 0
	failures := 0
	for _, result := range results {
		if result.Error != nil {
			failures++
			continue
		}
		successes++
	}

	output["success"] = successes
	output["failed"] = failures

	return output
}

func workflowFinalizeOutput(result *TaskResult) map[string]any {
	if result == nil {
		return map[string]any{"stop_reason": "error"}
	}

	output := map[string]any{
		"stop_reason": result.StopReason,
		"iterations":  result.Iterations,
		"tokens_used": result.TokensUsed,
	}

	if trimmed := strings.TrimSpace(result.Answer); trimmed != "" {
		output["answer_preview"] = trimmed
	}
	if len(result.Messages) > 0 {
		output["messages"] = ports.CloneMessages(result.Messages)
	}

	return output
}

// cleanToolCallMarkers removes leaked tool call XML markers from content
func (e *ReactEngine) cleanToolCallMarkers(content string) string {

	patterns := []string{
		`<\|tool_call_begin\|>.*`,
		`<tool_call>.*(?:</tool_call>)?$`,
		`user<\|tool_call_begin\|>.*`,
		`functions\.[\w_]+:\d+\(.*`,
	}

	cleaned := content
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		cleaned = re.ReplaceAllString(cleaned, "")
	}

	return strings.TrimSpace(cleaned)
}

func (e *ReactEngine) ensureSystemPromptMessage(state *TaskState) {
	if state == nil {
		return
	}

	prompt := strings.TrimSpace(state.SystemPrompt)
	if prompt == "" {
		return
	}

	for idx := range state.Messages {
		role := strings.ToLower(strings.TrimSpace(state.Messages[idx].Role))
		if role != "system" {
			continue
		}
		source := ports.MessageSource(strings.TrimSpace(string(state.Messages[idx].Source)))
		if source != "" && source != ports.MessageSourceSystemPrompt {

			continue
		}

		if strings.TrimSpace(state.Messages[idx].Content) != prompt {
			state.Messages[idx].Content = state.SystemPrompt
			state.Messages[idx].Source = ports.MessageSourceSystemPrompt
			e.logger.Debug("Updated existing system prompt in message history")
		} else if source == "" {
			state.Messages[idx].Source = ports.MessageSourceSystemPrompt
		}

		existing := state.Messages[idx]
		if idx > 0 {
			state.Messages = append(state.Messages[:idx], state.Messages[idx+1:]...)
			state.Messages = append([]Message{existing}, state.Messages...)
		} else {
			state.Messages[0] = existing
		}
		return
	}

	systemMessage := Message{
		Role:    "system",
		Content: state.SystemPrompt,
		Source:  ports.MessageSourceSystemPrompt,
	}

	state.Messages = append([]Message{systemMessage}, state.Messages...)
	e.logger.Debug("Inserted system prompt into message history")
}

func (e *ReactEngine) normalizeAttachmentsWithMigrator(ctx context.Context, state *TaskState, req materialports.MigrationRequest) map[string]ports.Attachment {
	if len(req.Attachments) == 0 || e.attachmentMigrator == nil {
		return req.Attachments
	}
	if req.Context == nil {
		req.Context = e.materialRequestContext(state, "")
	}
	normalized, err := e.attachmentMigrator.Normalize(ctx, req)
	if err != nil {
		e.logger.Warn("attachment migration failed: %v", err)
		return req.Attachments
	}
	return normalized
}

type attachmentMutations struct {
	replace map[string]ports.Attachment
	add     map[string]ports.Attachment
	update  map[string]ports.Attachment
	remove  []string
}

func (e *ReactEngine) applyToolAttachmentMutations(
	ctx context.Context,
	state *TaskState,
	call ToolCall,
	attachments map[string]ports.Attachment,
	metadata map[string]any,
	attachmentsMu *sync.Mutex,
) map[string]ports.Attachment {
	normalized := normalizeAttachmentMap(attachments)
	mutations := normalizeAttachmentMutations(metadata)

	if normalized != nil {
		normalized = e.normalizeAttachmentsWithMigrator(ctx, state, materialports.MigrationRequest{
			Context:     e.materialRequestContext(state, call.ID),
			Attachments: normalized,
			Status:      materialapi.MaterialStatusIntermediate,
			Origin:      call.Name,
		})
	}

	if mutations != nil {
		mutations.replace = e.normalizeAttachmentsWithMigrator(ctx, state, materialports.MigrationRequest{
			Context:     e.materialRequestContext(state, call.ID),
			Attachments: mutations.replace,
			Status:      materialapi.MaterialStatusIntermediate,
			Origin:      call.Name,
		})
		mutations.add = e.normalizeAttachmentsWithMigrator(ctx, state, materialports.MigrationRequest{
			Context:     e.materialRequestContext(state, call.ID),
			Attachments: mutations.add,
			Status:      materialapi.MaterialStatusIntermediate,
			Origin:      call.Name,
		})
		mutations.update = e.normalizeAttachmentsWithMigrator(ctx, state, materialports.MigrationRequest{
			Context:     e.materialRequestContext(state, call.ID),
			Attachments: mutations.update,
			Status:      materialapi.MaterialStatusIntermediate,
			Origin:      call.Name,
		})
	}

	var existing map[string]ports.Attachment
	if state != nil {
		if attachmentsMu != nil {
			attachmentsMu.Lock()
			existing = normalizeAttachmentMap(state.Attachments)
			attachmentsMu.Unlock()
		} else {
			existing = normalizeAttachmentMap(state.Attachments)
		}
	}

	merged := mergeAttachmentMutations(normalized, mutations, existing)
	if attachmentsMu != nil {
		attachmentsMu.Lock()
		defer attachmentsMu.Unlock()
	}
	applyAttachmentMutationsToState(state, merged, mutations, call.Name)
	return merged
}

func (e *ReactEngine) applyImportantNotes(state *TaskState, call ToolCall, metadata map[string]any) {
	if state == nil || len(metadata) == 0 {
		return
	}
	raw, ok := metadata["important_notes"]
	if !ok {
		return
	}
	notes := normalizeImportantNotes(raw, e.clock)
	if len(notes) == 0 {
		return
	}
	if state.Important == nil {
		state.Important = make(map[string]ports.ImportantNote)
	}
	for _, note := range notes {
		if strings.TrimSpace(note.Content) == "" {
			continue
		}
		if note.ID == "" {
			note.ID = id.NewKSUID()
		}
		if note.CreatedAt.IsZero() && e.clock != nil {
			note.CreatedAt = e.clock.Now()
		}
		if note.Source == "" {
			note.Source = call.Name
		}
		state.Important[note.ID] = note
	}
}

func normalizeImportantNotes(raw any, clock ports.Clock) []ports.ImportantNote {
	switch v := raw.(type) {
	case []ports.ImportantNote:
		notes := make([]ports.ImportantNote, len(v))
		copy(notes, v)
		return notes
	case []any:
		var notes []ports.ImportantNote
		for _, item := range v {
			switch note := item.(type) {
			case ports.ImportantNote:
				notes = append(notes, note)
			case map[string]any:
				if parsed := parseImportantNoteMap(note, clock); parsed.Content != "" {
					notes = append(notes, parsed)
				}
			}
		}
		return notes
	case map[string]any:
		if parsed := parseImportantNoteMap(v, clock); parsed.Content != "" {
			return []ports.ImportantNote{parsed}
		}
	}
	return nil
}

func parseImportantNoteMap(raw map[string]any, clock ports.Clock) ports.ImportantNote {
	note := ports.ImportantNote{}
	if idVal, ok := raw["id"].(string); ok {
		note.ID = strings.TrimSpace(idVal)
	}
	if content, ok := raw["content"].(string); ok {
		note.Content = strings.TrimSpace(content)
	}
	if source, ok := raw["source"].(string); ok {
		note.Source = strings.TrimSpace(source)
	}
	if tagsRaw, ok := raw["tags"].([]any); ok {
		for _, tag := range tagsRaw {
			if text, ok := tag.(string); ok {
				if trimmed := strings.TrimSpace(text); trimmed != "" {
					note.Tags = append(note.Tags, trimmed)
				}
			}
		}
	}
	switch created := raw["created_at"].(type) {
	case time.Time:
		note.CreatedAt = created
	case string:
		if parsed, err := time.Parse(time.RFC3339, created); err == nil {
			note.CreatedAt = parsed
		}
	}
	if note.CreatedAt.IsZero() && clock != nil {
		note.CreatedAt = clock.Now()
	}
	return note
}

func (e *ReactEngine) normalizeMessageHistoryAttachments(ctx context.Context, state *TaskState) {
	if state == nil {
		return
	}
	for idx := range state.Messages {
		msg := state.Messages[idx]
		if len(msg.Attachments) == 0 {
			continue
		}
		normalized := e.normalizeAttachmentsWithMigrator(ctx, state, materialports.MigrationRequest{
			Context:     e.materialRequestContext(state, msg.ToolCallID),
			Attachments: msg.Attachments,
			Status:      messageMaterialStatus(msg),
			Origin:      messageMaterialOrigin(msg),
		})
		if normalized != nil {
			state.Messages[idx].Attachments = normalized
		}
	}
}

func (e *ReactEngine) materialRequestContext(state *TaskState, toolCallID string) *materialapi.RequestContext {
	if state == nil {
		return nil
	}
	return &materialapi.RequestContext{
		RequestID:      state.TaskID,
		TaskID:         state.TaskID,
		AgentIteration: uint32(state.Iterations),
		ToolCallID:     toolCallID,
		ConversationID: state.SessionID,
		UserID:         state.SessionID,
	}
}

const attachmentCatalogMetadataKey = "attachment_catalog"

func (e *ReactEngine) updateAttachmentCatalogMessage(state *TaskState) {
	if state == nil {
		return
	}
	content := buildAttachmentCatalogContent(state)
	if strings.TrimSpace(content) == "" {
		content = "Attachment catalog (for model reference only).\nNo attachments are currently available."
	}
	note := Message{
		Role:    "assistant",
		Content: content,
		Source:  ports.MessageSourceAssistantReply,
		Metadata: map[string]any{
			attachmentCatalogMetadataKey: true,
		},
	}
	state.Messages = append(state.Messages, note)
}

func (e *ReactEngine) expandPlaceholders(args map[string]any, state *TaskState) map[string]any {
	if len(args) == 0 {
		return args
	}
	expanded := make(map[string]any, len(args))
	for key, value := range args {
		expanded[key] = e.expandPlaceholderValue(value, state)
	}
	return expanded
}

func (e *ReactEngine) expandToolCallArguments(toolName string, args map[string]any, state *TaskState) map[string]any {
	if len(args) == 0 {
		return args
	}

	// Artifact tools operate on attachment filenames; expanding them into URIs
	// breaks name-based operations (e.g., artifacts_list/delete).
	var skipKeys map[string]bool
	switch strings.TrimSpace(toolName) {
	case "artifacts_list":
		skipKeys = map[string]bool{"name": true}
	case "artifacts_write":
		skipKeys = map[string]bool{"name": true}
	case "artifacts_delete":
		skipKeys = map[string]bool{"name": true, "names": true}
	default:
		return e.expandPlaceholders(args, state)
	}

	expanded := make(map[string]any, len(args))
	for key, value := range args {
		if skipKeys[key] {
			expanded[key] = unwrapAttachmentPlaceholderValue(value)
			continue
		}
		expanded[key] = e.expandPlaceholderValue(value, state)
	}
	return expanded
}

func unwrapAttachmentPlaceholderValue(value any) any {
	switch v := value.(type) {
	case string:
		if name, ok := extractPlaceholderName(v); ok {
			return name
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = unwrapAttachmentPlaceholderValue(v[i])
		}
		return out
	case []string:
		out := make([]string, len(v))
		for i := range v {
			out[i] = v[i]
			if name, ok := extractPlaceholderName(v[i]); ok {
				out[i] = name
			}
		}
		return out
	default:
		return value
	}
}

func (e *ReactEngine) expandPlaceholderValue(value any, state *TaskState) any {
	switch v := value.(type) {
	case string:
		if replacement, ok := e.resolveStringAttachmentValue(v, state); ok {
			return replacement
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = e.expandPlaceholderValue(item, state)
		}
		return out
	case []string:
		out := make([]string, len(v))
		for i, item := range v {
			if replacement, ok := e.resolveStringAttachmentValue(item, state); ok {
				out[i] = replacement
				continue
			}
			out[i] = item
		}
		return out
	case map[string]any:
		nested := make(map[string]any, len(v))
		for key, item := range v {
			nested[key] = e.expandPlaceholderValue(item, state)
		}
		return nested
	case map[string]string:
		nested := make(map[string]string, len(v))
		for key, item := range v {
			if replacement, ok := e.resolveStringAttachmentValue(item, state); ok {
				nested[key] = replacement
				continue
			}
			nested[key] = item
		}
		return nested
	default:
		return value
	}
}

var genericImageAliasPattern = regexp.MustCompile(`(?i)^image(?:[_\-\s]?(\d+))?(?:\.[a-z0-9]+)?$`)

func (e *ReactEngine) lookupAttachmentByName(name string, state *TaskState) (ports.Attachment, string, bool) {
	att, canonical, kind, ok := lookupAttachmentByNameInternal(name, state)
	if !ok {
		return ports.Attachment{}, "", false
	}

	switch kind {
	case attachmentMatchSeedreamAlias:
		e.logger.Info("Resolved Seedream placeholder alias [%s] -> [%s]", name, canonical)
	case attachmentMatchGeneric:
		e.logger.Info("Mapping generic image placeholder [%s] to attachment [%s]", name, canonical)
	}

	return att, canonical, true
}

const (
	attachmentMatchExact           = "exact"
	attachmentMatchCaseInsensitive = "case_insensitive"
	attachmentMatchSeedreamAlias   = "seedream_alias"
	attachmentMatchGeneric         = "generic_alias"
)

type attachmentCandidate struct {
	key        string
	attachment ports.Attachment
	iteration  int
	generated  bool
}

var contentPlaceholderPattern = regexp.MustCompile(`\[([^\[\]]+)\]`)

func (e *ReactEngine) resolveStringAttachmentValue(value string, state *TaskState) (string, bool) {
	alias, att, canonical, ok := matchAttachmentReference(value, state)
	if !ok {
		return "", false
	}
	replacement := attachmentReferenceValue(att)
	if replacement == "" {
		return "", false
	}
	if canonical != "" && canonical != alias {
		e.logger.Info("Resolved placeholder [%s] as alias for attachment [%s]", alias, canonical)
	}
	return replacement, true
}

const snapshotSummaryLimit = 160

func (e *ReactEngine) observeToolResults(state *TaskState, iteration int, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	ensureWorldStateMap(state)
	updates := make([]map[string]any, 0, len(results))
	for _, result := range results {
		updates = append(updates, summarizeToolResultForWorld(result))
	}
	state.WorldState["last_tool_results"] = updates
	state.WorldState["last_iteration"] = iteration
	state.WorldState["last_updated_at"] = e.clock.Now().Format(time.RFC3339)
	state.WorldDiff = map[string]any{
		"iteration":    iteration,
		"tool_results": updates,
	}
	e.appendFeedbackSignals(state, results)
}

func (e *ReactEngine) appendFeedbackSignals(state *TaskState, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}
	now := e.clock.Now()
	for _, result := range results {
		signal := ports.FeedbackSignal{
			Kind:      "tool_result",
			Message:   buildFeedbackMessage(result),
			Value:     deriveFeedbackValue(result),
			CreatedAt: now,
		}
		state.FeedbackSignals = append(state.FeedbackSignals, signal)
	}
	if len(state.FeedbackSignals) > maxFeedbackSignals {
		state.FeedbackSignals = state.FeedbackSignals[len(state.FeedbackSignals)-maxFeedbackSignals:]
	}
}
