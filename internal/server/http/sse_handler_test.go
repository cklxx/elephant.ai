package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/attachments"
	serverapp "alex/internal/server/app"
	"alex/internal/testutil"
	"alex/internal/workflow"
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
func (e *stubSubtaskEvent) GetSessionID() string      { return e.original.GetSessionID() }
func (e *stubSubtaskEvent) GetRunID() string           { return e.original.GetRunID() }
func (e *stubSubtaskEvent) GetParentRunID() string     { return e.original.GetParentRunID() }
func (e *stubSubtaskEvent) GetCorrelationID() string   { return e.original.GetCorrelationID() }
func (e *stubSubtaskEvent) GetCausationID() string     { return e.original.GetCausationID() }
func (e *stubSubtaskEvent) GetEventID() string         { return e.original.GetEventID() }
func (e *stubSubtaskEvent) GetSeq() uint64             { return e.original.GetSeq() }
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

	lifecycleEvent := &domain.WorkflowLifecycleUpdatedEvent{
		BaseEvent:         base,
		WorkflowID:        snapshot.ID,
		WorkflowEventType: workflow.EventWorkflowUpdated,
		Phase:             snapshot.Phase,
		Node:              &firstNode,
		Workflow:          snapshot,
	}
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

	stepEvent := &domain.WorkflowNodeCompletedEvent{
		BaseEvent:       base,
		StepIndex:       0,
		StepDescription: "context",
		StepResult:      map[string]string{"status": "ok"},
		Status:          "succeeded",
		Iteration:       1,
		Workflow:        snapshot,
	}
	stepEnvelope := domain.NewWorkflowEnvelopeFromEvent(stepEvent, "workflow.node.completed")
	if stepEnvelope == nil {
		t.Fatal("failed to create step envelope")
	}
	stepEnvelope.WorkflowID = snapshot.ID
	stepEnvelope.RunID = snapshot.ID
	stepEnvelope.NodeID = "context"
	stepEnvelope.NodeKind = "step"
	stepEnvelope.Payload = map[string]any{
		"step_index":       stepEvent.StepIndex,
		"step_description": stepEvent.StepDescription,
		"status":           stepEvent.Status,
		"iteration":        stepEvent.Iteration,
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
	if status := stepPayload["status"]; status != stepEvent.Status {
		t.Fatalf("unexpected step status: %v", stepEnvelopeEvent.data)
	}
	if iteration := stepPayload["iteration"]; iteration != float64(stepEvent.Iteration) { // JSON numbers decode to float64
		t.Fatalf("unexpected iteration: %v", stepEnvelopeEvent.data)
	}
}

func TestSSEHandlerBlocksReactIterStepNodes(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-react-iter"
	now := time.Now()
	base := domain.NewBaseEvent(agent.LevelCore, sessionID, "task-react", "", now)

	stepEvent := &domain.WorkflowNodeCompletedEvent{
		BaseEvent:       base,
		StepIndex:       0,
		StepDescription: "react step tool",
		Status:          "succeeded",
		Iteration:       1,
	}
	stepEnvelope := domain.NewWorkflowEnvelopeFromEvent(stepEvent, "workflow.node.completed")
	if stepEnvelope == nil {
		t.Fatal("failed to create step envelope")
	}
	stepEnvelope.NodeID = "react:iter:1:tool:video_generate:0"
	stepEnvelope.NodeKind = "step"
	stepEnvelope.Payload = map[string]any{
		"step_index":       stepEvent.StepIndex,
		"step_description": stepEvent.StepDescription,
		"status":           stepEvent.Status,
		"iteration":        stepEvent.Iteration,
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

	stepEvent := &domain.WorkflowNodeCompletedEvent{
		BaseEvent:       base,
		StepIndex:       0,
		StepDescription: "react step tool",
		Status:          "succeeded",
		Iteration:       1,
	}
	stepEnvelope := domain.NewWorkflowEnvelopeFromEvent(stepEvent, "workflow.node.completed")
	if stepEnvelope == nil {
		t.Fatal("failed to create step envelope")
	}
	stepEnvelope.NodeID = "react:iter:1:tool:video_generate:0"
	stepEnvelope.NodeKind = "step"
	stepEnvelope.Payload = map[string]any{
		"step_index":       stepEvent.StepIndex,
		"step_description": stepEvent.StepDescription,
		"status":           stepEvent.Status,
		"iteration":        stepEvent.Iteration,
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

func TestSSEHandlerStreamsSubtaskEvents(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-subtask"
	now := time.Now()
	base := domain.NewBaseEvent(agent.LevelSubagent, sessionID, "task-sub", "parent-task", now)

	toolEvent := &domain.WorkflowToolCompletedEvent{
		BaseEvent: base,
		CallID:    "call-123",
		ToolName:  "web_search",
		Result:    "done",
		Duration:  250 * time.Millisecond,
	}
	envelope := domain.NewWorkflowEnvelopeFromEvent(toolEvent, "workflow.tool.completed")
	if envelope == nil {
		t.Fatal("failed to create tool envelope")
	}
	envelope.NodeID = toolEvent.CallID
	envelope.NodeKind = "tool"
	envelope.Payload = map[string]any{
		"call_id":   toolEvent.CallID,
		"tool_name": toolEvent.ToolName,
		"result":    toolEvent.Result,
		"duration":  toolEvent.Duration.Milliseconds(),
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
	if callID := payload["call_id"]; callID != toolEvent.CallID {
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

	startEvent := &domain.WorkflowToolStartedEvent{
		BaseEvent: base,
		Iteration: 1,
		CallID:    "call-subagent-1",
		ToolName:  "subagent",
		Arguments: map[string]any{"prompt": "inspect the backend pipeline"},
	}
	startEnvelope := domain.NewWorkflowEnvelopeFromEvent(startEvent, "workflow.tool.started")
	if startEnvelope == nil {
		t.Fatal("failed to create start envelope")
	}
	startEnvelope.NodeID = startEvent.CallID
	startEnvelope.NodeKind = "tool"
	startEnvelope.Payload = map[string]any{
		"call_id":   startEvent.CallID,
		"tool_name": startEvent.ToolName,
		"arguments": startEvent.Arguments,
		"iteration": startEvent.Iteration,
	}

	completeEvent := &domain.WorkflowToolCompletedEvent{
		BaseEvent: base,
		CallID:    startEvent.CallID,
		ToolName:  "subagent",
		Result:    "delegation complete",
		Duration:  175 * time.Millisecond,
	}
	completeEnvelope := domain.NewWorkflowEnvelopeFromEvent(completeEvent, "workflow.tool.completed")
	if completeEnvelope == nil {
		t.Fatal("failed to create completion envelope")
	}
	completeEnvelope.NodeID = completeEvent.CallID
	completeEnvelope.NodeKind = "tool"
	completeEnvelope.Payload = map[string]any{
		"call_id":   completeEvent.CallID,
		"tool_name": completeEvent.ToolName,
		"result":    completeEvent.Result,
		"duration":  completeEvent.Duration.Milliseconds(),
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

func TestSSEHandler_ReplaysPostgresHistory(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	historyStore := serverapp.NewPostgresEventHistoryStore(pool)
	if err := historyStore.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure history schema: %v", err)
	}

	sessionID := "session-replay-postgres"
	event := domain.NewWorkflowInputReceivedEvent(
		agent.LevelCore,
		sessionID,
		"task-1",
		"",
		"hello",
		nil,
		time.Now(),
	)
	writer := serverapp.NewEventBroadcaster(serverapp.WithEventHistoryStore(historyStore))
	writer.OnEvent(event)

	reader := serverapp.NewEventBroadcaster(serverapp.WithEventHistoryStore(historyStore))
	handler := NewSSEHandler(reader)

	reqCtx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id="+sessionID+"&replay=session", nil).WithContext(reqCtx)
	rec := newSSERecorder()

	done := make(chan struct{})
	go func() {
		handler.HandleSSEStream(rec, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SSE handler did not terminate after context cancellation")
	}

	events := parseSSEStream(t, rec.BodyString())
	found := false
	for _, evt := range events {
		if evt.event == event.EventType() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected replayed event %q, got %#v", event.EventType(), events)
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

	first := sanitizeAttachmentsForStream(attachments, sent, cache, nil, false)
	if first == nil || len(first) != 1 {
		t.Fatalf("expected initial attachments to be forwarded, got %#v", first)
	}
	if first["note.txt"].URI == "" || strings.HasPrefix(first["note.txt"].URI, "data:") {
		t.Fatalf("expected inline payload to be cached with URL, got %#v", first["note.txt"])
	}

	// Re-sending the same attachment payload should be suppressed.
	if dup := sanitizeAttachmentsForStream(attachments, sent, cache, nil, false); dup != nil {
		t.Fatalf("expected duplicate attachments to be filtered: %#v", dup)
	}

	updated := map[string]ports.Attachment{
		"note.txt": {
			Name:      "note.txt",
			MediaType: "text/plain",
			URI:       "https://cdn.example.com/note.txt",
		},
	}

	resent := sanitizeAttachmentsForStream(updated, sent, cache, nil, false)
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

	payload := sanitizeWorkflowEnvelopePayload(env, newStringLRU(sseSentAttachmentCacheSize), cache, nil)
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

	sanitized := sanitizeEnvelopePayload(raw, newStringLRU(sseSentAttachmentCacheSize), cache, nil)
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

func TestSanitizeAttachmentsForStreamPersistsHTMLToStore(t *testing.T) {
	cache := NewDataCache(4, time.Minute)
	sent := newStringLRU(sseSentAttachmentCacheSize)
	store, err := NewAttachmentStore(attachments.StoreConfig{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("failed to create attachment store: %v", err)
	}
	attachments := map[string]ports.Attachment{
		"game.html": {
			Name:      "game.html",
			MediaType: "text/html",
			Data:      base64.StdEncoding.EncodeToString([]byte("<html><body>play</body></html>")),
		},
	}

	sanitized := sanitizeAttachmentsForStream(attachments, sent, cache, store, false)
	if len(sanitized) != 1 {
		t.Fatalf("expected 1 sanitized attachment, got %d", len(sanitized))
	}
	att := sanitized["game.html"]
	if att.URI == "" || !strings.Contains(att.URI, "/api/attachments/") {
		t.Fatalf("expected stored URI pointing to attachments endpoint, got %q", att.URI)
	}
	if att.Data != "" {
		t.Fatalf("expected data to be cleared after persistence")
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
	store, err := NewAttachmentStore(attachments.StoreConfig{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("failed to create attachment store: %v", err)
	}
	att := ports.Attachment{
		Name:      "demo.html",
		MediaType: "text/html",
		Data:      base64.StdEncoding.EncodeToString([]byte("<html><body>ok</body></html>")),
	}

	out := normalizeAttachmentPayload(att, cache, store)
	if out.URI == "" || !strings.Contains(out.URI, "/api/attachments/") {
		t.Fatalf("expected stored URI pointing to attachments endpoint, got %q", out.URI)
	}
	if out.Data != "" {
		t.Fatalf("expected data to be cleared after persistence")
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

	sanitized := sanitizeEnvelopePayload(raw, newStringLRU(sseSentAttachmentCacheSize), cache, nil)
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
