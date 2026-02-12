package coordinator

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/domain/workflow"
	toolspolicy "alex/internal/infra/tools"
)

// wrapWithWorkflowEnvelope decorates the provided listener with a translator that
// converts domain workflow events into the `domain.WorkflowEventEnvelope` contract
// consumed by downstream adapters (SSE, CLI bridges, replay stores, etc.).
func wrapWithWorkflowEnvelope(listener agent.EventListener, logger *slog.Logger, slaCollectors ...*toolspolicy.SLACollector) agent.EventListener {
	if listener == nil {
		return nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	var collector *toolspolicy.SLACollector
	if len(slaCollectors) > 0 {
		collector = slaCollectors[0]
	}
	return &workflowEventTranslator{
		sink:           listener,
		logger:         logger,
		subflowTracker: newSubflowStatsTracker(),
		slaCollector:   collector,
	}
}

type workflowEventTranslator struct {
	sink           agent.EventListener
	logger         *slog.Logger
	subflowTracker *subflowStatsTracker
	slaCollector   *toolspolicy.SLACollector

	ctxMu sync.RWMutex
	ctx   workflowEnvelopeContext
}

func (t *workflowEventTranslator) OnEvent(evt agent.AgentEvent) {
	if evt == nil || t.sink == nil {
		return
	}

	// Avoid re-wrapping envelopes that already follow the new contract.
	if _, ok := evt.(*domain.WorkflowEventEnvelope); ok {
		t.sink.OnEvent(evt)
		return
	}

	// Unified events: pass through lightweight diagnostic signals that should
	// not be wrapped in an envelope (context snapshots, pre-analysis emoji).
	if e, ok := evt.(*domain.Event); ok {
		switch e.Kind {
		case types.EventDiagnosticContextSnapshot, types.EventDiagnosticPreanalysisEmoji:
			t.sink.OnEvent(evt)
			return
		}
	}

	for _, envelope := range t.translate(evt) {
		if envelope == nil {
			continue
		}
		t.sink.OnEvent(envelope)
	}
}

func (t *workflowEventTranslator) translate(evt agent.AgentEvent) []*domain.WorkflowEventEnvelope {
	// Handle unified Event type via Kind discriminator.
	if e, ok := evt.(*domain.Event); ok {
		return t.translateUnified(evt, e)
	}

	// Handle SubtaskWrapper (wraps inner events).
	if sw, ok := evt.(agent.SubtaskWrapper); ok {
		return t.translateSubtaskEvent(sw)
	}

	return nil
}

func (t *workflowEventTranslator) translateUnified(evt agent.AgentEvent, e *domain.Event) []*domain.WorkflowEventEnvelope {
	d := &e.Data
	switch e.Kind {
	case types.EventLifecycleUpdated:
		return t.translateLifecycle(evt, d)

	case types.EventNodeStarted:
		return t.translateNodeStarted(evt, d)

	case types.EventNodeCompleted:
		return t.translateNodeCompleted(evt, d)

	case types.EventNodeOutputSummary:
		return t.translateNodeOutputSummary(evt, d)

	case types.EventNodeOutputDelta:
		return t.translateNodeOutputDelta(evt, d)

	case types.EventToolStarted:
		return t.translateTool(evt, types.EventToolStarted, d.CallID, map[string]any{
			"tool_name": d.ToolName,
			"arguments": d.Arguments,
			"iteration": d.Iteration,
		})

	case types.EventToolProgress:
		return t.translateTool(evt, types.EventToolProgress, d.CallID, map[string]any{
			"chunk":       d.Chunk,
			"is_complete": d.IsComplete,
		})

	case types.EventToolCompleted:
		return t.translateToolComplete(evt, d)

	case types.EventReplanRequested:
		return t.singleEnvelope(evt, types.EventReplanRequested, "orchestrator", "replan", map[string]any{
			"call_id":   d.CallID,
			"tool_name": d.ToolName,
			"reason":    d.Reason,
			"error":     d.ErrorStr,
		})

	case types.EventResultFinal:
		return t.translateResultFinal(evt, d)

	case types.EventResultCancelled:
		return t.translateResultCancelled(evt, d)

	case types.EventNodeFailed:
		return t.translateNodeFailure(evt, d)

	case types.EventDiagnosticContextCompression:
		return t.diagnosticEnvelope(evt, types.EventDiagnosticContextCompression, map[string]any{
			"original_count":   d.OriginalCount,
			"compressed_count": d.CompressedCount,
			"compression_rate": d.CompressionRate,
		})

	case types.EventDiagnosticToolFiltering:
		return t.diagnosticEnvelope(evt, types.EventDiagnosticToolFiltering, map[string]any{
			"preset_name":       d.PresetName,
			"original_count":    d.OriginalCount,
			"filtered_count":    d.FilteredCount,
			"filtered_tools":    d.FilteredTools,
			"tool_filter_ratio": d.ToolFilterRatio,
		})

	case types.EventDiagnosticEnvironmentSnapshot:
		return t.diagnosticEnvelope(evt, types.EventDiagnosticEnvironmentSnapshot, map[string]any{
			"host":     d.Host,
			"captured": d.Captured,
		})

	case types.EventInputReceived:
		return t.singleEnvelope(evt, types.EventInputReceived, "input", "", map[string]any{
			"task":        d.Task,
			"attachments": d.Attachments,
		})

	case types.EventProactiveContextRefresh:
		return t.singleEnvelope(evt, types.EventProactiveContextRefresh, "diagnostic", "", map[string]any{
			"iteration":         d.Iteration,
			"memories_injected": d.MemoriesInjected,
		})

	case types.EventBackgroundTaskDispatched:
		return t.singleEnvelope(evt, types.EventBackgroundTaskDispatched, "background", d.TaskID, map[string]any{
			"task_id":     d.TaskID,
			"description": d.Description,
			"prompt":      d.Prompt,
			"agent_type":  d.AgentType,
		})

	case types.EventBackgroundTaskCompleted:
		return t.singleEnvelope(evt, types.EventBackgroundTaskCompleted, "background", d.TaskID, map[string]any{
			"task_id":     d.TaskID,
			"description": d.Description,
			"status":      d.Status,
			"answer":      d.Answer,
			"error":       d.ErrorStr,
			"duration":    d.Duration.Milliseconds(),
			"iterations":  d.Iterations,
			"tokens_used": d.TokensUsed,
		})

	case types.EventExternalAgentProgress:
		return t.singleEnvelope(evt, types.EventExternalAgentProgress, "external_agent", d.TaskID, map[string]any{
			"task_id":       d.TaskID,
			"agent_type":    d.AgentType,
			"iteration":     d.Iteration,
			"max_iter":      d.MaxIter,
			"tokens_used":   d.TokensUsed,
			"cost_usd":      d.CostUSD,
			"current_tool":  d.CurrentTool,
			"current_args":  d.CurrentArgs,
			"files_touched": d.FilesTouched,
			"last_activity": d.LastActivity,
			"elapsed":       d.Elapsed.Milliseconds(),
		})

	case types.EventExternalInputRequested:
		return t.singleEnvelope(evt, types.EventExternalInputRequested, "external_input", d.TaskID, map[string]any{
			"task_id":    d.TaskID,
			"agent_type": d.AgentType,
			"request_id": d.RequestID,
			"type":       d.Type,
			"summary":    d.Summary,
		})

	case types.EventExternalInputResponded:
		return t.singleEnvelope(evt, types.EventExternalInputResponded, "external_input", d.TaskID, map[string]any{
			"task_id":    d.TaskID,
			"request_id": d.RequestID,
			"approved":   d.Approved,
			"option_id":  d.OptionID,
			"message":    d.Message,
		})

	default:
		return nil
	}
}

func (t *workflowEventTranslator) translateTool(evt agent.AgentEvent, eventType, callID string, payload map[string]any) []*domain.WorkflowEventEnvelope {
	return t.toolEnvelope(evt, eventType, callID, payload)
}

func (t *workflowEventTranslator) translateToolComplete(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"tool_name":   d.ToolName,
		"result":      d.Result,
		"duration":    d.Duration.Milliseconds(),
		"metadata":    d.Metadata,
		"attachments": d.Attachments,
	}
	if t != nil && t.slaCollector != nil {
		sla := t.slaCollector.GetSLA(d.ToolName)
		payload["tool_sla"] = map[string]any{
			"tool_name":      sla.ToolName,
			"p50_latency_ms": sla.P50Latency.Milliseconds(),
			"p95_latency_ms": sla.P95Latency.Milliseconds(),
			"p99_latency_ms": sla.P99Latency.Milliseconds(),
			"error_rate":     sla.ErrorRate,
			"call_count":     sla.CallCount,
			"success_rate":   sla.SuccessRate,
			"cost_usd_total": sla.CostUSDTotal,
			"cost_usd_avg":   sla.CostUSDAvg,
		}
	}
	if d.Error != nil {
		payload["error"] = d.Error.Error()
	}

	envelopes := t.toolEnvelope(evt, types.EventToolCompleted, d.CallID, payload)
	if manifestPayload := buildArtifactManifestPayload(d); manifestPayload != nil {
		envelopes = append(envelopes, t.singleEnvelope(evt, types.EventArtifactManifest, "artifact", "artifact-manifest", manifestPayload)...)
	}
	return envelopes
}

func buildArtifactManifestPayload(d *domain.EventData) map[string]any {
	if d == nil {
		return nil
	}
	toolName := strings.ToLower(strings.TrimSpace(d.ToolName))
	if toolName == "acp_executor" {
		return nil
	}
	attachments := d.Attachments
	if d.Metadata != nil {
		if manifest, ok := d.Metadata["artifact_manifest"]; ok {
			payload := map[string]any{
				"manifest":    manifest,
				"source_tool": d.ToolName,
			}
			if len(attachments) > 0 {
				payload["attachments"] = attachments
			}
			return payload
		}
	}
	if toolName == "artifact_manifest" {
		payload := map[string]any{
			"result":      d.Result,
			"source_tool": d.ToolName,
		}
		if len(attachments) > 0 {
			payload["attachments"] = attachments
		}
		return payload
	}
	if len(attachments) == 0 {
		return nil
	}
	for _, att := range attachments {
		format := strings.ToLower(strings.TrimSpace(att.Format))
		if format == "manifest" || strings.Contains(strings.ToLower(att.Name), "manifest") {
			return map[string]any{
				"attachments": attachments,
				"source_tool": d.ToolName,
			}
		}
	}
	return nil
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

type nodeEventMeta struct {
	stepDescription string
	stepIndex       int
	iteration       int
	totalIters      int
	workflow        *workflow.WorkflowSnapshot
	hasInput        bool
}

func (m nodeEventMeta) isStep() bool {
	if m.stepDescription != "" || m.stepIndex > 0 || m.workflow != nil {
		return true
	}
	return m.hasInput
}

func (t *workflowEventTranslator) nodeEnvelope(evt agent.AgentEvent, eventType string, meta nodeEventMeta, decorate func(map[string]any)) []*domain.WorkflowEventEnvelope {
	opts := envelopeOptions{
		snapshot:     meta.workflow,
		eventType:    eventType,
		skipRecorder: true,
	}

	payload := map[string]any{}
	if meta.isStep() {
		opts.nodeKind = "step"
		opts.nodeID = meta.stepDescription
		opts.attachWorkflow = true
		payload["step_index"] = meta.stepIndex
		payload["step_description"] = meta.stepDescription
		if meta.iteration > 0 {
			payload["iteration"] = meta.iteration
		}
	} else {
		opts.nodeKind = "iteration"
		opts.nodeID = iterationNodeID(meta.iteration)
		payload["iteration"] = meta.iteration
		if meta.totalIters > 0 {
			payload["total_iters"] = meta.totalIters
		}
	}

	if decorate != nil {
		decorate(payload)
	}
	opts.payload = payload

	return t.workflowEnvelopeFromOptions(evt, opts)
}

type workflowEnvelopeContext struct {
	snapshot   *workflow.WorkflowSnapshot
	workflowID string
}

func workflowContextFromEvent(evt agent.AgentEvent, snapshot *workflow.WorkflowSnapshot) workflowEnvelopeContext {
	var workflowID string

	if e, ok := evt.(*domain.Event); ok {
		switch e.Kind {
		case types.EventLifecycleUpdated:
			if snapshot == nil {
				snapshot = e.Data.Workflow
			}
			workflowID = e.Data.WorkflowID
		case types.EventNodeStarted, types.EventNodeCompleted:
			if snapshot == nil {
				snapshot = e.Data.Workflow
			}
		}
	}

	return newWorkflowEnvelopeContext(snapshot, workflowID)
}

func (t *workflowEventTranslator) resolveWorkflowContext(ctx workflowEnvelopeContext) workflowEnvelopeContext {
	if t == nil {
		return ctx
	}

	t.ctxMu.RLock()
	stored := t.ctx
	t.ctxMu.RUnlock()

	if ctx.snapshot == nil && stored.snapshot != nil {
		ctx.snapshot = stored.snapshot
	}
	if ctx.workflowID == "" {
		ctx.workflowID = stored.workflowID
	}

	return ctx
}

func (t *workflowEventTranslator) rememberWorkflowContext(ctx workflowEnvelopeContext) {
	if t == nil || (ctx.snapshot == nil && ctx.workflowID == "") {
		return
	}

	t.ctxMu.Lock()
	t.ctx = ctx
	t.ctxMu.Unlock()
}

func newWorkflowEnvelopeContext(snapshot *workflow.WorkflowSnapshot, workflowID string) workflowEnvelopeContext {
	return workflowEnvelopeContext{
		snapshot:   sanitizeWorkflowSnapshot(snapshot),
		workflowID: workflowIDFromSnapshot(snapshot, workflowID),
	}
}

func (c workflowEnvelopeContext) envelope(evt agent.AgentEvent, eventType, nodeKind, nodeID string, payload map[string]any) *domain.WorkflowEventEnvelope {
	return newEnvelope(evt, eventType, nodeKind, nodeID, c.snapshot, c.workflowID, payload)
}

func (c workflowEnvelopeContext) attachWorkflow(payload map[string]any) {
	if payload == nil || c.snapshot == nil {
		return
	}
	payload["workflow"] = c.snapshot
}

func (c workflowEnvelopeContext) shouldSkip(nodeID string) bool {
	return isToolRecorderNodeID(nodeID)
}

func (t *workflowEventTranslator) singleEnvelope(evt agent.AgentEvent, eventType, nodeKind, nodeID string, payload map[string]any) []*domain.WorkflowEventEnvelope {
	return t.workflowEnvelopeFromOptions(evt, envelopeOptions{
		eventType: eventType,
		nodeKind:  nodeKind,
		nodeID:    nodeID,
		payload:   payload,
	})
}

func (t *workflowEventTranslator) diagnosticEnvelope(evt agent.AgentEvent, eventType string, payload map[string]any) []*domain.WorkflowEventEnvelope {
	return t.singleEnvelope(evt, eventType, "diagnostic", "", payload)
}

func (t *workflowEventTranslator) toolEnvelope(evt agent.AgentEvent, eventType, callID string, payload map[string]any) []*domain.WorkflowEventEnvelope {
	if payload == nil {
		payload = map[string]any{}
	}
	if callID != "" {
		payload["call_id"] = callID
	}
	return t.singleEnvelope(evt, eventType, "tool", callID, payload)
}

func newEnvelope(evt agent.AgentEvent, eventType, nodeKind, nodeID string, snapshot *workflow.WorkflowSnapshot, workflowID string, payload map[string]any) *domain.WorkflowEventEnvelope {
	if eventType == "" {
		return nil
	}

	env := domain.NewWorkflowEnvelopeFromEvent(evt, eventType)
	if env == nil {
		return nil
	}

	setNodeKind(env, nodeKind)
	if nodeID != "" {
		env.NodeID = nodeID
	}

	id := workflowIDFromSnapshot(snapshot, workflowID)
	if id != "" {
		env.WorkflowID = id
		env.RunID = id
	}

	if len(payload) > 0 {
		env.Payload = payload
	}

	return env
}

func setNodeKind(env *domain.WorkflowEventEnvelope, kind string) {
	if env != nil && kind != "" {
		env.NodeKind = kind
	}
}

func nodeID(evt agent.AgentEvent, node *workflow.NodeSnapshot) string {
	if node != nil {
		return node.ID
	}
	if e, ok := evt.(*domain.Event); ok && e.Kind == types.EventLifecycleUpdated && e.Data.Node != nil {
		return e.Data.Node.ID
	}
	return ""
}

func (t *workflowEventTranslator) recordSubflowStats(event agent.SubtaskWrapper, details agent.SubtaskMetadata) subflowSnapshot {
	if t == nil || t.subflowTracker == nil {
		return subflowSnapshot{total: details.Total}
	}
	return t.subflowTracker.snapshot(event, details)
}

func sanitizeWorkflowSnapshot(snapshot *workflow.WorkflowSnapshot) *workflow.WorkflowSnapshot {
	if snapshot == nil {
		return nil
	}

	// Single pass: filter nodes and build a retained-ID set simultaneously.
	filteredNodes := make([]workflow.NodeSnapshot, 0, len(snapshot.Nodes))
	retained := make(map[string]struct{}, len(snapshot.Nodes))
	summary := map[string]int64{
		"pending":   0,
		"running":   0,
		"succeeded": 0,
		"failed":    0,
	}
	for _, n := range snapshot.Nodes {
		if isToolRecorderNodeID(n.ID) {
			continue
		}
		filteredNodes = append(filteredNodes, n)
		retained[n.ID] = struct{}{}
		switch n.Status {
		case workflow.NodeStatusPending:
			summary["pending"]++
		case workflow.NodeStatusRunning:
			summary["running"]++
		case workflow.NodeStatusSucceeded:
			summary["succeeded"]++
		case workflow.NodeStatusFailed:
			summary["failed"]++
		}
	}

	filteredOrder := make([]string, 0, len(filteredNodes))
	for _, id := range snapshot.Order {
		if _, ok := retained[id]; ok {
			filteredOrder = append(filteredOrder, id)
		}
	}

	return &workflow.WorkflowSnapshot{
		ID:          snapshot.ID,
		Phase:       snapshot.Phase,
		Order:       filteredOrder,
		Nodes:       filteredNodes,
		StartedAt:   snapshot.StartedAt,
		CompletedAt: snapshot.CompletedAt,
		Duration:    snapshot.Duration,
		Summary:     summary,
	}
}

func isToolRecorderNodeID(id string) bool {
	if id == "" || !strings.HasPrefix(id, "react:iter:") {
		return false
	}
	return strings.Contains(id, ":tools")
}

func workflowIDFromSnapshot(snapshot *workflow.WorkflowSnapshot, fallback string) string {
	if snapshot != nil && snapshot.ID != "" {
		return snapshot.ID
	}
	return fallback
}

type envelopeOptions struct {
	snapshot       *workflow.WorkflowSnapshot
	eventType      string
	nodeKind       string
	nodeID         string
	payload        map[string]any
	attachWorkflow bool
	skipRecorder   bool
}

func (t *workflowEventTranslator) workflowEnvelopeFromOptions(evt agent.AgentEvent, opts envelopeOptions) []*domain.WorkflowEventEnvelope {
	ctx := t.resolveWorkflowContext(workflowContextFromEvent(evt, opts.snapshot))
	if opts.skipRecorder && ctx.shouldSkip(opts.nodeID) {
		return nil
	}

	env := ctx.envelope(evt, opts.eventType, opts.nodeKind, opts.nodeID, nil)
	if env == nil {
		return nil
	}

	if opts.attachWorkflow {
		ctx.attachWorkflow(opts.payload)
	}

	if len(opts.payload) > 0 {
		env.Payload = opts.payload
	}

	t.rememberWorkflowContext(ctx)

	return []*domain.WorkflowEventEnvelope{env}
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
