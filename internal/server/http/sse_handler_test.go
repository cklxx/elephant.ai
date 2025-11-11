package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agentports "alex/internal/agent/ports"
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

func TestSSEHandler_InvalidSessionID(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id=../../etc/passwd", nil)
	rec := httptest.NewRecorder()

	handler.HandleSSEStream(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	if !strings.Contains(rec.Body.String(), "invalid characters") {
		t.Fatalf("expected invalid characters error, got %q", rec.Body.String())
	}
}

func TestSSEHandler_StreamingEvents(t *testing.T) {
	broadcaster := app.NewEventBroadcaster()
	handler := NewSSEHandler(broadcaster)

	req := httptest.NewRequest(http.MethodGet, "/api/sse?session_id=test-session", nil)

	// Use a thread-safe buffer to capture output
	var mu sync.Mutex
	var buf bytes.Buffer

	// Create a custom ResponseWriter that is thread-safe
	writer := &threadSafeResponseWriter{
		mu:     &mu,
		buf:    &buf,
		header: make(http.Header),
	}

	// Run SSE handler in goroutine
	done := make(chan bool)
	go func() {
		handler.HandleSSEStream(writer, req)
		done <- true
	}()

	// Wait for connection to establish
	time.Sleep(100 * time.Millisecond)

	// Broadcast an event
	event := domain.NewTaskAnalysisEvent(types.LevelCore, "test-session", "sse-task", "", "Test Action", "Test Goal", time.Now())
	broadcaster.OnEvent(event)

	// Wait for event to be sent
	time.Sleep(200 * time.Millisecond)

	// Read response body safely
	mu.Lock()
	body := buf.String()
	mu.Unlock()

	// Should contain SSE format
	if !strings.Contains(body, "event: connected") {
		t.Error("Expected connected event in response")
	}

	if !strings.Contains(body, "parent_task_id") {
		t.Error("Expected connected event payload to include parent_task_id")
	}

	// Check headers (read safely after they're set)
	mu.Lock()
	contentType := writer.header.Get("Content-Type")
	cacheControl := writer.header.Get("Cache-Control")
	mu.Unlock()

	if contentType != "text/event-stream" {
		t.Errorf("Expected Content-Type text/event-stream, got %s", contentType)
	}

	if cacheControl != "no-cache" {
		t.Errorf("Expected Cache-Control no-cache, got %s", cacheControl)
	}
}

// threadSafeResponseWriter is a thread-safe ResponseWriter for testing
type threadSafeResponseWriter struct {
	mu     *sync.Mutex
	buf    *bytes.Buffer
	header http.Header
}

func (w *threadSafeResponseWriter) Header() http.Header {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.header
}

func (w *threadSafeResponseWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(b)
}

func (w *threadSafeResponseWriter) WriteHeader(statusCode int) {
	// Not needed for SSE
}

func (w *threadSafeResponseWriter) Flush() {
	// Implement Flusher interface
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
			event:     domain.NewTaskAnalysisEvent(types.LevelCore, "test-session", "sse-task", "", "Test", "Goal", time.Now()),
			wantField: "action_name",
		},
		{
			name: "ToolCallStartEvent",
			event: &domain.ToolCallStartEvent{
				BaseEvent: domain.BaseEvent{},
				CallID:    "test-call",
				ToolName:  "bash",
				Arguments: map[string]interface{}{
					"command": "ls",
					"apiKey":  "sk-secret",
					"nested": map[string]interface{}{
						"token": "abc",
					},
					"list": []interface{}{"Bearer token-value"},
				},
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
		{
			name: "TaskCancelledEvent",
			event: domain.NewTaskCancelledEvent(
				types.LevelCore,
				"test-session",
				"sse-task",
				"",
				"cancelled",
				"user",
				time.Now(),
			),
			wantField: "reason",
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

			if !strings.Contains(json, "\"task_id\"") {
				t.Errorf("Expected serialized event to include task_id, got: %s", json)
			}

			if !strings.Contains(json, "\"parent_task_id\"") {
				t.Errorf("Expected serialized event to include parent_task_id, got: %s", json)
			}

			if _, ok := tt.event.(*domain.ToolCallStartEvent); ok {
				if strings.Contains(json, "sk-secret") || strings.Contains(json, "Bearer token-value") {
					t.Errorf("Sensitive values should be redacted, got: %s", json)
				}

				if !strings.Contains(json, "arguments_preview") {
					t.Errorf("Expected arguments_preview in serialized start event, got: %s", json)
				}

				if !strings.Contains(json, "command") {
					t.Errorf("Expected sanitized command argument, got: %s", json)
				}
			}

			// Check common fields
			if !strings.Contains(json, "event_type") {
				t.Error("Expected event_type in JSON")
			}
			if !strings.Contains(json, "timestamp") {
				t.Error("Expected timestamp in JSON")
			}
			if !strings.Contains(json, "session_id") {
				t.Error("Expected session_id in JSON")
			}
		})
	}
}

func TestSSEHandler_SerializeEvent_ContextSnapshot(t *testing.T) {
	handler := NewSSEHandler(nil)
	now := time.Now()

	messages := []agentports.Message{
		{
			Role:    "system",
			Content: "You are a helpful assistant.",
			Source:  agentports.MessageSourceSystemPrompt,
		},
		{
			Role:       "tool",
			Content:    "Tool output",
			ToolCallID: "call-1",
			ToolResults: []agentports.ToolResult{
				{
					CallID:  "call-1",
					Content: "complete",
				},
			},
			Source: agentports.MessageSourceToolResult,
		},
	}

	excluded := []agentports.Message{
		{
			Role:    "system",
			Content: "Debug trace",
			Source:  agentports.MessageSourceDebug,
		},
	}

	event := domain.NewContextSnapshotEvent(
		types.LevelCore,
		"session-123",
		"task-456",
		"parent-789",
		2,
		"req-abc",
		messages,
		excluded,
		now,
	)

	payload, err := handler.serializeEvent(event)
	if err != nil {
		t.Fatalf("serializeEvent returned error: %v", err)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	if got := data["iteration"]; got != float64(2) {
		t.Fatalf("expected iteration 2, got %v", got)
	}

	if got := data["request_id"]; got != "req-abc" {
		t.Fatalf("expected request_id 'req-abc', got %v", got)
	}

	rawMessages, ok := data["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages field missing or wrong type: %T", data["messages"])
	}
	if len(rawMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(rawMessages))
	}

	first, ok := rawMessages[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first message type mismatch: %T", rawMessages[0])
	}
	if first["source"] != string(agentports.MessageSourceSystemPrompt) {
		t.Fatalf("expected first message source %q, got %v", agentports.MessageSourceSystemPrompt, first["source"])
	}

	second, ok := rawMessages[1].(map[string]interface{})
	if !ok {
		t.Fatalf("second message type mismatch: %T", rawMessages[1])
	}
	if _, ok := second["tool_results"].([]interface{}); !ok {
		t.Fatalf("expected tool_results array on second message, got %T", second["tool_results"])
	}

	excludedRaw, ok := data["excluded_messages"].([]interface{})
	if !ok || len(excludedRaw) != 1 {
		t.Fatalf("expected 1 excluded message, got %v", data["excluded_messages"])
	}

	excludedMsg, ok := excludedRaw[0].(map[string]interface{})
	if !ok {
		t.Fatalf("excluded message type mismatch: %T", excludedRaw[0])
	}
	if excludedMsg["source"] != string(agentports.MessageSourceDebug) {
		t.Fatalf("expected excluded message source %q, got %v", agentports.MessageSourceDebug, excludedMsg["source"])
	}
}

func TestSanitizeArguments(t *testing.T) {
	original := map[string]interface{}{
		"token":      "super-secret",
		"name":       "example",
		"apiKey":     "sk-example",
		"nested":     map[string]interface{}{"password": "p4ss"},
		"list":       []interface{}{"Bearer xyz", map[string]interface{}{"refresh_token": "abc"}},
		"whitespace": "   ",
	}

	sanitized := sanitizeArguments(original)

	if sanitized["token"] != redactedPlaceholder {
		t.Fatalf("expected token to be redacted, got %v", sanitized["token"])
	}

	if sanitized["apiKey"] != redactedPlaceholder {
		t.Fatalf("expected apiKey to be redacted, got %v", sanitized["apiKey"])
	}

	nested, ok := sanitized["nested"].(map[string]interface{})
	if !ok || nested["password"] != redactedPlaceholder {
		t.Fatalf("expected nested password to be redacted, got %v", sanitized["nested"])
	}

	list, ok := sanitized["list"].([]interface{})
	if !ok {
		t.Fatalf("expected list to remain a slice, got %T", sanitized["list"])
	}

	if list[0] != redactedPlaceholder {
		t.Fatalf("expected first list element to be redacted, got %v", list[0])
	}

	nestedList, ok := list[1].(map[string]interface{})
	if !ok || nestedList["refresh_token"] != redactedPlaceholder {
		t.Fatalf("expected nested refresh_token to be redacted, got %v", list[1])
	}

	if original["token"].(string) != "super-secret" {
		t.Fatalf("original map should not be mutated")
	}

	if sanitized["name"] != "example" {
		t.Fatalf("non-sensitive value changed unexpectedly: %v", sanitized["name"])
	}
}
