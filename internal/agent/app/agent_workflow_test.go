package app

import (
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/workflow"
)

type recordingListener struct {
	events []ports.AgentEvent
}

func (r *recordingListener) OnEvent(event ports.AgentEvent) {
	r.events = append(r.events, event)
}

func (r *recordingListener) snapshot() []ports.AgentEvent {
	cp := make([]ports.AgentEvent, len(r.events))
	copy(cp, r.events)
	return cp
}

func TestWorkflowEventBridgeEmitsLifecycleEvents(t *testing.T) {
	listener := &recordingListener{}
	bridge := newWorkflowEventBridge("wf-1", listener, nil, ports.LevelCore, "sess", "task", "parent")

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

	lifecycle, ok := events[0].(*domain.WorkflowLifecycleEvent)
	if !ok {
		t.Fatalf("expected workflow lifecycle event, got %T", events[0])
	}
	if lifecycle.EventType() != "workflow_event" {
		t.Fatalf("unexpected event type: %s", lifecycle.EventType())
	}
	if lifecycle.WorkflowID != "wf-1" || lifecycle.WorkflowEventType != workflow.EventNodeAdded {
		t.Fatalf("unexpected workflow metadata: id=%s type=%s", lifecycle.WorkflowID, lifecycle.WorkflowEventType)
	}
	if lifecycle.Timestamp() != ts {
		t.Fatalf("expected timestamp %v, got %v", ts, lifecycle.Timestamp())
	}
	if lifecycle.Node == nil || lifecycle.Node.ID != "step-1" {
		t.Fatalf("expected node snapshot to be forwarded, got %#v", lifecycle.Node)
	}
	if lifecycle.Workflow == nil || lifecycle.Workflow.Phase != workflow.PhasePending {
		t.Fatalf("expected workflow snapshot to be forwarded, got %#v", lifecycle.Workflow)
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

	startLifecycle, ok := events[1].(*domain.WorkflowLifecycleEvent)
	if !ok {
		t.Fatalf("expected lifecycle event for node start, got %T", events[1])
	}
	if startLifecycle.Phase != workflow.PhaseRunning {
		t.Fatalf("expected running phase on lifecycle event, got %s", startLifecycle.Phase)
	}

	stepStarted, ok := events[2].(*domain.StepStartedEvent)
	if !ok {
		t.Fatalf("expected step started event, got %T", events[2])
	}
	if stepStarted.StepIndex != 0 || stepStarted.Iteration != 2 {
		t.Fatalf("unexpected step metadata: idx=%d iter=%d", stepStarted.StepIndex, stepStarted.Iteration)
	}
	if stepStarted.Input == nil {
		t.Fatalf("expected step input to be forwarded")
	}
}
