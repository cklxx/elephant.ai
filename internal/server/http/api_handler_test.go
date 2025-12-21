package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agentPorts "alex/internal/agent/ports"
	"alex/internal/analytics/journal"
	"alex/internal/server/app"
	"alex/internal/session/filestore"
	sessionstate "alex/internal/session/state_store"
)

type failingAgentCoordinator struct {
	err error
}

func (f *failingAgentCoordinator) GetSession(ctx context.Context, id string) (*agentPorts.Session, error) {
	return nil, f.err
}

func (f *failingAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentPorts.EventListener) (*agentPorts.TaskResult, error) {
	return nil, errors.New("not implemented")
}

func (f *failingAgentCoordinator) GetConfig() agentPorts.AgentConfig {
	return agentPorts.AgentConfig{}
}

type stubAgentCoordinator struct{}

func (stubAgentCoordinator) GetSession(ctx context.Context, id string) (*agentPorts.Session, error) {
	if id == "" {
		id = "stub-session"
	}
	return &agentPorts.Session{
		ID:        id,
		Messages:  []agentPorts.Message{},
		Metadata:  map[string]string{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}, nil
}

func (stubAgentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agentPorts.EventListener) (*agentPorts.TaskResult, error) {
	return &agentPorts.TaskResult{SessionID: sessionID}, nil
}

func (stubAgentCoordinator) GetConfig() agentPorts.AgentConfig {
	return agentPorts.AgentConfig{}
}

func TestHandleCreateTaskReturnsJSONErrorOnSessionDecodeFailure(t *testing.T) {
	rootErr := errors.New("json: cannot unmarshal object into Go struct field ToolResult.messages.tool_results.error of type error")
	coordinator := app.NewServerCoordinator(&failingAgentCoordinator{err: rootErr}, app.NewEventBroadcaster(), nil, nil, nil)
	handler := NewAPIHandler(coordinator, app.NewHealthChecker(), false)

	reqBody := bytes.NewBufferString(`{"task":"demo"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/tasks", reqBody)
	rr := httptest.NewRecorder()

	handler.HandleCreateTask(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("expected JSON content type, got %s", contentType)
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if resp["error"] != "Failed to create task" {
		t.Fatalf("expected error message 'Failed to create task', got %s", resp["error"])
	}

	expectedDetails := "failed to get/create session: " + rootErr.Error()
	if resp["details"] != expectedDetails {
		t.Fatalf("expected details %q, got %q", expectedDetails, resp["details"])
	}
}

func TestSnapshotHandlers(t *testing.T) {
	sessionStore := filestore.New(t.TempDir())
	stateStore := sessionstate.NewInMemoryStore()
	broadcaster := app.NewEventBroadcaster()
	taskStore := app.NewInMemoryTaskStore()
	reader := &staticJournalReader{entries: []journal.TurnJournalEntry{{SessionID: "sess-1", TurnID: 1, Summary: "rehydrate"}}}
	coordinator := app.NewServerCoordinator(
		&stubAgentCoordinator{},
		broadcaster,
		sessionStore,
		taskStore,
		stateStore,
		app.WithJournalReader(reader),
	)
	handler := NewAPIHandler(coordinator, app.NewHealthChecker(), false)

	snapshot := sessionstate.Snapshot{
		SessionID:  "sess-1",
		TurnID:     1,
		LLMTurnSeq: 1,
		CreatedAt:  time.Now().UTC(),
		Summary:    "observed",
		Messages:   []agentPorts.Message{{Role: "system", Content: "hello"}},
	}
	if err := stateStore.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("failed to seed snapshot: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/sess-1/snapshots", nil)
	resp := httptest.NewRecorder()
	handler.HandleListSnapshots(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", resp.Code)
	}
	var list SessionSnapshotsResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &list); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].TurnID != 1 {
		t.Fatalf("unexpected snapshot list payload: %+v", list)
	}

	turnReq := httptest.NewRequest(http.MethodGet, "/api/sessions/sess-1/turns/1", nil)
	turnResp := httptest.NewRecorder()
	handler.HandleGetTurnSnapshot(turnResp, turnReq)
	if turnResp.Code != http.StatusOK {
		t.Fatalf("expected 200 for turn snapshot, got %d", turnResp.Code)
	}
	var turn TurnSnapshotResponse
	if err := json.Unmarshal(turnResp.Body.Bytes(), &turn); err != nil {
		t.Fatalf("failed to decode turn response: %v", err)
	}
	if turn.SessionID != "sess-1" || turn.TurnID != 1 {
		t.Fatalf("unexpected turn payload: %+v", turn)
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/api/sessions/sess-1/replay", nil)
	replayResp := httptest.NewRecorder()
	handler.HandleReplaySession(replayResp, replayReq)
	if replayResp.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for replay, got %d", replayResp.Code)
	}
}

type staticJournalReader struct {
	entries []journal.TurnJournalEntry
}

func (r *staticJournalReader) Stream(_ context.Context, sessionID string, fn func(journal.TurnJournalEntry) error) error {
	for _, entry := range r.entries {
		e := entry
		if e.SessionID == "" {
			e.SessionID = sessionID
		}
		if err := fn(e); err != nil {
			return err
		}
	}
	return nil
}

func TestHandleGetContextSnapshotsSanitizesDuplicateAttachments(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	coordinator := app.NewServerCoordinator(
		&stubAgentCoordinator{},
		broadcaster,
		filestore.New(t.TempDir()),
		app.NewInMemoryTaskStore(),
		sessionstate.NewInMemoryStore(),
	)
	handler := NewAPIHandler(coordinator, app.NewHealthChecker(), true)

	attachments := map[string]agentPorts.Attachment{
		"preview.png": {
			Name:      "preview.png",
			MediaType: "image/png",
			Data:      "iVBORw0KGgo=",
			URI:       "https://cdn.example/preview.png",
		},
		"notes.txt": {
			Name:      "notes.txt",
			MediaType: "text/plain",
			Data:      "hello",
		},
	}

	message := agentPorts.Message{
		Role:        "assistant",
		Content:     "see [preview.png]",
		Attachments: attachments,
	}

	broadcaster.OnEvent(domain.NewWorkflowDiagnosticContextSnapshotEvent(
		agentPorts.LevelCore,
		"sess-ctx",
		"task-1",
		"",
		1,
		1,
		"req-1",
		[]agentPorts.Message{message},
		nil,
		time.Now(),
	))

	broadcaster.OnEvent(domain.NewWorkflowDiagnosticContextSnapshotEvent(
		agentPorts.LevelCore,
		"sess-ctx",
		"task-1",
		"",
		2,
		2,
		"req-2",
		[]agentPorts.Message{message},
		nil,
		time.Now().Add(time.Second),
	))

	req := httptest.NewRequest(http.MethodGet, "/api/internal/sessions/sess-ctx/context", nil)
	resp := httptest.NewRecorder()
	handler.HandleGetContextSnapshots(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var body ContextSnapshotResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(body.Snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(body.Snapshots))
	}

	first := body.Snapshots[0]
	if len(first.Messages) != 1 {
		t.Fatalf("expected 1 message in first snapshot, got %d", len(first.Messages))
	}
	firstAttachments := first.Messages[0].Attachments
	if len(firstAttachments) != 2 {
		t.Fatalf("expected 2 attachments in first snapshot, got %d", len(firstAttachments))
	}
	if firstAttachments["preview.png"].Data != "iVBORw0KGgo=" {
		t.Fatalf("expected image data to remain, got %q", firstAttachments["preview.png"].Data)
	}
	if firstAttachments["notes.txt"].Data != "hello" {
		t.Fatalf("expected text attachment data to remain, got %q", firstAttachments["notes.txt"].Data)
	}

	second := body.Snapshots[1]
	if len(second.Messages) != 1 {
		t.Fatalf("expected 1 message in second snapshot, got %d", len(second.Messages))
	}
	if second.Messages[0].Attachments != nil {
		t.Fatalf("expected duplicate attachments to be omitted, got %v", second.Messages[0].Attachments)
	}
}

func TestHandleWebVitalsAcceptsPayload(t *testing.T) {
	handler := NewAPIHandler(nil, app.NewHealthChecker(), false)
	req := httptest.NewRequest(http.MethodPost, "/api/metrics/web-vitals", strings.NewReader(`{"name":"CLS","value":0.1,"page":"/sessions/123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	handler.HandleWebVitals(resp, req)
	if resp.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.Code)
	}
}

func TestHandleWebVitalsRejectsBadMethod(t *testing.T) {
	handler := NewAPIHandler(nil, app.NewHealthChecker(), false)
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/web-vitals", nil)
	resp := httptest.NewRecorder()
	handler.HandleWebVitals(resp, req)
	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.Code)
	}
}

func (r *staticJournalReader) ReadAll(_ context.Context, sessionID string) ([]journal.TurnJournalEntry, error) {
	entries := make([]journal.TurnJournalEntry, len(r.entries))
	copy(entries, r.entries)
	for i := range entries {
		if entries[i].SessionID == "" {
			entries[i].SessionID = sessionID
		}
	}
	return entries, nil
}
