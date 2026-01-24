package app

import (
	"fmt"
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

func TestWorkflowEventTranslatorEmitsToolCallNodes(t *testing.T) {
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

	events := sink.snapshot()
	if got := len(events); got != 1 {
		t.Fatalf("expected tool recorder node to be forwarded, got %d events", got)
	}
	env, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[0])
	}
	if env.Event != "workflow.node.completed" {
		t.Fatalf("unexpected event type %q", env.Event)
	}
	if env.NodeKind != "step" || env.NodeID != "react:iter:1:tool:text_to_image:0" {
		t.Fatalf("unexpected node metadata: kind=%q id=%q", env.NodeKind, env.NodeID)
	}
}

func TestWorkflowEventTranslatorSkipsToolsAggregateNode(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	ts := time.Unix(1710000000, 0)
	translator.OnEvent(&domain.WorkflowNodeCompletedEvent{
		BaseEvent:       domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", ts),
		StepDescription: "react:iter:1:tools",
		StepIndex:       3,
		Status:          string(workflow.NodeStatusSucceeded),
		Iteration:       1,
	})

	if got := len(sink.snapshot()); got != 0 {
		t.Fatalf("expected tools aggregate node to be filtered, got %d events", got)
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
	if len(ws.Nodes) != 2 || ws.Nodes[0].ID != planNode.ID || ws.Nodes[1].ID != toolNode.ID {
		t.Fatalf("expected tool nodes to be preserved, got %+v", ws.Nodes)
	}
	if len(ws.Order) != 2 || ws.Order[0] != planNode.ID || ws.Order[1] != toolNode.ID {
		t.Fatalf("expected order to preserve tool nodes, got %+v", ws.Order)
	}
	if ws.Summary[string(workflow.NodeStatusSucceeded)] != 2 {
		t.Fatalf("expected summary counts to include tool nodes, got %+v", ws.Summary)
	}
}

func TestWorkflowEventTranslatorUsesWorkflowIDFromLifecycleEvent(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	now := time.Unix(1710000100, 0)
	translator.OnEvent(&domain.WorkflowLifecycleUpdatedEvent{
		BaseEvent:         domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", now),
		WorkflowID:        "wf-lifecycle",
		WorkflowEventType: workflow.EventWorkflowUpdated,
	})

	events := sink.snapshot()
	if got := len(events); got != 1 {
		t.Fatalf("expected lifecycle event, got %d", got)
	}

	env, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[0])
	}

	if env.WorkflowID != "wf-lifecycle" || env.RunID != "wf-lifecycle" {
		t.Fatalf("expected workflow identifiers to propagate, got workflow_id=%q run_id=%q", env.WorkflowID, env.RunID)
	}
}

func TestWorkflowEventTranslatorSkipsLifecycleToolsAggregate(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	now := time.Unix(1710000001, 0)
	aggregate := workflow.NodeSnapshot{ID: "react:iter:1:tools", Status: workflow.NodeStatusSucceeded}

	translator.OnEvent(&domain.WorkflowLifecycleUpdatedEvent{
		BaseEvent:         domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", now),
		WorkflowID:        "wf-agg",
		WorkflowEventType: workflow.EventNodeSucceeded,
		Node:              &aggregate,
		Workflow:          &workflow.WorkflowSnapshot{ID: "wf-agg"},
	})

	if got := len(sink.snapshot()); got != 0 {
		t.Fatalf("expected lifecycle events for aggregate recorder nodes to be skipped, got %d", got)
	}
}

func TestWorkflowEventTranslatorSanitizesWorkflowOnNodeEvents(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	snapshot := workflow.WorkflowSnapshot{
		ID:    "wf-2",
		Phase: workflow.PhaseRunning,
		Order: []string{"react:iter:1:tools", "react:iter:1:plan"},
		Nodes: []workflow.NodeSnapshot{
			{ID: "react:iter:1:tools", Status: workflow.NodeStatusSucceeded},
			{ID: "react:iter:1:plan", Status: workflow.NodeStatusPending},
		},
	}

	translator.OnEvent(&domain.WorkflowNodeStartedEvent{
		BaseEvent:       domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", time.Unix(1710000002, 0)),
		StepDescription: "react:iter:1:plan",
		StepIndex:       1,
		Iteration:       1,
		Workflow:        &snapshot,
	})

	events := sink.snapshot()
	if got := len(events); got != 1 {
		t.Fatalf("expected single node event, got %d", got)
	}

	env, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[0])
	}

	if env.WorkflowID != "wf-2" || env.RunID != "wf-2" {
		t.Fatalf("expected workflow identifiers to be propagated, got workflow_id=%q run_id=%q", env.WorkflowID, env.RunID)
	}

	ws, ok := env.Payload["workflow"].(*workflow.WorkflowSnapshot)
	if !ok {
		t.Fatalf("expected workflow snapshot in payload, got %T", env.Payload["workflow"])
	}

	if len(ws.Nodes) != 1 || ws.Nodes[0].ID != "react:iter:1:plan" {
		t.Fatalf("expected tool recorder nodes to be removed, got %+v", ws.Nodes)
	}
	if len(ws.Order) != 1 || ws.Order[0] != "react:iter:1:plan" {
		t.Fatalf("expected sanitized order to exclude recorder nodes, got %+v", ws.Order)
	}
}

func TestWorkflowEventTranslatorEmitsLifecycleEventsForToolCallNode(t *testing.T) {
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

	if got := len(sink.snapshot()); got != 1 {
		t.Fatalf("expected tool lifecycle event to be forwarded, got %d", got)
	}
}

func TestWorkflowEventTranslatorAddsCallIDToToolPayload(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	translator.OnEvent(&domain.WorkflowToolProgressEvent{
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", time.Unix(1710000003, 0)),
		CallID:    "call-123",
		Chunk:     "partial",
	})

	events := sink.snapshot()
	if got := len(events); got != 1 {
		t.Fatalf("expected a single tool progress envelope, got %d", got)
	}

	env, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[0])
	}

	if env.NodeKind != "tool" || env.NodeID != "call-123" {
		t.Fatalf("unexpected tool metadata: kind=%q id=%q", env.NodeKind, env.NodeID)
	}

	callID, ok := env.Payload["call_id"].(string)
	if !ok || callID != "call-123" {
		t.Fatalf("expected call_id in payload, got %#v", env.Payload["call_id"])
	}
}

func TestWorkflowEventTranslatorEmitsDiagnosticNodeFailure(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	translator.OnEvent(&domain.WorkflowNodeFailedEvent{
		BaseEvent:   domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", time.Unix(1710000004, 0)),
		Iteration:   2,
		Phase:       "execute",
		Recoverable: true,
		Error:       fmt.Errorf("boom"),
	})

	events := sink.snapshot()
	if got := len(events); got != 1 {
		t.Fatalf("expected diagnostic failure envelope, got %d", got)
	}

	env, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[0])
	}

	if env.Event != "workflow.node.failed" || env.NodeKind != "diagnostic" {
		t.Fatalf("unexpected failure metadata: event=%q kind=%q", env.Event, env.NodeKind)
	}

	if env.Payload["iteration"] != 2 || env.Payload["phase"] != "execute" || env.Payload["recoverable"] != true {
		t.Fatalf("unexpected failure payload: %#v", env.Payload)
	}
	if env.Payload["error"] != "boom" {
		t.Fatalf("expected error string in payload, got %#v", env.Payload["error"])
	}
}

func TestWorkflowEventTranslatorReusesWorkflowContextForTools(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	snapshot := workflow.WorkflowSnapshot{ID: "wf-context", Phase: workflow.PhaseRunning}
	base := domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", time.Unix(1710000005, 0))

	translator.OnEvent(&domain.WorkflowLifecycleUpdatedEvent{
		BaseEvent:         base,
		WorkflowID:        "wf-context",
		WorkflowEventType: workflow.EventWorkflowUpdated,
		Workflow:          &snapshot,
	})

	translator.OnEvent(&domain.WorkflowToolProgressEvent{
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", time.Unix(1710000006, 0)),
		CallID:    "call-ctx",
		Chunk:     "partial",
	})

	events := sink.snapshot()
	if got := len(events); got != 2 {
		t.Fatalf("expected lifecycle and tool envelopes, got %d", got)
	}

	env, ok := events[1].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[1])
	}

	if env.WorkflowID != "wf-context" || env.RunID != "wf-context" {
		t.Fatalf("expected workflow identifiers to persist, got workflow_id=%q run_id=%q", env.WorkflowID, env.RunID)
	}
	if env.NodeKind != "tool" || env.NodeID != "call-ctx" {
		t.Fatalf("unexpected tool envelope metadata: kind=%q id=%q", env.NodeKind, env.NodeID)
	}
}

func TestWorkflowEventTranslatorForwardsContextSnapshotEvents(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	ts := time.Unix(1710000200, 0)
	original := domain.NewWorkflowDiagnosticContextSnapshotEvent(
		ports.LevelCore,
		"sess",
		"task",
		"parent",
		1,
		1,
		"req-1",
		[]ports.Message{{Role: "user", Content: "ping"}},
		nil,
		ts,
	)

	translator.OnEvent(original)

	events := sink.snapshot()
	if got := len(events); got != 1 {
		t.Fatalf("expected one event, got %d", got)
	}
	if _, ok := events[0].(*domain.WorkflowDiagnosticContextSnapshotEvent); !ok {
		t.Fatalf("expected context snapshot event, got %T", events[0])
	}
	if events[0].EventType() != original.EventType() {
		t.Fatalf("unexpected event type %q", events[0].EventType())
	}
}

func TestWorkflowEventTranslatorEmitsArtifactManifestEvent(t *testing.T) {
	sink := &recordingAgentListener{}
	translator := wrapWithWorkflowEnvelope(sink, nil)

	ts := time.Unix(1710000300, 0)
	translator.OnEvent(&domain.WorkflowToolCompletedEvent{
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", ts),
		CallID:    "call-1",
		ToolName:  "artifact_manifest",
		Result:    "Recorded 1 artifact(s).",
		Metadata:  map[string]any{"artifact_manifest": map[string]any{"items": []any{"artifact"}}},
		Attachments: map[string]ports.Attachment{
			"manifest.json": {Name: "manifest.json", MediaType: "application/json", Format: "manifest"},
		},
	})

	events := sink.snapshot()
	if got := len(events); got != 2 {
		t.Fatalf("expected tool completed + artifact manifest events, got %d", got)
	}

	env, ok := events[1].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[1])
	}
	if env.Event != "workflow.artifact.manifest" {
		t.Fatalf("expected artifact manifest event, got %q", env.Event)
	}
	if _, ok := env.Payload["manifest"]; !ok {
		t.Fatalf("expected manifest payload, got %#v", env.Payload)
	}
	if env.NodeKind != "artifact" {
		t.Fatalf("expected artifact node kind, got %q", env.NodeKind)
	}
}
