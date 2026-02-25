package coordinator

import (
	"strings"
	"sync"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	domain "alex/internal/domain/agent"
	"alex/internal/domain/workflow"
)

func (t *workflowEventTranslator) translateLifecycle(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"workflow_event_type": string(d.WorkflowEventType),
	}
	if d.Phase != "" {
		payload["phase"] = d.Phase
	}
	nID := ""
	if d.Node != nil {
		nID = d.Node.ID
		payload["node"] = *d.Node
	}

	return t.workflowEnvelopeFromOptions(evt, envelopeOptions{
		snapshot:       d.Workflow,
		eventType:      types.EventLifecycleUpdated,
		nodeKind:       "node",
		nodeID:         nID,
		payload:        payload,
		attachWorkflow: true,
		skipRecorder:   true,
	})
}

func (t *workflowEventTranslator) translateNodeStarted(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	if isToolRecorderNodeID(d.StepDescription) {
		return nil
	}

	return t.nodeEnvelope(evt, types.EventNodeStarted, nodeEventMeta{
		stepDescription: d.StepDescription,
		stepIndex:       d.StepIndex,
		iteration:       d.Iteration,
		totalIters:      d.TotalIters,
		workflow:        d.Workflow,
		hasInput:        d.Input != nil,
	}, nil)
}

func (t *workflowEventTranslator) translateNodeCompleted(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	if isToolRecorderNodeID(d.StepDescription) {
		return nil
	}

	status := strings.ToLower(strings.TrimSpace(d.Status))
	eventType := types.EventNodeCompleted
	if status == string(workflow.NodeStatusFailed) || status == "failed" {
		eventType = types.EventNodeFailed
	}

	return t.nodeEnvelope(evt, eventType, nodeEventMeta{
		stepDescription: d.StepDescription,
		stepIndex:       d.StepIndex,
		iteration:       d.Iteration,
		workflow:        d.Workflow,
	}, func(payload map[string]any) {
		if d.StepResult != nil {
			payload["result"] = d.StepResult
		}
		if d.TokensUsed > 0 {
			payload["tokens_used"] = d.TokensUsed
		}
		if d.ToolsRun > 0 {
			payload["tools_run"] = d.ToolsRun
		}
		if d.Duration > 0 {
			payload["duration_ms"] = d.Duration.Milliseconds()
		}
		if status != "" {
			payload["status"] = d.Status
		}
	})
}

func (t *workflowEventTranslator) translateNodeOutputSummary(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"iteration":       d.Iteration,
		"content":         d.Content,
		"tool_call_count": d.ToolCallCount,
	}
	for key, val := range d.Metadata {
		if _, exists := payload[key]; exists {
			continue
		}
		payload[key] = val
	}
	return t.singleEnvelope(evt, types.EventNodeOutputSummary, "generation", "", payload)
}

func (t *workflowEventTranslator) translateNodeOutputDelta(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"iteration": d.Iteration,
		"delta":     d.Delta,
		"final":     d.Final,
	}
	if !d.CreatedAt.IsZero() {
		payload["created_at"] = d.CreatedAt
	}
	if d.SourceModel != "" {
		payload["source_model"] = d.SourceModel
	}
	if d.MessageCount > 0 {
		payload["message_count"] = d.MessageCount
	}

	return t.singleEnvelope(evt, types.EventNodeOutputDelta, "generation", "", payload)
}

func (t *workflowEventTranslator) translateResultFinal(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	return t.singleEnvelope(evt, types.EventResultFinal, "result", stageSummarize, map[string]any{
		"final_answer":     d.FinalAnswer,
		"total_iterations": d.TotalIterations,
		"total_tokens":     d.TotalTokens,
		"stop_reason":      d.StopReason,
		"duration":         d.Duration.Milliseconds(),
		"is_streaming":     d.IsStreaming,
		"stream_finished":  d.StreamFinished,
		"attachments":      d.Attachments,
	})
}

func (t *workflowEventTranslator) translateResultCancelled(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	return t.singleEnvelope(evt, types.EventResultCancelled, "result", "", map[string]any{
		"reason":       d.Reason,
		"requested_by": d.RequestedBy,
	})
}

func (t *workflowEventTranslator) translateNodeFailure(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"iteration":   d.Iteration,
		"phase":       d.PhaseLabel,
		"recoverable": d.Recoverable,
	}

	if d.Error != nil {
		payload["error"] = d.Error.Error()
	}

	return t.diagnosticEnvelope(evt, types.EventNodeFailed, payload)
}

func (t *workflowEventTranslator) translateSubtaskEvent(event agent.SubtaskWrapper) []*domain.WorkflowEventEnvelope {
	if event == nil || event.WrappedEvent() == nil {
		return nil
	}

	eventType := types.EventSubflowProgress
	if inner, ok := event.WrappedEvent().(*domain.Event); ok {
		switch inner.Kind {
		case types.EventResultFinal, types.EventResultCancelled:
			eventType = types.EventSubflowCompleted
		}
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

	if eventType == types.EventSubflowCompleted {
		payload["success"] = stats.success
		payload["failed"] = stats.failed
	}

	if inner, ok := event.WrappedEvent().(*domain.Event); ok {
		switch inner.Kind {
		case types.EventResultFinal:
			payload["final_answer"] = inner.Data.FinalAnswer
			payload["stop_reason"] = inner.Data.StopReason
			payload["attachments"] = inner.Data.Attachments
		case types.EventResultCancelled:
			payload["cancel_reason"] = inner.Data.Reason
			payload["requested_by"] = inner.Data.RequestedBy
		}
	}

	env.Payload = payload
	return []*domain.WorkflowEventEnvelope{env}
}

func (t *workflowEventTranslator) recordSubflowStats(event agent.SubtaskWrapper, details agent.SubtaskMetadata) subflowSnapshot {
	if t == nil || t.subflowTracker == nil {
		return subflowSnapshot{total: details.Total}
	}
	return t.subflowTracker.snapshot(event, details)
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

func (t *subflowStatsTracker) snapshot(event agent.SubtaskWrapper, details agent.SubtaskMetadata) subflowSnapshot {
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

	if inner, ok := event.WrappedEvent().(*domain.Event); ok {
		switch inner.Kind {
		case types.EventToolCompleted:
			if !task.done {
				task.toolCalls++
			}
		case types.EventResultFinal:
			if inner.Data.IsStreaming && !inner.Data.StreamFinished {
				break
			}
			task.done = true
			task.failed = false
			task.tokens = inner.Data.TotalTokens
		case types.EventResultCancelled:
			task.done = true
			task.failed = true
		case types.EventNodeFailed:
			task.done = true
			task.failed = true
		}
	}

	snapshot := state.snapshot(details)
	if snapshot.total == 0 {
		snapshot.total = details.Total
	}
	if snapshot.total < snapshot.completed {
		snapshot.total = snapshot.completed
	}

	// Reclaim memory once all tasks in a subflow have finished.
	if snapshot.total > 0 && snapshot.completed >= snapshot.total {
		delete(t.flows, key)
	}

	return snapshot
}

func (t *subflowStatsTracker) flowKey(event agent.SubtaskWrapper) string {
	if event == nil {
		return "subflow"
	}
	if parent := event.GetParentRunID(); parent != "" {
		return parent
	}
	if task := event.GetRunID(); task != "" {
		return task
	}
	if session := event.GetSessionID(); session != "" {
		return session
	}
	return "subflow"
}

func (s *subflowState) snapshot(details agent.SubtaskMetadata) subflowSnapshot {
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
