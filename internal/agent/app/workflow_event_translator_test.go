package app

import (
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/workflow"
)

type recordingAgentListener struct {
	events []ports.AgentEvent
}

func (r *recordingAgentListener) OnEvent(event ports.AgentEvent) {
	r.events = append(r.events, event)
}

func (r *recordingAgentListener) snapshot() []ports.AgentEvent {
	out := make([]ports.AgentEvent, len(r.events))
	copy(out, r.events)
	return out
}

func TestWorkflowEventTranslatorSkipsToolRecorderNodes(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	ts := time.Unix(1710000000, 0)
	translator.OnEvent(&domain.WorkflowNodeCompletedEvent{
		BaseEvent:       domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", ts),
		StepDescription: "react:iter:1:tool:text_to_image:0",
		StepIndex:       4,
		Status:          string(workflow.NodeStatusSucceeded),
		Iteration:       1,
	})

	if got := len(sink.snapshot()); got != 0 {
		t.Fatalf("expected tool recorder node to be filtered, got %d events", got)
	}
}

func TestWorkflowEventTranslatorForwardsNonToolNodes(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	ts := time.Unix(1710000000, 0)
	translator.OnEvent(&domain.WorkflowNodeCompletedEvent{
		BaseEvent:       domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", ts),
		StepDescription: "react:iter:1:plan",
		StepIndex:       2,
		Status:          string(workflow.NodeStatusSucceeded),
		Iteration:       1,
	})

	events := sink.snapshot()
	if got := len(events); got != 1 {
		t.Fatalf("expected one event to be forwarded, got %d", got)
	}

	env, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[0])
	}
	if env.NodeID != "react:iter:1:plan" {
		t.Fatalf("unexpected node id %q", env.NodeID)
	}
	if env.Event != "workflow.node.completed" {
		t.Fatalf("unexpected event type %q", env.Event)
	}
}

func TestWorkflowEventTranslatorFiltersToolNodesFromLifecycleSnapshots(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	toolNode := workflow.NodeSnapshot{ID: "react:iter:1:tool:text_to_image:0", Status: workflow.NodeStatusSucceeded}
	planNode := workflow.NodeSnapshot{ID: "react:iter:1:plan", Status: workflow.NodeStatusSucceeded}
	snapshot := workflow.WorkflowSnapshot{
		ID:      "wf-1",
		Phase:   workflow.PhaseRunning,
		Order:   []string{planNode.ID, toolNode.ID},
		Nodes:   []workflow.NodeSnapshot{planNode, toolNode},
		Summary: map[string]int64{string(workflow.NodeStatusSucceeded): 2},
	}

	translator.OnEvent(&domain.WorkflowLifecycleUpdatedEvent{
		BaseEvent:         domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", time.Unix(1710000001, 0)),
		WorkflowID:        "wf-1",
		WorkflowEventType: workflow.EventWorkflowUpdated,
		Workflow:          &snapshot,
	})

	events := sink.snapshot()
	if got := len(events); got != 1 {
		t.Fatalf("expected lifecycle event, got %d", got)
	}
	env, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[0])
	}
	ws, ok := env.Payload["workflow"].(*workflow.WorkflowSnapshot)
	if !ok {
		t.Fatalf("expected workflow snapshot in payload, got %T", env.Payload["workflow"])
	}
	if len(ws.Nodes) != 1 || ws.Nodes[0].ID != planNode.ID {
		t.Fatalf("expected tool nodes to be filtered, got %+v", ws.Nodes)
	}
	if len(ws.Order) != 1 || ws.Order[0] != planNode.ID {
		t.Fatalf("expected order to exclude tool nodes, got %+v", ws.Order)
	}
	if ws.Summary[string(workflow.NodeStatusSucceeded)] != 1 {
		t.Fatalf("expected summary counts to be recomputed, got %+v", ws.Summary)
	}
}

func TestWorkflowEventTranslatorSkipsLifecycleEventsForToolRecorderNode(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	toolNode := workflow.NodeSnapshot{ID: "react:iter:1:tool:text_to_image:0", Status: workflow.NodeStatusSucceeded}

	translator.OnEvent(&domain.WorkflowLifecycleUpdatedEvent{
		BaseEvent:         domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", time.Unix(1710000002, 0)),
		WorkflowID:        "wf-1",
		WorkflowEventType: workflow.EventNodeSucceeded,
		Node:              &toolNode,
		Workflow: &workflow.WorkflowSnapshot{
			ID:    "wf-1",
			Phase: workflow.PhaseRunning,
		},
	})

	if got := len(sink.snapshot()); got != 0 {
		t.Fatalf("expected tool lifecycle event to be filtered, got %d", got)
	}
}
