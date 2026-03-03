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
)

// wrapWithWorkflowEnvelope decorates the provided listener with a translator that
// converts domain workflow events into the `domain.WorkflowEventEnvelope` contract
// consumed by downstream adapters (SSE, CLI bridges, replay stores, etc.).
func wrapWithWorkflowEnvelope(listener agent.EventListener, logger *slog.Logger) agent.EventListener {
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
	sink           agent.EventListener
	logger         *slog.Logger
	subflowTracker *subflowStatsTracker

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
			"task_id":      d.TaskID,
			"description":  d.Description,
			"status":       d.Status,
			"answer":       d.Answer,
			"error":        d.ErrorStr,
			"merge_status": d.MergeStatus,
			"duration":     d.Duration.Milliseconds(),
			"iterations":   d.Iterations,
			"tokens_used":  d.TokensUsed,
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
