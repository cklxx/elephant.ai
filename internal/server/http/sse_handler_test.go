package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/types"
	"alex/internal/server/app"
)

func TestSSEHandler_MissingSessionID(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	req := httptest.NewRequest(http.MethodGet, "/api/sse", nil)
	rec := httptest.NewRecorder()

	handler.HandleSSEStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rec.Code)
	}
}

func TestSSEHandler_StreamingEvents(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id=test-session", nil)
	rec := httptest.NewRecorder()

	// Run SSE handler in goroutine
	go handler.HandleSSEStream(rec, req)

	// Wait for connection to establish
	time.Sleep(100 * time.Millisecond)

	// Broadcast an event
	event := domain.NewTaskAnalysisEvent(types.LevelCore, "Test Action", "Test Goal")
	broadcaster.OnEvent(event)

	// Wait for event to be sent
	time.Sleep(200 * time.Millisecond)

	// Check response
	body := rec.Body.String()

	// Should contain SSE format
	if !strings.Contains(body, "event: connected") {
		t.Error("Expected connected event in response")
	}

	// Check headers
	contentType := rec.Header().Get("Content-Type")
	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
	}

	cacheControl := rec.Header().Get("Cache-Control")
	if cacheControl != "no-cache" {
		t.Errorf("Expected Cache-Control no-cache, got %s", cacheControl)
	}
}

func TestSSEHandler_SerializeEvent(t *testing.T) {
	handler := NewSSEHandler(nil)

	tests := []struct {
		name      string
		event     domain.AgentEvent
		wantField string
	}{
		{
			name:      "TaskAnalysisEvent",
			event:     domain.NewTaskAnalysisEvent(types.LevelCore, "Test", "Goal"),
			wantField: "action_name",
		},
		{
			name: "ToolCallStartEvent",
			event: &domain.ToolCallStartEvent{
				BaseEvent: domain.BaseEvent{},
				CallID:    "test-call",
				ToolName:  "bash",
				Arguments: map[string]interface{}{"command": "ls"},
			},
			wantField: "tool_name",
		},
		{
			name: "TaskCompleteEvent",
			event: &domain.TaskCompleteEvent{
				BaseEvent:       domain.BaseEvent{},
				FinalAnswer:     "Done",
				TotalIterations: 3,
			},
			wantField: "final_answer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			json, err := handler.serializeEvent(tt.event)
			if err != nil {
				t.Fatalf("Failed to serialize event: %v", err)
			}

			if !strings.Contains(json, tt.wantField) {
				t.Errorf("Expected field %s in JSON, got: %s", tt.wantField, json)
			}

			// Check common fields
			if !strings.Contains(json, "event_type") {
				t.Error("Expected event_type in JSON")
			}
			if !strings.Contains(json, "timestamp") {
				t.Error("Expected timestamp in JSON")
			}
		})
	}
}
