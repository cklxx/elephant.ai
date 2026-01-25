package coordinator

import (
	"log/slog"
	"sync"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/workflow"
)

const (
	stagePrepare   = "prepare"
	stageExecute   = "execute"
	stageSummarize = "summarize"
	stagePersist   = "persist"
)

// agentWorkflow wraps the generic workflow primitives with the fixed stages used by
// agent executions. It centralizes transitions so the coordinator can emit
// consistent snapshots for debugging without duplicating state management logic
// across multiple call sites.
type agentWorkflow struct {
	wf        *workflow.Workflow
	logger    *slog.Logger
	stages    map[string]*workflow.Node
	mu        sync.Mutex
	eventSink *workflowEventBridge
}

func newAgentWorkflow(id string, logger *slog.Logger, listener agent.EventListener, outCtx *agent.OutputContext) *agentWorkflow {
	if logger == nil {
		logger = slog.Default()
	}

	wf := workflow.New(id, logger)
	aw := &agentWorkflow{wf: wf, logger: logger, stages: make(map[string]*workflow.Node)}

	aw.register(stagePrepare, map[string]string{"stage": "prepare"})
	aw.register(stageExecute, map[string]string{"stage": "execute"})
	aw.register(stageSummarize, map[string]string{"stage": "summarize"})
	aw.register(stagePersist, map[string]string{"stage": "persist"})

	if listener != nil {
		if outCtx == nil {
			outCtx = &agent.OutputContext{Level: agent.LevelCore}
		}
		aw.eventSink = newWorkflowEventBridge(id, listener, logger, outCtx.Level, outCtx.SessionID, outCtx.TaskID, outCtx.ParentTaskID)
		wf.AddListener(aw.eventSink)
	}

	return aw
}

func (aw *agentWorkflow) register(id string, input any) {
	aw.ensureNode(id, input)
}

func (aw *agentWorkflow) start(stage string) {
	aw.transition(stage, "start", nil, func(nodeID string) error {
		_, _, err := aw.wf.StartNode(nodeID)
		return err
	})
}

func (aw *agentWorkflow) succeed(stage string, output any) {
	aw.transition(stage, "success", nil, func(nodeID string) error {
		_, _, err := aw.wf.CompleteNodeSuccess(nodeID, output)
		return err
	})
}

func (aw *agentWorkflow) fail(stage string, err error) {
	aw.transition(stage, "failure", nil, func(nodeID string) error {
		_, _, transErr := aw.wf.CompleteNodeFailure(nodeID, err)
		return transErr
	})
}

func (aw *agentWorkflow) snapshot() workflow.WorkflowSnapshot {
	return aw.wf.Snapshot()
}

func (aw *agentWorkflow) transition(stage, action string, input any, fn func(string) error) {
	node := aw.ensureNode(stage, input)
	if node == nil {
		return
	}

	if err := fn(node.ID()); err != nil && aw.logger != nil {
		aw.logger.Warn("agent workflow "+action+" failed", slog.String("stage", stage), slog.String("error", err.Error()))
	}
}

func (aw *agentWorkflow) ensureNode(id string, input any) *workflow.Node {
	if id == "" {
		return nil
	}

	aw.mu.Lock()
	if node, exists := aw.stages[id]; exists {
		aw.mu.Unlock()
		return node
	}

	node := workflow.NewNode(id, input, aw.logger)
	aw.stages[id] = node
	aw.mu.Unlock()

	if err := aw.wf.AddNode(node); err != nil && aw.logger != nil {
		aw.logger.Warn("agent workflow add node failed", slog.String("stage", id), slog.String("error", err.Error()))
	}

	return node
}

func (aw *agentWorkflow) setContext(sessionID, taskID, parentTaskID string, level agent.AgentLevel) {
	if aw.eventSink == nil {
		return
	}
	aw.eventSink.updateContext(sessionID, taskID, parentTaskID, level)
}

// EnsureNode satisfies the domain.WorkflowTracker interface.
func (aw *agentWorkflow) EnsureNode(id string, input any) {
	aw.ensureNode(id, input)
}

// StartNode satisfies the domain.WorkflowTracker interface.
func (aw *agentWorkflow) StartNode(id string) {
	aw.start(id)
}

// CompleteNodeSuccess satisfies the domain.WorkflowTracker interface.
func (aw *agentWorkflow) CompleteNodeSuccess(id string, output any) {
	aw.succeed(id, output)
}

// CompleteNodeFailure satisfies the domain.WorkflowTracker interface.
func (aw *agentWorkflow) CompleteNodeFailure(id string, err error) {
	aw.fail(id, err)
}

type workflowEventBridge struct {
	listener   agent.EventListener
	logger     *slog.Logger
	workflowID string
	mu         sync.RWMutex
	context    workflowEventContext
}

func newWorkflowEventBridge(workflowID string, listener agent.EventListener, logger *slog.Logger, level agent.AgentLevel, sessionID, taskID, parentTaskID string) *workflowEventBridge {
	return &workflowEventBridge{
		listener:   listener,
		logger:     logger,
		workflowID: workflowID,
		context: workflowEventContext{
			level:        level,
			sessionID:    sessionID,
			taskID:       taskID,
			parentTaskID: parentTaskID,
		},
	}
}

func (b *workflowEventBridge) updateContext(sessionID, taskID, parentTaskID string, level agent.AgentLevel) {
	b.mu.Lock()
	b.context.update(sessionID, taskID, parentTaskID, level)
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
	sessionID    string
	taskID       string
	parentTaskID string
	level        agent.AgentLevel
}

func (c *workflowEventContext) update(sessionID, taskID, parentTaskID string, level agent.AgentLevel) {
	if sessionID != "" {
		c.sessionID = sessionID
	}
	if taskID != "" {
		c.taskID = taskID
	}
	if parentTaskID != "" {
		c.parentTaskID = parentTaskID
	}
	if level != "" {
		c.level = level
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
	}

	return step
}

func (c workflowEventContext) baseEvent(ts time.Time) domain.BaseEvent {
	if ts.IsZero() {
		ts = time.Now()
	}
	return domain.NewBaseEvent(c.level, c.sessionID, c.taskID, c.parentTaskID, ts)
}

func (b *workflowEventBridge) emitLifecycle(base domain.BaseEvent, evt workflow.Event) {
	b.listener.OnEvent(&domain.WorkflowLifecycleUpdatedEvent{
		BaseEvent:         base,
		WorkflowID:        evt.Workflow,
		WorkflowEventType: evt.Type,
		Phase:             evt.Phase,
		Node:              evt.Node,
		Workflow:          evt.Snapshot,
	})
}

func (b *workflowEventBridge) emitStep(base domain.BaseEvent, evt workflow.Event, step *stepPayload) {
	if step == nil {
		return
	}

	switch evt.Type {
	case workflow.EventNodeStarted:
		b.listener.OnEvent(&domain.WorkflowNodeStartedEvent{
			BaseEvent:       base,
			StepIndex:       step.index,
			StepDescription: evt.Node.ID,
			Iteration:       step.iteration,
			Input:           evt.Node.Input,
			Workflow:        evt.Snapshot,
		})
	case workflow.EventNodeSucceeded, workflow.EventNodeFailed:
		b.listener.OnEvent(&domain.WorkflowNodeCompletedEvent{
			BaseEvent:       base,
			StepIndex:       step.index,
			StepDescription: evt.Node.ID,
			StepResult:      step.result,
			Status:          step.status,
			Iteration:       step.iteration,
			Workflow:        evt.Snapshot,
		})
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
