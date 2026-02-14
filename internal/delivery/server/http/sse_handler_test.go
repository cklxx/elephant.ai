package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	serverapp "alex/internal/delivery/server/app"
	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/domain/workflow"
	"alex/internal/infra/attachments"
)

// sseResponseRecorder captures streamed SSE payloads without buffering or flushing semantics.
type sseResponseRecorder struct {
	header http.Header
	body   strings.Builder
	mu     sync.Mutex
}

func newSSERecorder() *sseResponseRecorder {
	return &sseResponseRecorder{header: make(http.Header)}
}

func (r *sseResponseRecorder) Header() http.Header { return r.header }
func (r *sseResponseRecorder) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.body.Write(p)
}
func (r *sseResponseRecorder) WriteHeader(statusCode int) {}
func (r *sseResponseRecorder) Flush()                     {}

func (r *sseResponseRecorder) BodyString() string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.body.String()
}

type streamedEvent struct {
	event string
	data  map[string]any
}

type stubSubtaskEvent struct {
	original agent.AgentEvent
	meta     agent.SubtaskMetadata
}

func (e *stubSubtaskEvent) EventType() string { return e.original.EventType() }
func (e *stubSubtaskEvent) Timestamp() time.Time {
	return e.original.Timestamp()
}
func (e *stubSubtaskEvent) GetAgentLevel() agent.AgentLevel {
	level := e.original.GetAgentLevel()
	if level != "" && level != agent.LevelCore {
		return level
	}
	return agent.LevelSubagent
}
func (e *stubSubtaskEvent) GetSessionID() string     { return e.original.GetSessionID() }
func (e *stubSubtaskEvent) GetRunID() string         { return e.original.GetRunID() }
func (e *stubSubtaskEvent) GetParentRunID() string   { return e.original.GetParentRunID() }
func (e *stubSubtaskEvent) GetCorrelationID() string { return e.original.GetCorrelationID() }
func (e *stubSubtaskEvent) GetCausationID() string   { return e.original.GetCausationID() }
func (e *stubSubtaskEvent) GetEventID() string       { return e.original.GetEventID() }
func (e *stubSubtaskEvent) GetSeq() uint64           { return e.original.GetSeq() }
func (e *stubSubtaskEvent) SubtaskDetails() agent.SubtaskMetadata {
	return e.meta
}
func (e *stubSubtaskEvent) WrappedEvent() agent.AgentEvent { return e.original }

var _ agent.SubtaskWrapper = (*stubSubtaskEvent)(nil)

func parseSSEStream(t *testing.T, payload string) []streamedEvent {
	t.Helper()

	blocks := strings.Split(strings.TrimSpace(payload), "\n\n")
	events := make([]streamedEvent, 0, len(blocks))
	for _, block := range blocks {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		var evt streamedEvent
		for _, line := range lines {
			if strings.HasPrefix(line, "event: ") {
				evt.event = strings.TrimPrefix(line, "event: ")
			}
			if strings.HasPrefix(line, "data: ") {
				raw := strings.TrimPrefix(line, "data: ")
				if err := json.Unmarshal([]byte(raw), &evt.data); err != nil {
					t.Fatalf("failed to unmarshal SSE payload: %v", err)
				}
			}
		}
		if evt.event != "" {
			events = append(events, evt)
		}
	}

	return events
}

func TestSSEConnectedEventIncludesActiveRunID(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	tracker := serverapp.NewTaskProgressTracker(serverapp.NewInMemoryTaskStore())
	handler := NewSSEHandler(broadcaster, WithSSERunTracker(tracker))

	sessionID := "session-active-run"
	tracker.RegisterRunSession(sessionID, "run-xyz")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID+"&replay=none", nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	// Wait for the connected event to be written
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.BodyString(), "connected") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	var connectedEvent streamedEvent
	for _, evt := range events {
		if evt.event == "connected" {
			connectedEvent = evt
			break
		}
	}

	if connectedEvent.event == "" {
		t.Fatalf("expected connected event, got %v", events)
	}
	if activeRunID, ok := connectedEvent.data["active_run_id"].(string); !ok || activeRunID != "run-xyz" {
		t.Fatalf("expected active_run_id=run-xyz in connected event, got %v", connectedEvent.data)
	}
}

func TestSSEConnectedEventEmptyActiveRunID(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-no-run"
	// No run registered for this session

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID+"&replay=none", nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.BodyString(), "connected") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	var connectedEvent streamedEvent
	for _, evt := range events {
		if evt.event == "connected" {
			connectedEvent = evt
			break
		}
	}

	if connectedEvent.event == "" {
		t.Fatalf("expected connected event, got %v", events)
	}
	activeRunID, ok := connectedEvent.data["active_run_id"].(string)
	if !ok {
		t.Fatalf("expected active_run_id field in connected event, got %v", connectedEvent.data)
	}
	if activeRunID != "" {
		t.Fatalf("expected empty active_run_id when no run is active, got %q", activeRunID)
	}
}

func TestIsDelegationToolEvent(t *testing.T) {
	now := time.Now()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "session-1", "task-1", "", now),
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "subagent:0",
		Payload: map[string]any{
			"tool_name": "subagent",
			"result":    "delegation summary",
		},
	}

	if !isDelegationToolEvent(env) {
		t.Fatalf("expected subagent tool envelope to be treated as delegation")
	}

	env.Payload["tool_name"] = "bash"
	env.NodeID = "bash:1"
	if isDelegationToolEvent(env) {
		t.Fatalf("expected non-subagent tool envelope to pass through")
	}
}

func TestSSEHandlerReplaysStepEventsAndFiltersLifecycle(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-replay"
	now := time.Now()
	base := domain.NewBaseEvent(agent.LevelCore, sessionID, "task-1", "parent-1", now)

	snapshot := &workflow.WorkflowSnapshot{
		ID:    "wf-1",
		Phase: workflow.PhaseRunning,
		Order: []string{"react:context", "react:finalize"},
		Nodes: []workflow.NodeSnapshot{
			{ID: "react:context", Status: workflow.NodeStatusSucceeded},
			{ID: "react:finalize", Status: workflow.NodeStatusPending},
		},
	}
	firstNode := snapshot.Nodes[0]

	lifecycleEvent := domain.NewLifecycleUpdatedEvent(
		base, snapshot.ID, workflow.EventWorkflowUpdated, snapshot.Phase, &firstNode, snapshot,
	)
	lifecycle := domain.NewWorkflowEnvelopeFromEvent(lifecycleEvent, "workflow.lifecycle.updated")
	if lifecycle == nil {
		t.Fatal("failed to create lifecycle envelope")
	}
	lifecycle.WorkflowID = snapshot.ID
	lifecycle.RunID = snapshot.ID
	lifecycle.NodeID = firstNode.ID
	lifecycle.NodeKind = "node"
	lifecycle.Payload = map[string]any{
		"workflow.lifecycle.updated_type": string(workflow.EventWorkflowUpdated),
		"phase":                           snapshot.Phase,
		"node":                            firstNode,
		"workflow":                        snapshot,
	}

	stepEvent := domain.NewNodeCompletedEvent(
		base, 0, "context", map[string]string{"status": "ok"}, "succeeded", 1, 0, 0, 0, snapshot,
	)
	stepEnvelope := domain.NewWorkflowEnvelopeFromEvent(stepEvent, "workflow.node.completed")
	if stepEnvelope == nil {
		t.Fatal("failed to create step envelope")
	}
	stepEnvelope.WorkflowID = snapshot.ID
	stepEnvelope.RunID = snapshot.ID
	stepEnvelope.NodeID = "context"
	stepEnvelope.NodeKind = "step"
	stepEnvelope.Payload = map[string]any{
		"step_index":       stepEvent.Data.StepIndex,
		"step_description": stepEvent.Data.StepDescription,
		"status":           stepEvent.Data.Status,
		"iteration":        stepEvent.Data.Iteration,
		"workflow":         snapshot,
	}

	// Seed history before establishing the connection to simulate a reconnecting client.
	broadcaster.OnEvent(lifecycle)
	broadcaster.OnEvent(stepEnvelope)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID, nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		payload := rec.BodyString()
		if strings.Contains(payload, "workflow.lifecycle.updated") && strings.Contains(payload, "workflow.node.completed") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	if len(events) < 2 { // connected + step envelope
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	var stepEnvelopeEvent streamedEvent
	for _, evt := range events {
		switch evt.event {
		case "workflow.lifecycle.updated":
			t.Fatalf("lifecycle event should not be streamed: %v", evt)
		case "workflow.node.completed":
			stepEnvelopeEvent = evt
		}
	}

	if stepEnvelopeEvent.event == "" {
		t.Fatalf("step_completed not replayed: %v", events)
	}

	stepPayload, ok := stepEnvelopeEvent.data["payload"].(map[string]any)
	if !ok {
		t.Fatalf("step payload missing or wrong type: %v", stepEnvelopeEvent.data)
	}
	workflowPayload, ok := stepPayload["workflow"].(map[string]any)
	if !ok {
		t.Fatalf("workflow payload missing or wrong type: %v", stepPayload)
	}
	if _, hasNodes := workflowPayload["nodes"]; hasNodes {
		t.Fatalf("workflow nodes should not be streamed: %v", workflowPayload)
	}
	if status := stepPayload["status"]; status != stepEvent.Data.Status {
		t.Fatalf("unexpected step status: %v", stepEnvelopeEvent.data)
	}
	if iteration := stepPayload["iteration"]; iteration != float64(stepEvent.Data.Iteration) { // JSON numbers decode to float64
		t.Fatalf("unexpected iteration: %v", stepEnvelopeEvent.data)
	}
}

func TestSSEHandlerDedupesBySeqDuringReplay(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-dedupe"
	runID := "run-dedupe"
	now := time.Now()

	primary := makeToolEnvelopeWithSeq(sessionID, runID, 1, now)
	duplicate := makeToolEnvelopeWithSeq(sessionID, runID, 1, now.Add(5*time.Millisecond))

	broadcaster.OnEvent(primary)
	broadcaster.OnEvent(duplicate)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID, nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.BodyString(), "workflow.tool.completed") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	count := 0
	for _, evt := range events {
		if evt.event == "workflow.tool.completed" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 workflow.tool.completed event after dedupe, got %d", count)
	}
}

func makeToolEnvelopeWithSeq(sessionID, runID string, seq uint64, ts time.Time) *domain.WorkflowEventEnvelope {
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, runID, "", ts),
		Version:   1,
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "bash:1",
		Payload: map[string]any{
			"tool_name": "bash",
			"result":    "ok",
		},
	}
	env.SetSeq(seq)
	return env
}

func TestSSEHandlerBlocksReactIterStepNodes(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-react-iter"
	now := time.Now()
	base := domain.NewBaseEvent(agent.LevelCore, sessionID, "task-react", "", now)

	stepEvent := domain.NewNodeCompletedEvent(
		base, 0, "react step tool", nil, "succeeded", 1, 0, 0, 0, nil,
	)
	stepEnvelope := domain.NewWorkflowEnvelopeFromEvent(stepEvent, "workflow.node.completed")
	if stepEnvelope == nil {
		t.Fatal("failed to create step envelope")
	}
	stepEnvelope.NodeID = "react:iter:1:tool:video_generate:0"
	stepEnvelope.NodeKind = "step"
	stepEnvelope.Payload = map[string]any{
		"step_index":       stepEvent.Data.StepIndex,
		"step_description": stepEvent.Data.StepDescription,
		"status":           stepEvent.Data.Status,
		"iteration":        stepEvent.Data.Iteration,
	}

	broadcaster.OnEvent(stepEnvelope)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID, nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	time.Sleep(150 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	for _, evt := range events {
		if evt.event == "workflow.node.completed" {
			t.Fatalf("react iter step node should be filtered: %v", evt)
		}
	}
}

func TestSSEHandlerDebugModeStreamsReactIterStepNodes(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-react-iter-debug"
	now := time.Now()
	base := domain.NewBaseEvent(agent.LevelCore, sessionID, "task-react", "", now)

	stepEvent := domain.NewNodeCompletedEvent(
		base, 0, "react step tool", nil, "succeeded", 1, 0, 0, 0, nil,
	)
	stepEnvelope := domain.NewWorkflowEnvelopeFromEvent(stepEvent, "workflow.node.completed")
	if stepEnvelope == nil {
		t.Fatal("failed to create step envelope")
	}
	stepEnvelope.NodeID = "react:iter:1:tool:video_generate:0"
	stepEnvelope.NodeKind = "step"
	stepEnvelope.Payload = map[string]any{
		"step_index":       stepEvent.Data.StepIndex,
		"step_description": stepEvent.Data.StepDescription,
		"status":           stepEvent.Data.Status,
		"iteration":        stepEvent.Data.Iteration,
	}

	broadcaster.OnEvent(stepEnvelope)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID+"&debug=1", nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.BodyString(), "workflow.node.completed") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	found := false
	for _, evt := range events {
		if evt.event == "workflow.node.completed" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("react iter step node should be streamed in debug mode: %v", events)
	}
}

func TestSSEHandlerFiltersDebugOnlyEventsByDefault(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-debug-only"
	runID := "run-debug"
	now := time.Now()

	debugOnly := []string{
		types.EventDiagnosticContextCompression,
		types.EventDiagnosticToolFiltering,
		types.EventProactiveContextRefresh,
	}

	for i, eventType := range debugOnly {
		base := domain.NewBaseEvent(agent.LevelCore, sessionID, runID, "", now.Add(time.Duration(i)*time.Millisecond))
		envelope := &domain.WorkflowEventEnvelope{
			BaseEvent: base,
			Event:     eventType,
			Version:   1,
			NodeKind:  "diagnostic",
			NodeID:    fmt.Sprintf("debug-%d", i),
			Payload:   map[string]any{"kind": eventType},
		}
		broadcaster.OnEvent(envelope)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID, nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.BodyString(), "connected") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	for _, evt := range events {
		for _, eventType := range debugOnly {
			if evt.event == eventType {
				t.Fatalf("debug-only event should be filtered without debug=1: %v", evt)
			}
		}
	}

	ctxDebug, cancelDebug := context.WithCancel(context.Background())
	defer cancelDebug()

	reqDebug := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID+"&debug=1", nil).WithContext(ctxDebug)
	recDebug := newSSERecorder()

	doneDebug := make(chan struct{})
	go func() {
		handler.HandleSSEStream(recDebug, reqDebug)
		close(doneDebug)
	}()

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		payload := recDebug.BodyString()
		found := false
		for _, eventType := range debugOnly {
			if strings.Contains(payload, eventType) {
				found = true
				break
			}
		}
		if found {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancelDebug()

	select {
	case <-doneDebug:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	debugEvents := parseSSEStream(t, recDebug.BodyString())
	for _, eventType := range debugOnly {
		found := false
		for _, evt := range debugEvents {
			if evt.event == eventType {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected debug-only event to stream with debug=1: %s", eventType)
		}
	}
}

func TestSSEHandlerStreamsReplanRequestedEvent(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-replan"
	runID := "run-replan"
	now := time.Now()

	base := domain.NewBaseEvent(agent.LevelCore, sessionID, runID, "", now)
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: base,
		Event:     types.EventReplanRequested,
		Version:   1,
		NodeKind:  "orchestrator",
		NodeID:    "replan",
		Payload: map[string]any{
			"tool_name": "web_search",
			"reason":    "orchestrator tool failure triggered replan injection",
			"error":     "boom",
		},
	}
	broadcaster.OnEvent(envelope)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID, nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.BodyString(), types.EventReplanRequested) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	found := false
	for _, evt := range events {
		if evt.event == types.EventReplanRequested {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected replan requested event to be streamed: %v", events)
	}
}

func TestSSEHandlerStreamsSubtaskEvents(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-subtask"
	now := time.Now()
	base := domain.NewBaseEvent(agent.LevelSubagent, sessionID, "task-sub", "parent-task", now)

	toolEvent := domain.NewToolCompletedEvent(
		base, "call-123", "web_search", "done", nil, 250*time.Millisecond, nil, nil,
	)
	envelope := domain.NewWorkflowEnvelopeFromEvent(toolEvent, "workflow.tool.completed")
	if envelope == nil {
		t.Fatal("failed to create tool envelope")
	}
	envelope.NodeID = toolEvent.Data.CallID
	envelope.NodeKind = "tool"
	envelope.Payload = map[string]any{
		"call_id":   toolEvent.Data.CallID,
		"tool_name": toolEvent.Data.ToolName,
		"result":    toolEvent.Data.Result,
		"duration":  toolEvent.Data.Duration.Milliseconds(),
	}

	subtask := &stubSubtaskEvent{
		original: envelope,
		meta: agent.SubtaskMetadata{
			Index:       1,
			Total:       3,
			Preview:     "inspect UI output",
			MaxParallel: 2,
		},
	}

	// Seed history with subtask-wrapped event.
	broadcaster.OnEvent(subtask)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID, nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.BodyString(), "workflow.tool.completed") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	var streamed streamedEvent
	for _, evt := range events {
		if evt.event == "workflow.tool.completed" {
			streamed = evt
			break
		}
	}

	if streamed.event == "" {
		t.Fatalf("subtask event not streamed: %v", events)
	}
	if level := streamed.data["agent_level"]; level != string(agent.LevelSubagent) {
		t.Fatalf("expected subagent level, got %v", level)
	}
	if isSubtask := streamed.data["is_subtask"]; isSubtask != true {
		t.Fatalf("expected is_subtask=true, got %v", isSubtask)
	}
	if idx := streamed.data["subtask_index"]; idx != float64(subtask.meta.Index) {
		t.Fatalf("unexpected subtask_index: %v", idx)
	}
	payload, ok := streamed.data["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload missing or wrong type: %v", streamed.data)
	}
	if callID := payload["call_id"]; callID != toolEvent.Data.CallID {
		t.Fatalf("unexpected call_id in payload: %v", payload)
	}
}

func TestSSEHandlerStreamsSubagentToolStartAndComplete(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-subagent-tool"
	taskID := "task-main"
	parentTaskID := "task-parent"
	now := time.Now()
	base := domain.NewBaseEvent(agent.LevelCore, sessionID, taskID, parentTaskID, now)

	startEvent := domain.NewToolStartedEvent(
		base, 1, "call-subagent-1", "subagent", map[string]interface{}{"prompt": "inspect the backend pipeline"},
	)
	startEnvelope := domain.NewWorkflowEnvelopeFromEvent(startEvent, "workflow.tool.started")
	if startEnvelope == nil {
		t.Fatal("failed to create start envelope")
	}
	startEnvelope.NodeID = startEvent.Data.CallID
	startEnvelope.NodeKind = "tool"
	startEnvelope.Payload = map[string]any{
		"call_id":   startEvent.Data.CallID,
		"tool_name": startEvent.Data.ToolName,
		"arguments": startEvent.Data.Arguments,
		"iteration": startEvent.Data.Iteration,
	}

	completeEvent := domain.NewToolCompletedEvent(
		base, startEvent.Data.CallID, "subagent", "delegation complete", nil, 175*time.Millisecond, nil, nil,
	)
	completeEnvelope := domain.NewWorkflowEnvelopeFromEvent(completeEvent, "workflow.tool.completed")
	if completeEnvelope == nil {
		t.Fatal("failed to create completion envelope")
	}
	completeEnvelope.NodeID = completeEvent.Data.CallID
	completeEnvelope.NodeKind = "tool"
	completeEnvelope.Payload = map[string]any{
		"call_id":   completeEvent.Data.CallID,
		"tool_name": completeEvent.Data.ToolName,
		"result":    completeEvent.Data.Result,
		"duration":  completeEvent.Data.Duration.Milliseconds(),
	}

	broadcaster.OnEvent(startEnvelope)
	broadcaster.OnEvent(completeEnvelope)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID, nil).WithContext(ctx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(rec.BodyString(), "workflow.tool.completed") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	var started, completed map[string]any
	for _, evt := range events {
		switch evt.event {
		case "workflow.tool.started":
			started = evt.data
		case "workflow.tool.completed":
			completed = evt.data
		}
	}

	if started == nil {
		t.Fatalf("expected workflow.tool.started to be streamed: %v", events)
	}
	if completed == nil {
		t.Fatalf("expected workflow.tool.completed to be streamed: %v", events)
	}

	for _, payload := range []map[string]any{started, completed} {
		if payload["run_id"] != taskID {
			t.Fatalf("expected run_id %s, got %v", taskID, payload["run_id"])
		}
		if payload["parent_run_id"] != parentTaskID {
			t.Fatalf("expected parent_run_id %s, got %v", parentTaskID, payload["parent_run_id"])
		}
		toolPayload, ok := payload["payload"].(map[string]any)
		if !ok {
			t.Fatalf("expected payload map in event data, got %T", payload["payload"])
		}
		if toolPayload["tool_name"] != "subagent" {
			t.Fatalf("expected tool_name subagent, got %v", toolPayload["tool_name"])
		}
	}
}

func TestSSEHandlerRejectsInvalidSessionID(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id=../bad", nil)
	rec := httptest.NewRecorder()

	handler.HandleSSEStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d", rec.Code)
	}
}

func TestSanitizeAttachmentsForStreamResendsUpdates(t *testing.T) {
	sent := newStringLRU(sseSentAttachmentCacheSize)
	cache := NewDataCache(4, time.Minute)
	attachments := map[string]ports.Attachment{
		"note.txt": {
			Name:      "note.txt",
			MediaType: "text/plain",
			Data:      "ZmlsZSBkYXRh",
		},
	}

	first := sanitizeAttachmentsForStream(attachments, sent, cache, false)
	if first == nil || len(first) != 1 {
		t.Fatalf("expected initial attachments to be forwarded, got %#v", first)
	}
	if first["note.txt"].URI == "" || strings.HasPrefix(first["note.txt"].URI, "data:") {
		t.Fatalf("expected inline payload to be cached with URL, got %#v", first["note.txt"])
	}

	// Re-sending the same attachment payload should be suppressed.
	if dup := sanitizeAttachmentsForStream(attachments, sent, cache, false); dup != nil {
		t.Fatalf("expected duplicate attachments to be filtered: %#v", dup)
	}

	updated := map[string]ports.Attachment{
		"note.txt": {
			Name:      "note.txt",
			MediaType: "text/plain",
			URI:       "https://cdn.example.com/note.txt",
		},
	}

	resent := sanitizeAttachmentsForStream(updated, sent, cache, false)
	if resent == nil || len(resent) != 1 {
		t.Fatalf("expected updated attachment to be forwarded, got %#v", resent)
	}
	if resent["note.txt"].URI == "" {
		t.Fatalf("expected updated attachment URI to be preserved: %#v", resent)
	}
}

func TestSanitizeWorkflowEnvelopePayloadStripsStepResultMessages(t *testing.T) {
	cache := NewDataCache(4, time.Minute)
	env := &domain.WorkflowEventEnvelope{
		Event:    "workflow.node.completed",
		NodeKind: "step",
		Payload: map[string]any{
			"result": map[string]any{
				"summary": "done",
				"messages": []any{
					map[string]any{"role": "system", "content": "very long prompt"},
				},
				"attachments": map[string]any{
					"inline.png": map[string]any{
						"name":       "inline.png",
						"media_type": "image/png",
						"data":       "aGVsbG8=",
					},
				},
			},
		},
	}

	payload := sanitizeWorkflowEnvelopePayload(env, newStringLRU(sseSentAttachmentCacheSize), cache)
	res, ok := payload["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected sanitized result map, got %T", payload["result"])
	}
	if _, hasMessages := res["messages"]; hasMessages {
		t.Fatalf("expected messages to be stripped from result")
	}

	stepResult, ok := payload["step_result"].(string)
	if !ok || stepResult != "done" {
		t.Fatalf("expected step_result summary, got %v", payload["step_result"])
	}

	attachments, ok := res["attachments"].(map[string]ports.Attachment)
	if !ok {
		t.Fatalf("expected attachments map, got %T", res["attachments"])
	}
	att := attachments["inline.png"]
	if att.Data != "" || !strings.HasPrefix(att.URI, "/api/data/") {
		t.Fatalf("expected attachment data to be cached with URI, got %#v", att)
	}
}

func TestSanitizeEnvelopePayloadStripsInlineAttachments(t *testing.T) {
	cache := NewDataCache(4, time.Minute)
	raw := map[string]any{
		"status": "succeeded",
		"attachments": map[string]any{
			"inline.png": map[string]any{
				"name":       "inline.png",
				"media_type": "image/png",
				"data":       "aGVsbG8=", // "hello" base64
			},
		},
	}

	sanitized := sanitizeEnvelopePayload(raw, newStringLRU(sseSentAttachmentCacheSize), cache)
	if sanitized == nil {
		t.Fatalf("expected sanitized payload")
	}

	attachments, ok := sanitized["attachments"].(map[string]ports.Attachment)
	if !ok {
		t.Fatalf("expected attachments map, got %T", sanitized["attachments"])
	}

	att, ok := attachments["inline.png"]
	if !ok {
		t.Fatalf("expected inline attachment to be preserved")
	}
	if att.Data != "" {
		t.Fatalf("expected attachment data to be stripped, got %q", att.Data)
	}
	if att.URI == "" || !strings.HasPrefix(att.URI, "/api/data/") {
		t.Fatalf("expected attachment URI to reference data cache, got %q", att.URI)
	}
	if att.MediaType != "image/png" {
		t.Fatalf("expected media type to be preserved, got %q", att.MediaType)
	}
}

func TestSanitizeAttachmentsForStreamCachesHTML(t *testing.T) {
	cache := NewDataCache(4, time.Minute)
	sent := newStringLRU(sseSentAttachmentCacheSize)
	attachments := map[string]ports.Attachment{
		"game.html": {
			Name:      "game.html",
			MediaType: "text/html",
			Data:      base64.StdEncoding.EncodeToString([]byte("<html><body>play</body></html>")),
		},
	}

	sanitized := sanitizeAttachmentsForStream(attachments, sent, cache, false)
	if len(sanitized) != 1 {
		t.Fatalf("expected 1 sanitized attachment, got %d", len(sanitized))
	}
	att := sanitized["game.html"]
	if att.URI == "" || !strings.Contains(att.URI, "/api/data/") {
		t.Fatalf("expected cached URI pointing to data endpoint, got %q", att.URI)
	}
	if att.Data == "" {
		t.Fatalf("expected inline data retained for HTML")
	}
	if att.PreviewProfile != "document.html" {
		t.Fatalf("expected HTML preview profile, got %q", att.PreviewProfile)
	}
	if len(att.PreviewAssets) == 0 || att.PreviewAssets[0].CDNURL != att.URI {
		t.Fatalf("expected preview asset pointing to stored URI, got %#v", att.PreviewAssets)
	}
}

func TestNormalizeAttachmentPayloadExternalizesHTML(t *testing.T) {
	cache := NewDataCache(4, time.Minute)
	att := ports.Attachment{
		Name:      "demo.html",
		MediaType: "text/html",
		Data:      base64.StdEncoding.EncodeToString([]byte("<html><body>ok</body></html>")),
	}

	out := normalizeAttachmentPayload(att, cache)
	if out.URI == "" || !strings.Contains(out.URI, "/api/data/") {
		t.Fatalf("expected cached URI pointing to data endpoint, got %q", out.URI)
	}
	if out.Data == "" {
		t.Fatalf("expected inline data retained for HTML")
	}
	if out.PreviewProfile != "document.html" {
		t.Fatalf("expected HTML preview profile, got %q", out.PreviewProfile)
	}
	if len(out.PreviewAssets) == 0 || out.PreviewAssets[0].CDNURL != out.URI {
		t.Fatalf("expected preview asset pointing to stored URI, got %#v", out.PreviewAssets)
	}
}

func TestSanitizeEnvelopePayloadRetainsInlineMarkdown(t *testing.T) {
	cache := NewDataCache(4, time.Minute)
	raw := map[string]any{
		"status": "succeeded",
		"attachments": map[string]any{
			"note.md": map[string]any{
				"name":       "note.md",
				"media_type": "text/markdown",
				"data":       base64.StdEncoding.EncodeToString([]byte("# Title\ncontent")),
			},
		},
	}

	sanitized := sanitizeEnvelopePayload(raw, newStringLRU(sseSentAttachmentCacheSize), cache)
	attachments, ok := sanitized["attachments"].(map[string]ports.Attachment)
	if !ok {
		t.Fatalf("expected attachments map, got %T", sanitized["attachments"])
	}

	att, ok := attachments["note.md"]
	if !ok {
		t.Fatalf("expected markdown attachment to be preserved")
	}
	if att.URI == "" || !strings.HasPrefix(att.URI, "/api/data/") {
		t.Fatalf("expected attachment URI to reference data cache, got %q", att.URI)
	}
	if att.Data == "" {
		t.Fatalf("expected inline markdown payload to be retained for fallback")
	}
	if att.MediaType != "text/markdown" {
		t.Fatalf("expected markdown media type to be preserved, got %q", att.MediaType)
	}
}

func TestBuildEventDataForceIncludesAttachmentsOnTerminalFinal(t *testing.T) {
	handler := NewSSEHandler(serverapp.NewEventBroadcaster())
	cache := NewDataCache(4, time.Minute)
	store, err := NewAttachmentStore(attachments.StoreConfig{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("failed to create attachment store: %v", err)
	}
	handler.dataCache = cache
	handler.attachmentStore = store

	now := time.Now()
	base := domain.NewBaseEvent(agent.LevelCore, "session-final", "run-final", "", now)

	imgData := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))
	typedAtts := map[string]ports.Attachment{
		"chart.png": {
			Name:      "chart.png",
			MediaType: "image/png",
			Data:      imgData,
		},
	}

	// First, simulate a tool-completed event that sends the attachment so it
	// gets registered in the LRU.
	toolEnvelope := &domain.WorkflowEventEnvelope{
		BaseEvent: base,
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "tool:0",
		Payload: map[string]any{
			"tool_name":   "image_gen",
			"attachments": typedAtts,
		},
	}
	sent := newStringLRU(sseSentAttachmentCacheSize)
	finalAnswerCache := newStringLRU(sseFinalAnswerCacheSize)

	_, err = handler.buildEventData(toolEnvelope, sent, finalAnswerCache, true)
	if err != nil {
		t.Fatalf("buildEventData for tool event: %v", err)
	}

	// Now send the terminal final event with the same attachments.
	finalEnvelope := &domain.WorkflowEventEnvelope{
		BaseEvent: base,
		Event:     "workflow.result.final",
		Payload: map[string]any{
			"stream_finished": true,
			"final_answer":    "Here is your chart",
			"attachments":     typedAtts,
		},
	}

	data, err := handler.buildEventData(finalEnvelope, sent, finalAnswerCache, true)
	if err != nil {
		t.Fatalf("buildEventData for final event: %v", err)
	}

	payload, ok := data["payload"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload map, got %T", data["payload"])
	}

	atts, ok := payload["attachments"].(map[string]ports.Attachment)
	if !ok {
		t.Fatalf("expected attachments in final payload, got %T", payload["attachments"])
	}
	att, ok := atts["chart.png"]
	if !ok {
		t.Fatalf("expected chart.png in final attachments, got keys: %v", atts)
	}
	if att.URI == "" {
		t.Fatalf("expected attachment to have a URI, got empty")
	}
}

func TestNormalizeAttachmentPayloadCachesImagePayload(t *testing.T) {
	cache := NewDataCache(4, time.Minute)

	pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A} // PNG header
	att := ports.Attachment{
		Name:      "diagram.png",
		MediaType: "image/png",
		Data:      base64.StdEncoding.EncodeToString(pngData),
	}

	out := normalizeAttachmentPayload(att, cache)
	if out.URI == "" || !strings.Contains(out.URI, "/api/data/") {
		t.Fatalf("expected cached URI pointing to data endpoint, got %q", out.URI)
	}
	if out.Data != "" {
		t.Fatalf("expected binary data to be cleared after caching, got non-empty")
	}
	if out.MediaType != "image/png" {
		t.Fatalf("expected media type to be preserved, got %q", out.MediaType)
	}
}

func TestNormalizeAttachmentPayloadCachesDataURI(t *testing.T) {
	cache := NewDataCache(4, time.Minute)

	pdfData := []byte("%PDF-1.4 fake content")
	dataURI := "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(pdfData)
	att := ports.Attachment{
		Name: "report.pdf",
		URI:  dataURI,
	}

	out := normalizeAttachmentPayload(att, cache)
	if out.URI == "" || !strings.Contains(out.URI, "/api/data/") {
		t.Fatalf("expected cached URI pointing to data endpoint, got %q", out.URI)
	}
	if out.Data != "" {
		t.Fatalf("expected data to be cleared after caching")
	}
	if out.MediaType != "application/pdf" {
		t.Fatalf("expected media type to be set from data URI, got %q", out.MediaType)
	}
}

func TestNormalizeAttachmentPayloadFallsBackToCacheWithoutStore(t *testing.T) {
	cache := NewDataCache(4, time.Minute)

	att := ports.Attachment{
		Name:      "photo.jpg",
		MediaType: "image/jpeg",
		Data:      base64.StdEncoding.EncodeToString([]byte("fake-jpeg")),
	}

	out := normalizeAttachmentPayload(att, cache)
	if out.URI == "" || !strings.HasPrefix(out.URI, "/api/data/") {
		t.Fatalf("expected data cache URI, got %q", out.URI)
	}
}

func TestNormalizeAttachmentPayloadRetainsInlineTextPayload(t *testing.T) {
	cache := NewDataCache(4, time.Minute)
	content := []byte("small text content")
	att := ports.Attachment{
		Name:      "notes.txt",
		MediaType: "text/plain",
		Data:      base64.StdEncoding.EncodeToString(content),
	}

	out := normalizeAttachmentPayload(att, cache)
	if out.URI == "" || !strings.Contains(out.URI, "/api/data/") {
		t.Fatalf("expected cached URI, got %q", out.URI)
	}
	// Small text payloads should be retained inline.
	if out.Data == "" {
		t.Fatalf("expected small text payload to be retained inline")
	}
}

func TestCoerceAttachmentMapTyped(t *testing.T) {
	typed := map[string]ports.Attachment{
		"img.png": {Name: "img.png", URI: "https://cdn/img.png"},
	}
	result := coerceAttachmentMap(typed)
	if len(result) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result))
	}
	if result["img.png"].URI != "https://cdn/img.png" {
		t.Fatalf("expected URI preserved, got %q", result["img.png"].URI)
	}
}

func TestCoerceAttachmentMapUntyped(t *testing.T) {
	// Simulates JSON round-trip through Postgres: map[string]any with map[string]any values.
	untyped := map[string]any{
		"report.pdf": map[string]any{
			"name":       "report.pdf",
			"media_type": "application/pdf",
			"uri":        "https://cdn/report.pdf",
			"source":     "artifacts_write",
		},
	}
	result := coerceAttachmentMap(untyped)
	if len(result) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result))
	}
	att := result["report.pdf"]
	if att.Name != "report.pdf" {
		t.Fatalf("expected name report.pdf, got %q", att.Name)
	}
	if att.MediaType != "application/pdf" {
		t.Fatalf("expected media_type application/pdf, got %q", att.MediaType)
	}
	if att.URI != "https://cdn/report.pdf" {
		t.Fatalf("expected URI https://cdn/report.pdf, got %q", att.URI)
	}
}

func TestCoerceAttachmentMapNilAndEmpty(t *testing.T) {
	if result := coerceAttachmentMap(nil); result != nil {
		t.Fatalf("expected nil for nil input, got %v", result)
	}
	if result := coerceAttachmentMap("not-a-map"); result != nil {
		t.Fatalf("expected nil for string input, got %v", result)
	}
	if result := coerceAttachmentMap(map[string]any{}); result != nil {
		t.Fatalf("expected nil for empty map, got %v", result)
	}
}

func TestCoerceAttachmentMapFallsBackToKey(t *testing.T) {
	// When name is missing, the map key should be used.
	untyped := map[string]any{
		"chart.svg": map[string]any{
			"media_type": "image/svg+xml",
			"uri":        "https://cdn/chart.svg",
		},
	}
	result := coerceAttachmentMap(untyped)
	if len(result) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result))
	}
	if result["chart.svg"].Name != "chart.svg" {
		t.Fatalf("expected name from key, got %q", result["chart.svg"].Name)
	}
}
