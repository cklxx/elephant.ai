package builtin

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/agent/ports"
	id "alex/internal/utils/id"
)

type stubEvent struct{}

func (stubEvent) EventType() string               { return "tool_call_complete" }
func (stubEvent) Timestamp() time.Time            { return time.Unix(0, 0) }
func (stubEvent) GetAgentLevel() ports.AgentLevel { return ports.AgentLevel("subagent") }
func (stubEvent) GetSessionID() string            { return "session" }
func (stubEvent) GetTaskID() string               { return "task" }
func (stubEvent) GetParentTaskID() string         { return "parent" }

type stubCoreEvent struct{}

func (stubCoreEvent) EventType() string               { return "assistant_message" }
func (stubCoreEvent) Timestamp() time.Time            { return time.Unix(0, 0) }
func (stubCoreEvent) GetAgentLevel() ports.AgentLevel { return ports.LevelCore }
func (stubCoreEvent) GetSessionID() string            { return "session" }
func (stubCoreEvent) GetTaskID() string               { return "task" }
func (stubCoreEvent) GetParentTaskID() string         { return "parent" }

func TestSubtaskEventEventTypeMatchesOriginal(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubEvent{}}

	if got := evt.EventType(); got != "tool_call_complete" {
		t.Fatalf("expected event type to match original event, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelDefaultsToSubagent(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubCoreEvent{}}

	if got := evt.GetAgentLevel(); got != ports.LevelSubagent {
		t.Fatalf("expected core events to be elevated to subagent, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelRespectsExistingLevel(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubEvent{}}

	if got := evt.GetAgentLevel(); got != ports.AgentLevel("subagent") {
		t.Fatalf("expected non-core agent level to be preserved, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelNilOriginal(t *testing.T) {
	evt := &SubtaskEvent{}

	if got := evt.GetAgentLevel(); got != ports.LevelSubagent {
		t.Fatalf("expected nil original events to default to subagent level, got %q", got)
	}
}

type sessionIDRecorder struct {
	mu        sync.Mutex
	sessionID string
	ctxIDs    id.IDs
}

func (r *sessionIDRecorder) ExecuteTask(ctx context.Context, task string, sessionID string, listener ports.EventListener) (*ports.TaskResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionID = sessionID
	r.ctxIDs = id.IDsFromContext(ctx)
	return &ports.TaskResult{}, nil
}

func (r *sessionIDRecorder) PrepareExecution(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	return nil, nil
}

func (r *sessionIDRecorder) SaveSessionAfterExecution(ctx context.Context, session *ports.Session, result *ports.TaskResult) error {
	return nil
}

func (r *sessionIDRecorder) ListSessions(ctx context.Context) ([]string, error) { return nil, nil }
func (r *sessionIDRecorder) GetConfig() ports.AgentConfig                       { return ports.AgentConfig{} }
func (r *sessionIDRecorder) GetLLMClient() (ports.LLMClient, error)             { return nil, nil }
func (r *sessionIDRecorder) GetToolRegistryWithoutSubagent() ports.ToolRegistry {
	return nil
}
func (r *sessionIDRecorder) GetParser() ports.FunctionCallParser     { return nil }
func (r *sessionIDRecorder) GetContextManager() ports.ContextManager { return nil }
func (r *sessionIDRecorder) GetSystemPrompt() string                 { return "" }

func TestSubagentUsesParentSessionID(t *testing.T) {
	recorder := &sessionIDRecorder{}
	tool := NewSubAgent(recorder, 1)

	sessionID := "session-root"
	ctx := id.WithIDs(context.Background(), id.IDs{SessionID: sessionID, TaskID: "task-root"})

	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "subagent",
		Arguments: map[string]any{"subtasks": []any{"capture session"}},
	}

	if _, err := tool.Execute(ctx, call); err != nil {
		t.Fatalf("subagent execute failed: %v", err)
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	if recorder.sessionID != sessionID {
		t.Fatalf("expected coordinator to receive session id %s, got %s", sessionID, recorder.sessionID)
	}

	if recorder.ctxIDs.SessionID != sessionID {
		t.Fatalf("expected context to retain session id %s, got %s", sessionID, recorder.ctxIDs.SessionID)
	}
}
