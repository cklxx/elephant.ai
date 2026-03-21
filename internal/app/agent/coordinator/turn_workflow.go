package coordinator

import (
	"log/slog"
	"sync"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/workflow"
)

// turnWorkflow wraps workflow.Workflow as a pure WorkflowTracker for
// the ReactEngine. No pre-registered stages — only ReactEngine creates nodes.
type turnWorkflow struct {
	wf        *workflow.Workflow
	logger    *slog.Logger
	nodes     map[string]*workflow.Node
	mu        sync.Mutex
	eventSink *workflowEventBridge
}

func newTurnWorkflow(id string, logger *slog.Logger, listener agent.EventListener, outCtx *agent.OutputContext) *turnWorkflow {
	if logger == nil {
		logger = slog.Default()
	}

	wf := workflow.New(id, logger)
	tw := &turnWorkflow{wf: wf, logger: logger, nodes: make(map[string]*workflow.Node)}

	if listener != nil {
		if outCtx == nil {
			outCtx = &agent.OutputContext{Level: agent.LevelCore}
		}
		tw.eventSink = newWorkflowEventBridge(id, listener, logger, outCtx)
		wf.AddListener(tw.eventSink)
	}

	return tw
}

func (tw *turnWorkflow) start(id string) {
	tw.transition(id, "start", nil, func(nodeID string) error {
		_, _, err := tw.wf.StartNode(nodeID)
		return err
	})
}

func (tw *turnWorkflow) succeed(id string, output any) {
	tw.transition(id, "success", nil, func(nodeID string) error {
		_, _, err := tw.wf.CompleteNodeSuccess(nodeID, output)
		return err
	})
}

func (tw *turnWorkflow) fail(id string, err error) {
	tw.transition(id, "failure", nil, func(nodeID string) error {
		_, _, transErr := tw.wf.CompleteNodeFailure(nodeID, err)
		return transErr
	})
}

func (tw *turnWorkflow) snapshot() workflow.WorkflowSnapshot {
	return tw.wf.Snapshot()
}

func (tw *turnWorkflow) transition(id, action string, input any, fn func(string) error) {
	node := tw.ensureNode(id, input)
	if node == nil {
		return
	}

	if err := fn(node.ID()); err != nil && tw.logger != nil {
		tw.logger.Warn("turn workflow "+action+" failed", slog.String("node", id), slog.String("error", err.Error()))
	}
}

func (tw *turnWorkflow) ensureNode(id string, input any) *workflow.Node {
	if id == "" {
		return nil
	}

	tw.mu.Lock()
	if node, exists := tw.nodes[id]; exists {
		tw.mu.Unlock()
		return node
	}

	node := workflow.NewNode(id, input, tw.logger)
	tw.nodes[id] = node
	tw.mu.Unlock()

	if err := tw.wf.AddNode(node); err != nil && tw.logger != nil {
		tw.logger.Warn("turn workflow add node failed", slog.String("node", id), slog.String("error", err.Error()))
	}

	return node
}

func (tw *turnWorkflow) setContext(outCtx *agent.OutputContext) {
	if tw.eventSink == nil {
		return
	}
	tw.eventSink.updateContext(outCtx)
}

// EnsureNode satisfies the react.WorkflowTracker interface.
func (tw *turnWorkflow) EnsureNode(id string, input any) {
	tw.ensureNode(id, input)
}

// StartNode satisfies the react.WorkflowTracker interface.
func (tw *turnWorkflow) StartNode(id string) {
	tw.start(id)
}

// CompleteNodeSuccess satisfies the react.WorkflowTracker interface.
func (tw *turnWorkflow) CompleteNodeSuccess(id string, output any) {
	tw.succeed(id, output)
}

// CompleteNodeFailure satisfies the react.WorkflowTracker interface.
func (tw *turnWorkflow) CompleteNodeFailure(id string, err error) {
	tw.fail(id, err)
}

type workflowEventBridge struct {
	listener   agent.EventListener
	logger     *slog.Logger
	workflowID string
	mu         sync.RWMutex
	context    workflowEventContext
}

func newWorkflowEventBridge(workflowID string, listener agent.EventListener, logger *slog.Logger, outCtx *agent.OutputContext) *workflowEventBridge {
	ctx := workflowEventContext{level: agent.LevelCore}
	if outCtx != nil {
		ctx.level = outCtx.Level
		ctx.sessionID = outCtx.SessionID
		ctx.runID = outCtx.TaskID
		ctx.parentRunID = outCtx.ParentTaskID
		ctx.logID = outCtx.LogID
	}
	return &workflowEventBridge{
		listener:   listener,
		logger:     logger,
		workflowID: workflowID,
		context:    ctx,
	}
}

func (b *workflowEventBridge) updateContext(outCtx *agent.OutputContext) {
	if outCtx == nil {
		return
	}
	b.mu.Lock()
	b.context.updateFromOutputContext(outCtx)
	b.mu.Unlock()
}

func (b *workflowEventBridge) OnWorkflowEvent(evt workflow.Event) {
	if b.listener == nil {
		return
	}

	base := b.snapshotContext().baseEvent(evt.Timestamp)
	b.emitLifecycle(base, evt)
	b.emitStep(base, evt, b.buildStepPayload(evt))
}

func (b *workflowEventBridge) resolveIndex(evt workflow.Event) (int, bool) {
	if evt.Node == nil {
		return -1, false
	}

	if evt.Snapshot != nil {
		for i, id := range evt.Snapshot.Order {
			if id == evt.Node.ID {
				return i, true
			}
		}
		for i := range evt.Snapshot.Nodes {
			if evt.Snapshot.Nodes[i].ID == evt.Node.ID {
				return i, true
			}
		}
	}
	return -1, false
}

type workflowEventContext struct {
	sessionID   string
	runID       string
	parentRunID string
	level       agent.AgentLevel
	logID       string
}

func (c *workflowEventContext) updateFromOutputContext(outCtx *agent.OutputContext) {
	if outCtx.SessionID != "" {
		c.sessionID = outCtx.SessionID
	}
	if outCtx.TaskID != "" {
		c.runID = outCtx.TaskID
	}
	if outCtx.ParentTaskID != "" {
		c.parentRunID = outCtx.ParentTaskID
	}
	if outCtx.Level != "" {
		c.level = outCtx.Level
	}
	if outCtx.LogID != "" {
		c.logID = outCtx.LogID
	}
}

func (b *workflowEventBridge) snapshotContext() workflowEventContext {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.context
}

type stepPayload struct {
	index     int
	iteration int
	result    any
	status    string
	duration  time.Duration
}

func (b *workflowEventBridge) buildStepPayload(evt workflow.Event) *stepPayload {
	if evt.Node == nil || evt.Snapshot == nil {
		return nil
	}

	switch evt.Type {
	case workflow.EventNodeStarted, workflow.EventNodeSucceeded, workflow.EventNodeFailed:
	default:
		return nil
	}

	idx, ok := b.resolveIndex(evt)
	if !ok {
		return nil
	}

	step := &stepPayload{
		index:     idx,
		iteration: extractIteration(evt.Node),
	}

	if evt.Type == workflow.EventNodeSucceeded || evt.Type == workflow.EventNodeFailed {
		step.result = normalizeStepResult(evt.Node)
		step.status = string(evt.Node.Status)
		if evt.Node.Duration > 0 {
			step.duration = evt.Node.Duration
		}
	}

	return step
}

func (c workflowEventContext) baseEvent(ts time.Time) domain.BaseEvent {
	if ts.IsZero() {
		ts = time.Now()
	}
	base := domain.NewBaseEvent(c.level, c.sessionID, c.runID, c.parentRunID, ts)
	if c.logID != "" {
		base.SetLogID(c.logID)
	}
	return base
}

func (b *workflowEventBridge) emitLifecycle(base domain.BaseEvent, evt workflow.Event) {
	b.listener.OnEvent(domain.NewLifecycleUpdatedEvent(base, evt.Workflow, evt.Type, evt.Phase, evt.Node, evt.Snapshot))
}

func (b *workflowEventBridge) emitStep(base domain.BaseEvent, evt workflow.Event, step *stepPayload) {
	if step == nil {
		return
	}

	switch evt.Type {
	case workflow.EventNodeStarted:
		b.listener.OnEvent(domain.NewNodeStartedEvent(base, step.iteration, 0, step.index, evt.Node.ID, evt.Node.Input, evt.Snapshot))
	case workflow.EventNodeSucceeded, workflow.EventNodeFailed:
		b.listener.OnEvent(domain.NewNodeCompletedEvent(base, step.index, evt.Node.ID, step.result, step.status, step.iteration, 0, 0, step.duration, evt.Snapshot))
	}
}

func extractIteration(node *workflow.NodeSnapshot) int {
	if node == nil {
		return 0
	}

	switch val := node.Input.(type) {
	case map[string]any:
		if iter, ok := val["iteration"]; ok {
			return coerceIteration(iter)
		}
	case map[string]int:
		if iter, ok := val["iteration"]; ok {
			return iter
		}
	}

	return coerceIteration(node.Output)
}

func coerceIteration(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float32:
		return int(v)
	case float64:
		return int(v)
	case map[string]any:
		if iter, ok := v["iteration"]; ok {
			return coerceIteration(iter)
		}
	case *map[string]any:
		if v != nil {
			return coerceIteration(*v)
		}
	}
	return 0
}

func normalizeStepResult(node *workflow.NodeSnapshot) any {
	if node == nil {
		return nil
	}
	if node.Error != "" {
		return map[string]any{"error": node.Error}
	}
	return node.Output
}
