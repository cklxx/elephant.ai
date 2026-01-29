package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	llm "alex/internal/agent/ports/llm"
	storage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/observability"
	"alex/internal/tools/builtin/orchestration"
	id "alex/internal/utils/id"
)

type recordingCoordinator struct {
	mu            sync.Mutex
	lastIDs       id.IDs
	logBuffer     *bytes.Buffer
	logger        *observability.Logger
	receivedTask  string
	inheritedAtt  map[string]ports.Attachment
	inheritedIter map[string]int
}

func newRecordingCoordinator(buf *bytes.Buffer) *recordingCoordinator {
	logger := observability.NewLogger(observability.LogConfig{Level: "debug", Format: "json", Output: buf})
	return &recordingCoordinator{logBuffer: buf, logger: logger}
}

func (r *recordingCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	ids := id.IDsFromContext(ctx)
	inheritedAtt, inheritedIter := appcontext.GetInheritedAttachments(ctx)

	r.mu.Lock()
	r.lastIDs = ids
	r.receivedTask = task
	r.inheritedAtt = inheritedAtt
	r.inheritedIter = inheritedIter
	r.mu.Unlock()

	r.logger.InfoContext(ctx, "executing subtask", "task", task)

	return &agent.TaskResult{
		Answer:       "subtask complete",
		Iterations:   1,
		TokensUsed:   42,
		SessionID:    ids.SessionID,
		RunID:        id.RunIDFromContext(ctx),
		ParentRunID:  id.ParentRunIDFromContext(ctx),
	}, nil
}

func (r *recordingCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	return nil, nil
}

func (r *recordingCoordinator) SaveSessionAfterExecution(ctx context.Context, _ *storage.Session, _ *agent.TaskResult) error {
	return nil
}

func (r *recordingCoordinator) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return nil, nil
}

func (r *recordingCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (r *recordingCoordinator) GetLLMClient() (llm.LLMClient, error) {
	return nil, nil
}

func (r *recordingCoordinator) GetToolRegistryWithoutSubagent() tools.ToolRegistry {
	return nil
}

func (r *recordingCoordinator) GetParser() tools.FunctionCallParser {
	return nil
}

func (r *recordingCoordinator) GetContextManager() agent.ContextManager {
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

func (r *recordingCoordinator) InheritedAttachments() (map[string]ports.Attachment, map[string]int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.inheritedAtt, r.inheritedIter
}

func TestSubagentDelegationPropagatesIdentifiers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration-style identifier propagation test in short mode")
	}

	buf := &bytes.Buffer{}
	coordinator := newRecordingCoordinator(buf)
	tool := orchestration.NewSubAgent(coordinator, 1)

	sessionID := "session-root-123"
	rootTaskID := "task-root-456"
	ancestorTaskID := "task-ancestor-789"
	logID := "log-root-000"

	ctx := id.WithIDs(context.Background(), id.IDs{SessionID: sessionID, RunID: rootTaskID, ParentRunID: ancestorTaskID, LogID: logID})

	call := ports.ToolCall{
		ID:           "call-1",
		Name:         "subagent",
		Arguments:    map[string]any{"prompt": "document the integration"},
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
	if ids.ParentRunID != rootTaskID {
		t.Fatalf("expected delegated parent task %s, got %s", rootTaskID, ids.ParentRunID)
	}
	if ids.RunID == "" {
		t.Fatal("expected delegated task id to be set")
	}
	if ids.RunID == rootTaskID {
		t.Fatalf("expected delegated task id to differ from root (%s)", rootTaskID)
	}
	if ids.LogID == "" {
		t.Fatalf("expected delegated log id to be set")
	}
	if !strings.Contains(ids.LogID, logID) {
		t.Fatalf("expected delegated log id to include %s, got %s", logID, ids.LogID)
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
	if entry["run_id"] != ids.RunID {
		t.Fatalf("expected log run_id %s, got %v", ids.RunID, entry["run_id"])
	}
	if entry["parent_run_id"] != rootTaskID {
		t.Fatalf("expected log parent_run_id %s, got %v", rootTaskID, entry["parent_run_id"])
	}
	if entry["log_id"] != ids.LogID {
		t.Fatalf("expected log log_id %s, got %v", ids.LogID, entry["log_id"])
	}
	if !strings.Contains(ids.LogID, logID) {
		t.Fatalf("expected log log_id to include %s, got %v", logID, entry["log_id"])
	}
}

func TestSubagentPropagatesAttachmentsToCoordinator(t *testing.T) {
	buf := &bytes.Buffer{}
	coordinator := newRecordingCoordinator(buf)
	tool := orchestration.NewSubAgent(coordinator, 1)

	placeholder := "doubao-seedream-3-0_nonce_0.png"
	attachments := map[string]ports.Attachment{
		placeholder: {
			Name:      placeholder,
			MediaType: "image/png",
			Data:      "YmFzZTY0X2RhdGE=",
			Source:    "seedream",
			URI:       "",
		},
	}
	iterations := map[string]int{placeholder: 7}

	ctx := context.Background()
	ctx = tools.WithAttachmentContext(ctx, attachments, iterations)
	ctx = id.WithIDs(ctx, id.IDs{SessionID: "session-1", RunID: "root-task"})

	call := ports.ToolCall{
		ID:        "call-attachments",
		Name:      "subagent",
		Arguments: map[string]any{"prompt": "refine the generated art"},
		SessionID: "session-1",
		TaskID:    "root-task",
	}

	if _, err := tool.Execute(ctx, call); err != nil {
		t.Fatalf("subagent execution failed: %v", err)
	}

	gotAtt, gotIter := coordinator.InheritedAttachments()
	if len(gotAtt) != 1 {
		t.Fatalf("expected 1 inherited attachment, got %d", len(gotAtt))
	}
	got, ok := gotAtt[placeholder]
	if !ok {
		t.Fatalf("missing propagated attachment %q", placeholder)
	}
	if got.Data != attachments[placeholder].Data {
		t.Fatalf("expected attachment data to match, got %q", got.Data)
	}
	iter, ok := gotIter[placeholder]
	if !ok {
		t.Fatalf("missing iteration entry for %q", placeholder)
	}
	if iter != 7 {
		t.Fatalf("expected iteration 7, got %d", iter)
	}
}
