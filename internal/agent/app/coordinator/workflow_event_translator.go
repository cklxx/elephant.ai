package coordinator

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
	"alex/internal/workflow"
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

	// Context snapshots are intentionally stored for replay/diagnostics but are not
	// streamed to the UI. They also don't follow the workflow envelope contract,
	// so forward them as-is.
	if _, ok := evt.(*domain.WorkflowDiagnosticContextSnapshotEvent); ok {
		t.sink.OnEvent(evt)
		return
	}

	// Pre-analysis emoji events are lightweight signals consumed by gateway
	// interceptors (e.g., Lark reaction). Pass through without envelope wrapping.
	if _, ok := evt.(*domain.WorkflowPreAnalysisEmojiEvent); ok {
		t.sink.OnEvent(evt)
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

func (t *workflowEventTranslator) translate(evt agent.AgentEvent) []*domain.WorkflowEventEnvelope {
	switch e := evt.(type) {
	case *domain.WorkflowLifecycleUpdatedEvent:
		return t.translateLifecycle(e, evt)

	case *domain.WorkflowNodeStartedEvent:
		return t.translateNodeStarted(e, evt)

	case *domain.WorkflowNodeCompletedEvent:
		return t.translateNodeCompleted(e, evt)

	case *domain.WorkflowNodeOutputSummaryEvent:
		return t.translateNodeOutputSummary(e, evt)

	case *domain.WorkflowNodeOutputDeltaEvent:
		return t.translateNodeOutputDelta(e, evt)

	case *domain.WorkflowToolStartedEvent:
		return t.translateTool(evt, types.EventToolStarted, e.CallID, map[string]any{
			"tool_name": e.ToolName,
			"arguments": e.Arguments,
			"iteration": e.Iteration,
		})

	case *domain.WorkflowToolProgressEvent:
		return t.translateTool(evt, types.EventToolProgress, e.CallID, map[string]any{
			"chunk":       e.Chunk,
			"is_complete": e.IsComplete,
		})

	case *domain.WorkflowToolCompletedEvent:
		return t.translateToolComplete(evt, e)

	case *domain.WorkflowResultFinalEvent:
		return t.translateResultFinal(evt, e)

	case *domain.WorkflowResultCancelledEvent:
		return t.translateResultCancelled(evt, e)

	case *domain.WorkflowNodeFailedEvent:
		return t.translateNodeFailure(evt, e)

	case *domain.WorkflowDiagnosticContextCompressionEvent:
		return t.diagnosticEnvelope(evt, types.EventDiagnosticContextCompression, map[string]any{
			"original_count":   e.OriginalCount,
			"compressed_count": e.CompressedCount,
			"compression_rate": e.CompressionRate,
		})

	case *domain.WorkflowDiagnosticToolFilteringEvent:
		return t.diagnosticEnvelope(evt, types.EventDiagnosticToolFiltering, map[string]any{
			"preset_name":       e.PresetName,
			"original_count":    e.OriginalCount,
			"filtered_count":    e.FilteredCount,
			"filtered_tools":    e.FilteredTools,
			"tool_filter_ratio": e.ToolFilterRatio,
		})

	case *domain.WorkflowDiagnosticEnvironmentSnapshotEvent:
		return t.diagnosticEnvelope(evt, types.EventDiagnosticEnvironmentSnapshot, map[string]any{
			"host":     e.Host,
			"captured": e.Captured,
		})

	case *domain.WorkflowInputReceivedEvent:
		return t.translateInputEnvelope(evt, e)

	case *domain.ProactiveContextRefreshEvent:
		return t.singleEnvelope(evt, types.EventProactiveContextRefresh, "diagnostic", "", map[string]any{
			"iteration":         e.Iteration,
			"memories_injected": e.MemoriesInjected,
		})

	case agent.SubtaskWrapper:
		return t.translateSubtaskEvent(e)
	default:
		return nil
	}
}

func (t *workflowEventTranslator) translateTool(evt agent.AgentEvent, eventType, callID string, payload map[string]any) []*domain.WorkflowEventEnvelope {
	return t.toolEnvelope(evt, eventType, callID, payload)
}

func (t *workflowEventTranslator) translateToolComplete(evt agent.AgentEvent, e *domain.WorkflowToolCompletedEvent) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"tool_name":   e.ToolName,
		"result":      e.Result,
		"duration":    e.Duration.Milliseconds(),
		"metadata":    e.Metadata,
		"attachments": e.Attachments,
	}
	if e.Error != nil {
		payload["error"] = e.Error.Error()
	}

	envelopes := t.toolEnvelope(evt, types.EventToolCompleted, e.CallID, payload)
	if manifestPayload := buildArtifactManifestPayload(e); manifestPayload != nil {
		envelopes = append(envelopes, t.singleEnvelope(evt, types.EventArtifactManifest, "artifact", "artifact-manifest", manifestPayload)...)
	}
	return envelopes
}

func buildArtifactManifestPayload(e *domain.WorkflowToolCompletedEvent) map[string]any {
	if e == nil {
		return nil
	}
	if strings.EqualFold(strings.TrimSpace(e.ToolName), "acp_executor") {
		return nil
	}
	attachments := e.Attachments
	if e.Metadata != nil {
		if manifest, ok := e.Metadata["artifact_manifest"]; ok {
			payload := map[string]any{
				"manifest":    manifest,
				"source_tool": e.ToolName,
			}
			if len(attachments) > 0 {
				payload["attachments"] = attachments
			}
			return payload
		}
	}
	if strings.EqualFold(strings.TrimSpace(e.ToolName), "artifact_manifest") {
		payload := map[string]any{
			"result":      e.Result,
			"source_tool": e.ToolName,
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
		if strings.EqualFold(strings.TrimSpace(att.Format), "manifest") ||
			strings.Contains(strings.ToLower(att.Name), "manifest") {
			return map[string]any{
				"attachments": attachments,
				"source_tool": e.ToolName,
			}
		}
	}
	return nil
}

func (t *workflowEventTranslator) translateResultFinal(evt agent.AgentEvent, e *domain.WorkflowResultFinalEvent) []*domain.WorkflowEventEnvelope {
	return t.singleEnvelope(evt, types.EventResultFinal, "result", stageSummarize, map[string]any{
		"final_answer":     e.FinalAnswer,
		"total_iterations": e.TotalIterations,
		"total_tokens":     e.TotalTokens,
		"stop_reason":      e.StopReason,
		"duration":         e.Duration.Milliseconds(),
		"is_streaming":     e.IsStreaming,
		"stream_finished":  e.StreamFinished,
		"attachments":      e.Attachments,
	})
}

func (t *workflowEventTranslator) translateResultCancelled(evt agent.AgentEvent, e *domain.WorkflowResultCancelledEvent) []*domain.WorkflowEventEnvelope {
	return t.singleEnvelope(evt, types.EventResultCancelled, "result", "", map[string]any{
		"reason":       e.Reason,
		"requested_by": e.RequestedBy,
	})
}

func (t *workflowEventTranslator) translateNodeFailure(evt agent.AgentEvent, e *domain.WorkflowNodeFailedEvent) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"iteration":   e.Iteration,
		"phase":       e.Phase,
		"recoverable": e.Recoverable,
	}

	if e.Error != nil {
		payload["error"] = e.Error.Error()
	}

	return t.diagnosticEnvelope(evt, types.EventNodeFailed, payload)
}

func (t *workflowEventTranslator) translateInputEnvelope(evt agent.AgentEvent, e *domain.WorkflowInputReceivedEvent) []*domain.WorkflowEventEnvelope {
	return t.singleEnvelope(evt, types.EventInputReceived, "input", "", map[string]any{
		"task":        e.Task,
		"attachments": e.Attachments,
	})
}

func (t *workflowEventTranslator) translateLifecycle(e *domain.WorkflowLifecycleUpdatedEvent, evt agent.AgentEvent) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"workflow_event_type": string(e.WorkflowEventType),
	}
	if e.Phase != "" {
		payload["phase"] = e.Phase
	}
	nodeID := nodeID(evt, e.Node)
	if e.Node != nil {
		payload["node"] = *e.Node
	}

	return t.workflowEnvelopeFromOptions(evt, envelopeOptions{
		snapshot:       e.Workflow,
		eventType:      types.EventLifecycleUpdated,
		nodeKind:       "node",
		nodeID:         nodeID,
		payload:        payload,
		attachWorkflow: true,
		skipRecorder:   true,
	})
}

func (t *workflowEventTranslator) translateNodeStarted(e *domain.WorkflowNodeStartedEvent, evt agent.AgentEvent) []*domain.WorkflowEventEnvelope {
	if isToolRecorderNodeID(e.StepDescription) {
		return nil
	}

	return t.nodeEnvelope(evt, types.EventNodeStarted, nodeEventMeta{
		stepDescription: e.StepDescription,
		stepIndex:       e.StepIndex,
		iteration:       e.Iteration,
		totalIters:      e.TotalIters,
		workflow:        e.Workflow,
		hasInput:        e.Input != nil,
	}, nil)
}

func (t *workflowEventTranslator) translateNodeCompleted(e *domain.WorkflowNodeCompletedEvent, evt agent.AgentEvent) []*domain.WorkflowEventEnvelope {
	if isToolRecorderNodeID(e.StepDescription) {
		return nil
	}

	status := strings.ToLower(strings.TrimSpace(e.Status))
	eventType := types.EventNodeCompleted
	if status == string(workflow.NodeStatusFailed) || status == "failed" {
		eventType = types.EventNodeFailed
	}

	return t.nodeEnvelope(evt, eventType, nodeEventMeta{
		stepDescription: e.StepDescription,
		stepIndex:       e.StepIndex,
		iteration:       e.Iteration,
		workflow:        e.Workflow,
	}, func(payload map[string]any) {
		if e.StepResult != nil {
			payload["result"] = e.StepResult
		}
		if e.TokensUsed > 0 {
			payload["tokens_used"] = e.TokensUsed
		}
		if e.ToolsRun > 0 {
			payload["tools_run"] = e.ToolsRun
		}
		if e.Duration > 0 {
			payload["duration_ms"] = e.Duration.Milliseconds()
		}
		if status != "" {
			payload["status"] = e.Status
		}
	})
}

func (t *workflowEventTranslator) translateNodeOutputSummary(e *domain.WorkflowNodeOutputSummaryEvent, evt agent.AgentEvent) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"iteration":       e.Iteration,
		"content":         e.Content,
		"tool_call_count": e.ToolCallCount,
	}
	for key, val := range e.Metadata {
		if _, exists := payload[key]; exists {
			continue
		}
		payload[key] = val
	}
	return t.singleEnvelope(evt, types.EventNodeOutputSummary, "generation", "", payload)
}

func (t *workflowEventTranslator) translateNodeOutputDelta(e *domain.WorkflowNodeOutputDeltaEvent, evt agent.AgentEvent) []*domain.WorkflowEventEnvelope {
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

	return t.singleEnvelope(evt, types.EventNodeOutputDelta, "generation", "", payload)
}

func (t *workflowEventTranslator) translateSubtaskEvent(event agent.SubtaskWrapper) []*domain.WorkflowEventEnvelope {
	if event == nil || event.WrappedEvent() == nil {
		return nil
	}

	eventType := types.EventSubflowProgress
	switch event.WrappedEvent().(type) {
	case *domain.WorkflowResultFinalEvent, *domain.WorkflowResultCancelledEvent:
		eventType = types.EventSubflowCompleted
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

	switch e := evt.(type) {
	case *domain.WorkflowLifecycleUpdatedEvent:
		if snapshot == nil {
			snapshot = e.Workflow
		}
		workflowID = e.WorkflowID
	case *domain.WorkflowNodeStartedEvent:
		if snapshot == nil {
			snapshot = e.Workflow
		}
	case *domain.WorkflowNodeCompletedEvent:
		if snapshot == nil {
			snapshot = e.Workflow
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
	if e, ok := evt.(*domain.WorkflowLifecycleUpdatedEvent); ok && e.Node != nil {
		return e.Node.ID
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

	filteredNodes := make([]workflow.NodeSnapshot, 0, len(snapshot.Nodes))
	filteredOrder := make([]string, 0, len(snapshot.Order))
	for _, n := range snapshot.Nodes {
		if isToolRecorderNodeID(n.ID) {
			continue
		}
		filteredNodes = append(filteredNodes, n)
	}
	orderSet := make(map[string]bool, len(filteredNodes))
	for _, n := range filteredNodes {
		orderSet[n.ID] = true
	}
	for _, id := range snapshot.Order {
		if orderSet[id] {
			filteredOrder = append(filteredOrder, id)
		}
	}

	summary := map[string]int64{
		"pending":   0,
		"running":   0,
		"succeeded": 0,
		"failed":    0,
	}
	for _, n := range filteredNodes {
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
