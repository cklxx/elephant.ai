package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	serverapp "alex/internal/server/app"
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

func TestSSEHandlerReplaysWorkflowAndStepEvents(t *testing.T) {
	broadcaster := serverapp.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	sessionID := "session-replay"
	now := time.Now()
	base := domain.NewBaseEvent(ports.LevelCore, sessionID, "task-1", "parent-1", now)

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

	lifecycle := &domain.WorkflowLifecycleEvent{
		BaseEvent:         base,
		WorkflowID:        snapshot.ID,
		WorkflowEventType: workflow.EventWorkflowUpdated,
		Phase:             snapshot.Phase,
		Node:              &firstNode,
		Workflow:          snapshot,
	}

	stepCompleted := &domain.StepCompletedEvent{
		BaseEvent:       base,
		StepIndex:       0,
		StepDescription: "context",
		StepResult:      map[string]string{"status": "ok"},
		Status:          "succeeded",
		Iteration:       1,
		Workflow:        snapshot,
	}

	// Seed history before establishing the connection to simulate a reconnecting client.
	broadcaster.OnEvent(lifecycle)
	broadcaster.OnEvent(stepCompleted)

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
		if strings.Contains(payload, "workflow_event") && strings.Contains(payload, "step_completed") {
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
	if len(events) < 3 { // connected + two replayed events
		t.Fatalf("expected at least 3 events, got %d", len(events))
	}

	var workflowEvent streamedEvent
	var stepEvent streamedEvent
	for _, evt := range events {
		switch evt.event {
		case lifecycle.EventType():
			workflowEvent = evt
		case stepCompleted.EventType():
			stepEvent = evt
		}
	}

	if workflowEvent.event == "" {
		t.Fatalf("workflow_event not replayed: %v", events)
	}
	if stepEvent.event == "" {
		t.Fatalf("step_completed not replayed: %v", events)
	}

	workflowPayload, ok := workflowEvent.data["workflow"].(map[string]any)
	if !ok {
		t.Fatalf("workflow payload missing or wrong type: %v", workflowEvent.data)
	}
	if order, ok := workflowPayload["order"].([]any); ok {
		got := make([]string, 0, len(order))
		for _, value := range order {
			if s, ok := value.(string); ok {
				got = append(got, s)
			}
		}
		if len(got) != len(snapshot.Order) {
			t.Fatalf("workflow order length mismatch: %v", got)
		}
		for idx, expected := range snapshot.Order {
			if got[idx] != expected {
				t.Fatalf("workflow order mismatch at %d: got %v expected %v", idx, got, snapshot.Order)
			}
		}
	} else {
		t.Fatalf("workflow order missing: %v", workflowPayload)
	}

	if phase := workflowEvent.data["phase"]; phase != string(snapshot.Phase) {
		t.Fatalf("unexpected workflow phase: %v", workflowEvent.data)
	}

	if status := stepEvent.data["status"]; status != stepCompleted.Status {
		t.Fatalf("unexpected step status: %v", stepEvent.data)
	}
	if iteration := stepEvent.data["iteration"]; iteration != float64(stepCompleted.Iteration) { // JSON numbers decode to float64
		t.Fatalf("unexpected iteration: %v", stepEvent.data)
	}
}

func TestSanitizeAttachmentsForStreamResendsUpdates(t *testing.T) {
	sent := make(map[string]string)
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
