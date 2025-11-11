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

	agentPorts "alex/internal/agent/ports"
	"alex/internal/server/app"
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

func TestHandleCreateTaskReturnsJSONErrorOnSessionDecodeFailure(t *testing.T) {
	rootErr := errors.New("json: cannot unmarshal object into Go struct field ToolResult.messages.tool_results.error of type error")
	coordinator := app.NewServerCoordinator(&failingAgentCoordinator{err: rootErr}, app.NewEventBroadcaster(), nil, nil)
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
