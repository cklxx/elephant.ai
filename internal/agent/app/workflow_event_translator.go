package app

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/workflow"
)

// wrapWithWorkflowEnvelope decorates the provided listener with a translator that
// emits semantic workflow.* envelope events alongside domain events.
func wrapWithWorkflowEnvelope(listener ports.EventListener, logger *slog.Logger) ports.EventListener {
	if listener == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &workflowEventTranslator{
		sink:           listener,
		logger:         logger,
		subflowTracker: newSubflowStatsTracker(),
	}
}

type workflowEventTranslator struct {
	sink           ports.EventListener
	logger         *slog.Logger
	subflowTracker *subflowStatsTracker
}

func (t *workflowEventTranslator) OnEvent(evt ports.AgentEvent) {
	if evt == nil || t.sink == nil {
		return
	}

	// Avoid re-wrapping envelopes that already follow the new contract.
	if _, ok := evt.(*domain.WorkflowEventEnvelope); ok {
		t.sink.OnEvent(evt)
		return
	}

	for _, envelope := range t.translate(evt) {
		if envelope == nil {
			continue
		}
		t.sink.OnEvent(envelope)
	}
}

func (t *workflowEventTranslator) translate(evt ports.AgentEvent) []*domain.WorkflowEventEnvelope {
	switch e := evt.(type) {
	case *domain.WorkflowLifecycleUpdatedEvent:
		env := baseEnvelope(evt, "workflow.lifecycle.updated")
		if env == nil {
			return nil
		}
		env.WorkflowID = e.WorkflowID
		env.RunID = e.WorkflowID
		payload := map[string]any{
			"workflow_event_type": string(e.WorkflowEventType),
		}
		if e.Phase != "" {
			payload["phase"] = e.Phase
		}
		if e.Node != nil {
			env.NodeID = e.Node.ID
			env.NodeKind = "node"
			payload["node"] = *e.Node
		}
		if e.Workflow != nil {
			payload["workflow"] = e.Workflow
		}
		env.Payload = payload
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowNodeStartedEvent:
		env := baseEnvelope(evt, "workflow.node.started")
		if env == nil {
			return nil
		}
		env.WorkflowID = workflowIDFromSnapshot(e.Workflow)
		env.RunID = env.WorkflowID

		isStep := e.StepDescription != "" || e.StepIndex > 0 || e.Workflow != nil || e.Input != nil
		payload := map[string]any{}
		if isStep {
			env.NodeKind = "step"
			env.NodeID = e.StepDescription
			payload["step_index"] = e.StepIndex
			payload["step_description"] = e.StepDescription
			if e.Iteration > 0 {
				payload["iteration"] = e.Iteration
			}
			if e.Workflow != nil {
				payload["workflow"] = e.Workflow
			}
		} else {
			env.NodeKind = "iteration"
			env.NodeID = iterationNodeID(e.Iteration)
			payload["iteration"] = e.Iteration
			payload["total_iters"] = e.TotalIters
		}
		env.Payload = payload
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowNodeCompletedEvent:
		status := strings.ToLower(strings.TrimSpace(e.Status))
		eventType := "workflow.node.completed"
		if status == string(workflow.NodeStatusFailed) || status == "failed" {
			eventType = "workflow.node.failed"
		}
		env := baseEnvelope(evt, eventType)
		if env == nil {
			return nil
		}
		env.WorkflowID = workflowIDFromSnapshot(e.Workflow)
		env.RunID = env.WorkflowID

		isStep := e.StepDescription != "" || e.StepIndex > 0 || e.Workflow != nil
		payload := map[string]any{}
		if isStep {
			env.NodeKind = "step"
			env.NodeID = e.StepDescription
			payload["step_index"] = e.StepIndex
			payload["step_description"] = e.StepDescription
			payload["status"] = e.Status
			if e.Iteration > 0 {
				payload["iteration"] = e.Iteration
			}
			if e.StepResult != nil {
				payload["result"] = e.StepResult
			}
			if e.Workflow != nil {
				payload["workflow"] = e.Workflow
			}
		} else {
			env.NodeKind = "iteration"
			env.NodeID = iterationNodeID(e.Iteration)
			payload["iteration"] = e.Iteration
			if e.TokensUsed > 0 {
				payload["tokens_used"] = e.TokensUsed
			}
			if e.ToolsRun > 0 {
				payload["tools_run"] = e.ToolsRun
			}
		}
		env.Payload = payload
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowNodeOutputSummaryEvent:
		env := baseEnvelope(evt, "workflow.node.output.summary")
		if env == nil {
			return nil
		}
		env.NodeKind = "generation"
		env.Payload = map[string]any{
			"iteration":       e.Iteration,
			"content":         e.Content,
			"tool_call_count": e.ToolCallCount,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowNodeOutputDeltaEvent:
		env := baseEnvelope(evt, "workflow.node.output.delta")
		if env == nil {
			return nil
		}
		env.NodeKind = "generation"
		payload := map[string]any{
			"iteration": e.Iteration,
			"delta":     e.Delta,
			"final":     e.Final,
		}
		if !e.CreatedAt.IsZero() {
			payload["created_at"] = e.CreatedAt
		}
		if e.SourceModel != "" {
			payload["source_model"] = e.SourceModel
		}
		if e.MessageCount > 0 {
			payload["message_count"] = e.MessageCount
		}
		env.Payload = payload
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowToolStartedEvent:
		env := baseEnvelope(evt, "workflow.tool.started")
		if env == nil {
			return nil
		}
		env.NodeKind = "tool"
		env.NodeID = e.CallID
		env.Payload = map[string]any{
			"call_id":   e.CallID,
			"tool_name": e.ToolName,
			"arguments": e.Arguments,
			"iteration": e.Iteration,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowToolProgressEvent:
		env := baseEnvelope(evt, "workflow.tool.progress")
		if env == nil {
			return nil
		}
		env.NodeKind = "tool"
		env.NodeID = e.CallID
		env.Payload = map[string]any{
			"call_id":     e.CallID,
			"chunk":       e.Chunk,
			"is_complete": e.IsComplete,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowToolCompletedEvent:
		env := baseEnvelope(evt, "workflow.tool.completed")
		if env == nil {
			return nil
		}
		env.NodeKind = "tool"
		env.NodeID = e.CallID
		payload := map[string]any{
			"call_id":     e.CallID,
			"tool_name":   e.ToolName,
			"result":      e.Result,
			"duration":    e.Duration.Milliseconds(),
			"metadata":    e.Metadata,
			"attachments": e.Attachments,
		}
		if e.Error != nil {
			payload["error"] = e.Error.Error()
		}
		env.Payload = payload
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowResultFinalEvent:
		env := baseEnvelope(evt, "workflow.result.final")
		if env == nil {
			return nil
		}
		env.NodeKind = "result"
		env.Payload = map[string]any{
			"final_answer":     e.FinalAnswer,
			"total_iterations": e.TotalIterations,
			"total_tokens":     e.TotalTokens,
			"stop_reason":      e.StopReason,
			"duration":         e.Duration.Milliseconds(),
			"is_streaming":     e.IsStreaming,
			"stream_finished":  e.StreamFinished,
			"attachments":      e.Attachments,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowResultCancelledEvent:
		env := baseEnvelope(evt, "workflow.result.cancelled")
		if env == nil {
			return nil
		}
		env.NodeKind = "result"
		env.Payload = map[string]any{
			"reason":       e.Reason,
			"requested_by": e.RequestedBy,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowNodeFailedEvent:
		env := baseEnvelope(evt, "workflow.node.failed")
		if env == nil {
			return nil
		}
		env.NodeKind = "diagnostic"
		env.Payload = map[string]any{
			"iteration":   e.Iteration,
			"phase":       e.Phase,
			"recoverable": e.Recoverable,
		}
		if e.Error != nil {
			env.Payload["error"] = e.Error.Error()
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowDiagnosticContextCompressionEvent:
		env := baseEnvelope(evt, "workflow.diagnostic.context_compression")
		if env == nil {
			return nil
		}
		env.NodeKind = "diagnostic"
		env.Payload = map[string]any{
			"original_count":   e.OriginalCount,
			"compressed_count": e.CompressedCount,
			"compression_rate": e.CompressionRate,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowDiagnosticToolFilteringEvent:
		env := baseEnvelope(evt, "workflow.diagnostic.tool_filtering")
		if env == nil {
			return nil
		}
		env.NodeKind = "diagnostic"
		env.Payload = map[string]any{
			"preset_name":       e.PresetName,
			"original_count":    e.OriginalCount,
			"filtered_count":    e.FilteredCount,
			"filtered_tools":    e.FilteredTools,
			"tool_filter_ratio": e.ToolFilterRatio,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowDiagnosticBrowserInfoEvent:
		env := baseEnvelope(evt, "workflow.diagnostic.browser_info")
		if env == nil {
			return nil
		}
		env.NodeKind = "diagnostic"
		env.Payload = map[string]any{
			"success":         e.Success,
			"message":         e.Message,
			"user_agent":      e.UserAgent,
			"cdp_url":         e.CDPURL,
			"vnc_url":         e.VNCURL,
			"viewport_width":  e.ViewportWidth,
			"viewport_height": e.ViewportHeight,
			"captured":        e.Captured,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowDiagnosticEnvironmentSnapshotEvent:
		env := baseEnvelope(evt, "workflow.diagnostic.environment_snapshot")
		if env == nil {
			return nil
		}
		env.NodeKind = "diagnostic"
		env.Payload = map[string]any{
			"host":     e.Host,
			"sandbox":  e.Sandbox,
			"captured": e.Captured,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowDiagnosticSandboxProgressEvent:
		env := baseEnvelope(evt, "workflow.diagnostic.sandbox_progress")
		if env == nil {
			return nil
		}
		env.NodeKind = "diagnostic"
		env.Payload = map[string]any{
			"status":      e.Status,
			"stage":       e.Stage,
			"message":     e.Message,
			"step":        e.Step,
			"total_steps": e.TotalSteps,
			"error":       e.Error,
			"updated":     e.Updated,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.WorkflowInputReceivedEvent:
		env := baseEnvelope(evt, "workflow.input.received")
		if env == nil {
			return nil
		}
		env.NodeKind = "input"
		env.Payload = map[string]any{
			"task":        e.Task,
			"attachments": e.Attachments,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case ports.SubtaskWrapper:
		return t.translateSubtaskEvent(e)
	default:
		return nil
	}
}

func (t *workflowEventTranslator) translateSubtaskEvent(event ports.SubtaskWrapper) []*domain.WorkflowEventEnvelope {
	if event == nil || event.WrappedEvent() == nil {
		return nil
	}

	eventType := "workflow.subflow.progress"
	switch event.WrappedEvent().(type) {
	case *domain.WorkflowResultFinalEvent, *domain.WorkflowResultCancelledEvent:
		eventType = "workflow.subflow.completed"
	}

	env := domain.NewWorkflowEnvelopeFromEvent(event, eventType)
	if env == nil {
		return nil
	}
	env.NodeKind = "subflow"
	details := event.SubtaskDetails()
	env.NodeID = subflowNodeID(details.Index)
	env.IsSubtask = true
	env.SubtaskIndex = details.Index
	env.TotalSubtasks = details.Total
	env.SubtaskPreview = details.Preview
	env.MaxParallel = details.MaxParallel
	payload := map[string]any{
		"subtask_index":   details.Index,
		"total_subtasks":  details.Total,
		"subtask_preview": details.Preview,
		"max_parallel":    details.MaxParallel,
	}

	stats := t.recordSubflowStats(event, details)
	payload["completed"] = stats.completed
	payload["total"] = stats.total
	payload["tokens"] = stats.tokens
	payload["tool_calls"] = stats.toolCalls

	if eventType == "workflow.subflow.completed" {
		payload["success"] = stats.success
		payload["failed"] = stats.failed
	}

	switch inner := event.WrappedEvent().(type) {
	case *domain.WorkflowResultFinalEvent:
		payload["final_answer"] = inner.FinalAnswer
		payload["stop_reason"] = inner.StopReason
		payload["attachments"] = inner.Attachments
	case *domain.WorkflowResultCancelledEvent:
		payload["cancel_reason"] = inner.Reason
		payload["requested_by"] = inner.RequestedBy
	}

	env.Payload = payload
	return []*domain.WorkflowEventEnvelope{env}
}

func (t *workflowEventTranslator) recordSubflowStats(event ports.SubtaskWrapper, details ports.SubtaskMetadata) subflowSnapshot {
	if t == nil || t.subflowTracker == nil {
		return subflowSnapshot{total: details.Total}
	}
	return t.subflowTracker.snapshot(event, details)
}

func baseEnvelope(evt ports.AgentEvent, eventType string) *domain.WorkflowEventEnvelope {
	if eventType == "" {
		return nil
	}
	return domain.NewWorkflowEnvelopeFromEvent(evt, eventType)
}

func workflowIDFromSnapshot(snapshot *workflow.WorkflowSnapshot) string {
	if snapshot == nil {
		return ""
	}
	return snapshot.ID
}

func iterationNodeID(iteration int) string {
	if iteration <= 0 {
		return "iteration"
	}
	return fmt.Sprintf("iteration-%d", iteration)
}

func subflowNodeID(index int) string {
	if index < 0 {
		return "subflow"
	}
	return fmt.Sprintf("subflow-%d", index)
}

// subflowStatsTracker aggregates per-subflow counters so subflow envelopes can
// mirror progress payloads.
type subflowStatsTracker struct {
	mu    sync.Mutex
	flows map[string]*subflowState
}

type subflowState struct {
	total int
	tasks map[int]*subflowTaskState
}

type subflowTaskState struct {
	toolCalls int
	tokens    int
	done      bool
	failed    bool
}

type subflowSnapshot struct {
	total     int
	completed int
	success   int
	failed    int
	tokens    int
	toolCalls int
}

func newSubflowStatsTracker() *subflowStatsTracker {
	return &subflowStatsTracker{
		flows: make(map[string]*subflowState),
	}
}

func (t *subflowStatsTracker) snapshot(event ports.SubtaskWrapper, details ports.SubtaskMetadata) subflowSnapshot {
	if event == nil || event.WrappedEvent() == nil {
		return subflowSnapshot{total: details.Total}
	}

	key := t.flowKey(event)

	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.flows[key]
	if !ok {
		state = &subflowState{
			total: details.Total,
			tasks: make(map[int]*subflowTaskState),
		}
		t.flows[key] = state
	}

	if details.Total > 0 && details.Total > state.total {
		state.total = details.Total
	}

	task := state.tasks[details.Index]
	if task == nil {
		task = &subflowTaskState{}
		state.tasks[details.Index] = task
	}

	switch inner := event.WrappedEvent().(type) {
	case *domain.WorkflowToolCompletedEvent:
		if !task.done {
			task.toolCalls++
		}
	case *domain.WorkflowResultFinalEvent:
		if inner.IsStreaming && !inner.StreamFinished {
			break
		}
		task.done = true
		task.failed = false
		task.tokens = inner.TotalTokens
	case *domain.WorkflowResultCancelledEvent:
		task.done = true
		task.failed = true
	case *domain.WorkflowNodeFailedEvent:
		task.done = true
		task.failed = true
	}

	snapshot := state.snapshot(details)
	if snapshot.total == 0 {
		snapshot.total = details.Total
	}
	if snapshot.total < snapshot.completed {
		snapshot.total = snapshot.completed
	}

	return snapshot
}

func (t *subflowStatsTracker) flowKey(event ports.SubtaskWrapper) string {
	if event == nil {
		return "subflow"
	}
	if parent := event.GetParentTaskID(); parent != "" {
		return parent
	}
	if task := event.GetTaskID(); task != "" {
		return task
	}
	if session := event.GetSessionID(); session != "" {
		return session
	}
	return "subflow"
}

func (s *subflowState) snapshot(details ports.SubtaskMetadata) subflowSnapshot {
	total := s.total
	if total == 0 {
		total = details.Total
	}
	if total == 0 {
		total = len(s.tasks)
	}

	snap := subflowSnapshot{total: total}
	for _, task := range s.tasks {
		snap.toolCalls += task.toolCalls
		if task.done {
			snap.completed++
			if task.failed {
				snap.failed++
			} else {
				snap.success++
			}
			snap.tokens += task.tokens
		}
	}
	return snap
}
