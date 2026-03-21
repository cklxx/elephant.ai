package coordinator

import (
	"sync"
	"testing"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/domain/workflow"
)

type recordingListener struct {
	mu     sync.Mutex
	events []agent.AgentEvent
}

func (r *recordingListener) OnEvent(event agent.AgentEvent) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *recordingListener) snapshot() []agent.AgentEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]agent.AgentEvent, len(r.events))
	copy(cp, r.events)
	return cp
}

func TestWorkflowEventBridgeEmitsLifecycleEvents(t *testing.T) {
	listener := &recordingListener{}
	bridge := newWorkflowEventBridge("wf-1", listener, nil, &agent.OutputContext{
		Level:        agent.LevelCore,
		SessionID:    "sess",
		TaskID:       "task",
		ParentTaskID: "parent",
		LogID:        "log-1",
	})

	pendingNode := workflow.NodeSnapshot{ID: "step-1", Status: workflow.NodeStatusPending}
	pendingSnapshot := workflow.WorkflowSnapshot{ID: "wf-1", Phase: workflow.PhasePending, Order: []string{"step-1"}}
	ts := time.Unix(1710000000, 0)

	bridge.OnWorkflowEvent(workflow.Event{
		Type:      workflow.EventNodeAdded,
		Workflow:  "wf-1",
		Phase:     workflow.PhasePending,
		Node:      &pendingNode,
		Snapshot:  &pendingSnapshot,
		Timestamp: ts,
	})

	events := listener.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected 1 event after node added, got %d", len(events))
	}

	lifecycle, ok := events[0].(*domain.Event)
	if !ok {
		t.Fatalf("expected *domain.Event, got %T", events[0])
	}
	if lifecycle.Kind != types.EventLifecycleUpdated {
		t.Fatalf("unexpected event kind: %s", lifecycle.Kind)
	}
	if lifecycle.Data.WorkflowID != "wf-1" || lifecycle.Data.WorkflowEventType != workflow.EventNodeAdded {
		t.Fatalf("unexpected workflow metadata: id=%s type=%s", lifecycle.Data.WorkflowID, lifecycle.Data.WorkflowEventType)
	}
	if lifecycle.Timestamp() != ts {
		t.Fatalf("expected timestamp %v, got %v", ts, lifecycle.Timestamp())
	}
	if lifecycle.Data.Node == nil || lifecycle.Data.Node.ID != "step-1" {
		t.Fatalf("expected node snapshot to be forwarded, got %#v", lifecycle.Data.Node)
	}
	if lifecycle.Data.Workflow == nil || lifecycle.Data.Workflow.Phase != workflow.PhasePending {
		t.Fatalf("expected workflow snapshot to be forwarded, got %#v", lifecycle.Data.Workflow)
	}

	runningNode := workflow.NodeSnapshot{ID: "step-1", Status: workflow.NodeStatusRunning, Input: map[string]any{"iteration": 2}}
	runningSnapshot := workflow.WorkflowSnapshot{ID: "wf-1", Phase: workflow.PhaseRunning, Order: []string{"step-1"}}

	bridge.OnWorkflowEvent(workflow.Event{
		Type:      workflow.EventNodeStarted,
		Workflow:  "wf-1",
		Phase:     workflow.PhaseRunning,
		Node:      &runningNode,
		Snapshot:  &runningSnapshot,
		Timestamp: ts.Add(time.Second),
	})

	events = listener.snapshot()
	if len(events) != 3 {
		t.Fatalf("expected lifecycle + step start events, got %d", len(events))
	}

	startLifecycle, ok := events[1].(*domain.Event)
	if !ok {
		t.Fatalf("expected *domain.Event for node start lifecycle, got %T", events[1])
	}
	if startLifecycle.Kind != types.EventLifecycleUpdated {
		t.Fatalf("expected lifecycle event kind, got %s", startLifecycle.Kind)
	}
	if startLifecycle.Data.Phase != workflow.PhaseRunning {
		t.Fatalf("expected running phase on lifecycle event, got %s", startLifecycle.Data.Phase)
	}

	stepStarted, ok := events[2].(*domain.Event)
	if !ok {
		t.Fatalf("expected *domain.Event for step started, got %T", events[2])
	}
	if stepStarted.Kind != types.EventNodeStarted {
		t.Fatalf("expected node started event kind, got %s", stepStarted.Kind)
	}
	if stepStarted.Data.StepIndex != 0 || stepStarted.Data.Iteration != 2 {
		t.Fatalf("unexpected step metadata: idx=%d iter=%d", stepStarted.Data.StepIndex, stepStarted.Data.Iteration)
	}
	if stepStarted.Data.Input == nil {
		t.Fatalf("expected step input to be forwarded")
	}
}

func TestWorkflowEventBridgeUsesLatestContext(t *testing.T) {
	listener := &recordingListener{}
	bridge := newWorkflowEventBridge("wf-context", listener, nil, &agent.OutputContext{
		Level:        agent.LevelCore,
		SessionID:    "sess-1",
		TaskID:       "task-1",
		ParentTaskID: "parent-1",
		LogID:        "log-1",
	})

	bridge.updateContext(&agent.OutputContext{
		Level:        agent.LevelSubagent,
		SessionID:    "sess-2",
		TaskID:       "task-2",
		ParentTaskID: "parent-2",
		LogID:        "log-2",
	})

	node := workflow.NodeSnapshot{ID: "step-ctx", Status: workflow.NodeStatusPending}
	snapshot := workflow.WorkflowSnapshot{ID: "wf-context", Phase: workflow.PhasePending, Order: []string{"step-ctx"}}

	bridge.OnWorkflowEvent(workflow.Event{Type: workflow.EventNodeAdded, Workflow: "wf-context", Phase: workflow.PhasePending, Node: &node, Snapshot: &snapshot})

	events := listener.snapshot()
	if len(events) != 1 {
		t.Fatalf("expected lifecycle event only, got %d", len(events))
	}

	lifecycleCtx, ok := events[0].(*domain.Event)
	if !ok {
		t.Fatalf("expected *domain.Event, got %T", events[0])
	}
	if lifecycleCtx.Kind != types.EventLifecycleUpdated {
		t.Fatalf("expected lifecycle event kind, got %s", lifecycleCtx.Kind)
	}
	if lifecycleCtx.GetSessionID() != "sess-2" || lifecycleCtx.GetRunID() != "task-2" || lifecycleCtx.GetParentRunID() != "parent-2" {
		t.Fatalf("unexpected context fields: session=%s run=%s parent=%s", lifecycleCtx.GetSessionID(), lifecycleCtx.GetRunID(), lifecycleCtx.GetParentRunID())
	}
	if lifecycleCtx.GetAgentLevel() != agent.LevelSubagent {
		t.Fatalf("expected updated agent level, got %s", lifecycleCtx.GetAgentLevel())
	}
	if lifecycleCtx.Timestamp().IsZero() {
		t.Fatalf("expected bridge to provide timestamp when missing on event")
	}
}
