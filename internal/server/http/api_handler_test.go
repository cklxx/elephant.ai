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

	agentPorts "alex/internal/agent/ports"
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
	coordinator := app.NewServerCoordinator(&stubAgentCoordinator{}, broadcaster, sessionStore, taskStore, stateStore)
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
