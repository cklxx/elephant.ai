package app

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/workflow"
)

// wrapWithWorkflowEnvelope decorates the provided listener with a translator that
// emits semantic workflow.* envelope events alongside legacy domain events.
func wrapWithWorkflowEnvelope(listener ports.EventListener, logger *slog.Logger) ports.EventListener {
	if listener == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &workflowEventTranslator{sink: listener, logger: logger}
}

type workflowEventTranslator struct {
	sink   ports.EventListener
	logger *slog.Logger
}

func (t *workflowEventTranslator) OnEvent(evt ports.AgentEvent) {
	if evt == nil || t.sink == nil {
		return
	}

	// Always forward the legacy event first.
	t.sink.OnEvent(evt)

	// Avoid re-wrapping envelopes that already follow the new contract.
	if _, ok := evt.(*domain.WorkflowEventEnvelope); ok {
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
	case *domain.WorkflowLifecycleEvent:
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

	case *domain.StepStartedEvent:
		env := baseEnvelope(evt, "workflow.node.started")
		if env == nil {
			return nil
		}
		env.WorkflowID = workflowIDFromSnapshot(e.Workflow)
		env.RunID = env.WorkflowID
		env.NodeID = e.StepDescription
		env.NodeKind = "step"
		payload := map[string]any{
			"step_index":       e.StepIndex,
			"step_description": e.StepDescription,
		}
		if e.Iteration > 0 {
			payload["iteration"] = e.Iteration
		}
		if e.Workflow != nil {
			payload["workflow"] = e.Workflow
		}
		env.Payload = payload
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.StepCompletedEvent:
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
		env.NodeID = e.StepDescription
		env.NodeKind = "step"
		payload := map[string]any{
			"step_index":       e.StepIndex,
			"step_description": e.StepDescription,
			"status":           e.Status,
		}
		if e.Iteration > 0 {
			payload["iteration"] = e.Iteration
		}
		if e.StepResult != nil {
			payload["result"] = e.StepResult
		}
		if e.Workflow != nil {
			payload["workflow"] = e.Workflow
		}
		env.Payload = payload

		envelopes := []*domain.WorkflowEventEnvelope{env}
		if isPlanStep(e.StepDescription) {
			plan := baseEnvelope(evt, "workflow.plan.generated")
			if plan != nil {
				plan.WorkflowID = env.WorkflowID
				plan.RunID = env.RunID
				plan.NodeID = e.StepDescription
				plan.NodeKind = "plan"
				plan.Payload = map[string]any{
					"step_index": e.StepIndex,
					"iteration":  e.Iteration,
					"plan":       e.StepResult,
				}
				envelopes = append(envelopes, plan)
			}
		}
		return envelopes

	case *domain.ThinkCompleteEvent:
		env := baseEnvelope(evt, "workflow.node.output.summary")
		if env == nil {
			return nil
		}
		env.NodeKind = "generation"
		env.Payload = map[string]any{
			"iteration":         e.Iteration,
			"content":           e.Content,
			"tool_call_count":   e.ToolCallCount,
			"legacy_event_type": e.EventType(),
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.AssistantMessageEvent:
		env := baseEnvelope(evt, "workflow.node.output.delta")
		if env == nil {
			return nil
		}
		env.NodeKind = "generation"
		payload := map[string]any{
			"iteration": e.Iteration,
			"delta":     e.Delta,
			"final":     e.Final,
			"created_at": func(ts time.Time) any {
				if ts.IsZero() {
					return nil
				}
				return ts
			}(e.CreatedAt),
		}
		if e.SourceModel != "" {
			payload["source_model"] = e.SourceModel
		}
		env.Payload = payload
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.ToolCallStartEvent:
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

	case *domain.ToolCallStreamEvent:
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

	case *domain.ToolCallCompleteEvent:
		eventType := "workflow.tool.completed"
		if e.Error != nil {
			eventType = "workflow.tool.failed"
		}
		env := baseEnvelope(evt, eventType)
		if env == nil {
			return nil
		}
		env.NodeKind = "tool"
		env.NodeID = e.CallID
		payload := map[string]any{
			"call_id":     e.CallID,
			"tool_name":   e.ToolName,
			"result":      e.Result,
			"duration_ms": e.Duration.Milliseconds(),
			"metadata":    e.Metadata,
			"attachments": e.Attachments,
		}
		if e.Error != nil {
			payload["error"] = e.Error.Error()
		}
		env.Payload = payload
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.IterationStartEvent:
		env := baseEnvelope(evt, "workflow.iteration.started")
		if env == nil {
			return nil
		}
		env.NodeKind = "iteration"
		env.NodeID = iterationNodeID(e.Iteration)
		env.Payload = map[string]any{
			"iteration":    e.Iteration,
			"total_iters":  e.TotalIters,
			"legacy_event": e.EventType(),
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.IterationCompleteEvent:
		env := baseEnvelope(evt, "workflow.iteration.completed")
		if env == nil {
			return nil
		}
		env.NodeKind = "iteration"
		env.NodeID = iterationNodeID(e.Iteration)
		env.Payload = map[string]any{
			"iteration":   e.Iteration,
			"tokens_used": e.TokensUsed,
			"tools_run":   e.ToolsRun,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.TaskCompleteEvent:
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
			"duration_ms":      e.Duration.Milliseconds(),
			"is_streaming":     e.IsStreaming,
			"stream_finished":  e.StreamFinished,
			"attachments":      e.Attachments,
		}
		return []*domain.WorkflowEventEnvelope{env}

	case *domain.TaskCancelledEvent:
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

	case *domain.ErrorEvent:
		env := baseEnvelope(evt, "workflow.diagnostic.error")
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

	case *domain.ContextCompressionEvent:
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

	case *domain.ToolFilteringEvent:
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

	case *domain.BrowserInfoEvent:
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

	case *domain.EnvironmentSnapshotEvent:
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

	case *domain.SandboxProgressEvent:
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

	case *domain.UserTaskEvent:
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
	case *domain.TaskCompleteEvent, *domain.TaskCancelledEvent:
		eventType = "workflow.subflow.completed"
	}

	env := domain.NewWorkflowEnvelopeFromEvent(event, eventType)
	if env == nil {
		return nil
	}
	env.NodeKind = "subflow"
	details := event.SubtaskDetails()
	env.NodeID = subflowNodeID(details.Index)
	payload := map[string]any{
		"subtask_index":       details.Index,
		"total_subtasks":      details.Total,
		"subtask_preview":     details.Preview,
		"max_parallel":        details.MaxParallel,
		"original_event_type": event.WrappedEvent().EventType(),
	}

	switch inner := event.WrappedEvent().(type) {
	case *domain.TaskCompleteEvent:
		payload["final_answer"] = inner.FinalAnswer
		payload["stop_reason"] = inner.StopReason
		payload["attachments"] = inner.Attachments
	case *domain.TaskCancelledEvent:
		payload["cancel_reason"] = inner.Reason
		payload["requested_by"] = inner.RequestedBy
	}

	env.Payload = payload
	return []*domain.WorkflowEventEnvelope{env}
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

func isPlanStep(stepID string) bool {
	step := strings.ToLower(strings.TrimSpace(stepID))
	return step != "" && strings.Contains(step, ":plan")
}
