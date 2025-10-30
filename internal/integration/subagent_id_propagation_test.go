package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"sync"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/observability"
	builtin "alex/internal/tools/builtin"
	id "alex/internal/utils/id"
)

type recordingCoordinator struct {
	mu           sync.Mutex
	lastIDs      id.IDs
	logBuffer    *bytes.Buffer
	logger       *observability.Logger
	receivedTask string
}

func newRecordingCoordinator(buf *bytes.Buffer) *recordingCoordinator {
	logger := observability.NewLogger(observability.LogConfig{Level: "debug", Format: "json", Output: buf})
	return &recordingCoordinator{logBuffer: buf, logger: logger}
}

func (r *recordingCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener ports.EventListener) (*ports.TaskResult, error) {
	ids := id.IDsFromContext(ctx)

	r.mu.Lock()
	r.lastIDs = ids
	r.receivedTask = task
	r.mu.Unlock()

	r.logger.InfoContext(ctx, "executing subtask", "task", task)

	return &ports.TaskResult{
		Answer:       "subtask complete",
		Iterations:   1,
		TokensUsed:   42,
		SessionID:    ids.SessionID,
		TaskID:       id.TaskIDFromContext(ctx),
		ParentTaskID: id.ParentTaskIDFromContext(ctx),
	}, nil
}

func (r *recordingCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	return nil, nil
}

func (r *recordingCoordinator) SaveSessionAfterExecution(ctx context.Context, session *ports.Session, result *ports.TaskResult) error {
	return nil
}

func (r *recordingCoordinator) ListSessions(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (r *recordingCoordinator) GetConfig() ports.AgentConfig {
	return ports.AgentConfig{}
}

func (r *recordingCoordinator) GetLLMClient() (ports.LLMClient, error) {
	return nil, nil
}

func (r *recordingCoordinator) GetToolRegistryWithoutSubagent() ports.ToolRegistry {
	return nil
}

func (r *recordingCoordinator) GetParser() ports.FunctionCallParser {
	return nil
}

func (r *recordingCoordinator) GetContextManager() ports.ContextManager {
	return nil
}

func (r *recordingCoordinator) GetSystemPrompt() string {
	return ""
}

func (r *recordingCoordinator) LastIDs() id.IDs {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastIDs
}

func (r *recordingCoordinator) LoggedEntries() ([]map[string]any, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	decoder := json.NewDecoder(bytes.NewReader(r.logBuffer.Bytes()))
	var entries []map[string]any
	for decoder.More() {
		var entry map[string]any
		if err := decoder.Decode(&entry); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func TestSubagentDelegationPropagatesIdentifiers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration-style identifier propagation test in short mode")
	}

	buf := &bytes.Buffer{}
	coordinator := newRecordingCoordinator(buf)
	tool := builtin.NewSubAgent(coordinator, 1)

	sessionID := "session-root-123"
	rootTaskID := "task-root-456"
	ancestorTaskID := "task-ancestor-789"

	ctx := id.WithIDs(context.Background(), id.IDs{SessionID: sessionID, TaskID: rootTaskID, ParentTaskID: ancestorTaskID})

	call := ports.ToolCall{
		ID:           "call-1",
		Name:         "subagent",
		Arguments:    map[string]any{"subtasks": []any{"document the integration"}, "mode": "serial"},
		SessionID:    sessionID,
		TaskID:       rootTaskID,
		ParentTaskID: ancestorTaskID,
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("subagent execution failed: %v", err)
	}
	if result == nil {
		t.Fatalf("expected tool result, got nil")
	}

	if result.SessionID != sessionID {
		t.Fatalf("expected tool result session %s, got %s", sessionID, result.SessionID)
	}
	if result.TaskID != rootTaskID {
		t.Fatalf("expected tool result task %s, got %s", rootTaskID, result.TaskID)
	}
	if result.ParentTaskID != ancestorTaskID {
		t.Fatalf("expected tool result parent %s, got %s", ancestorTaskID, result.ParentTaskID)
	}

	ids := coordinator.LastIDs()
	if ids.SessionID != sessionID {
		t.Fatalf("expected delegated session %s, got %s", sessionID, ids.SessionID)
	}
	if ids.ParentTaskID != rootTaskID {
		t.Fatalf("expected delegated parent task %s, got %s", rootTaskID, ids.ParentTaskID)
	}
	if ids.TaskID == "" {
		t.Fatal("expected delegated task id to be set")
	}
	if ids.TaskID == rootTaskID {
		t.Fatalf("expected delegated task id to differ from root (%s)", rootTaskID)
	}

	entries, err := coordinator.LoggedEntries()
	if err != nil {
		t.Fatalf("failed to decode log entries: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one log entry")
	}

	entry := entries[len(entries)-1]
	if entry["session_id"] != sessionID {
		t.Fatalf("expected log session_id %s, got %v", sessionID, entry["session_id"])
	}
	if entry["task_id"] != ids.TaskID {
		t.Fatalf("expected log task_id %s, got %v", ids.TaskID, entry["task_id"])
	}
	if entry["parent_task_id"] != rootTaskID {
		t.Fatalf("expected log parent_task_id %s, got %v", rootTaskID, entry["parent_task_id"])
	}
}
