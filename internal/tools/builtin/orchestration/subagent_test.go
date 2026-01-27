package orchestration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	llm "alex/internal/agent/ports/llm"
	storage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	id "alex/internal/utils/id"
	"alex/internal/workflow"
)

type stubEvent struct{}

func (stubEvent) EventType() string               { return "workflow.tool.completed" }
func (stubEvent) Timestamp() time.Time            { return time.Unix(0, 0) }
func (stubEvent) GetAgentLevel() agent.AgentLevel { return agent.AgentLevel("subagent") }
func (stubEvent) GetSessionID() string            { return "session" }
func (stubEvent) GetTaskID() string               { return "task" }
func (stubEvent) GetParentTaskID() string         { return "parent" }

type stubCoreEvent struct{}

func (stubCoreEvent) EventType() string               { return "workflow.node.output.delta" }
func (stubCoreEvent) Timestamp() time.Time            { return time.Unix(0, 0) }
func (stubCoreEvent) GetAgentLevel() agent.AgentLevel { return agent.LevelCore }
func (stubCoreEvent) GetSessionID() string            { return "session" }
func (stubCoreEvent) GetTaskID() string               { return "task" }
func (stubCoreEvent) GetParentTaskID() string         { return "parent" }

func TestSubtaskEventEventTypeMatchesOriginal(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubEvent{}}

	if got := evt.EventType(); got != "workflow.tool.completed" {
		t.Fatalf("expected event type to match original event, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelDefaultsToSubagent(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubCoreEvent{}}

	if got := evt.GetAgentLevel(); got != agent.LevelSubagent {
		t.Fatalf("expected core events to be elevated to subagent, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelRespectsExistingLevel(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubEvent{}}

	if got := evt.GetAgentLevel(); got != agent.AgentLevel("subagent") {
		t.Fatalf("expected non-core agent level to be preserved, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelNilOriginal(t *testing.T) {
	evt := &SubtaskEvent{}

	if got := evt.GetAgentLevel(); got != agent.LevelSubagent {
		t.Fatalf("expected nil original events to default to subagent level, got %q", got)
	}
}

type sessionIDRecorder struct {
	mu        sync.Mutex
	sessionID string
	ctxIDs    id.IDs
}

func (r *sessionIDRecorder) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessionID = sessionID
	r.ctxIDs = id.IDsFromContext(ctx)
	return &agent.TaskResult{}, nil
}

func (r *sessionIDRecorder) PrepareExecution(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	return nil, nil
}

func (r *sessionIDRecorder) SaveSessionAfterExecution(ctx context.Context, _ *storage.Session, _ *agent.TaskResult) error {
	return nil
}

func (r *sessionIDRecorder) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return nil, nil
}
func (r *sessionIDRecorder) GetConfig() agent.AgentConfig           { return agent.AgentConfig{} }
func (r *sessionIDRecorder) GetLLMClient() (llm.LLMClient, error) { return nil, nil }
func (r *sessionIDRecorder) GetToolRegistryWithoutSubagent() tools.ToolRegistry {
	return nil
}
func (r *sessionIDRecorder) GetParser() tools.FunctionCallParser     { return nil }
func (r *sessionIDRecorder) GetContextManager() agent.ContextManager { return nil }
func (r *sessionIDRecorder) GetSystemPrompt() string                 { return "" }

type workflowRecordingCoordinator struct {
	mu    sync.Mutex
	tasks []string
}

func (w *workflowRecordingCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	idx := len(w.tasks)
	w.tasks = append(w.tasks, task)

	snapshot := &workflow.WorkflowSnapshot{
		ID:    fmt.Sprintf("wf-%d", idx+1),
		Phase: workflow.PhaseSucceeded,
		Order: []string{"node"},
		Nodes: []workflow.NodeSnapshot{{ID: "node", Status: workflow.NodeStatusSucceeded}},
	}

	return &agent.TaskResult{
		Answer:     fmt.Sprintf("result-%d", idx+1),
		Iterations: idx + 1,
		TokensUsed: (idx + 1) * 10,
		Workflow:   snapshot,
	}, nil
}

func (*workflowRecordingCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	return nil, nil
}

func (*workflowRecordingCoordinator) SaveSessionAfterExecution(ctx context.Context, _ *storage.Session, _ *agent.TaskResult) error {
	return nil
}

func (*workflowRecordingCoordinator) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return nil, nil
}
func (*workflowRecordingCoordinator) GetConfig() agent.AgentConfig                       { return agent.AgentConfig{} }
func (*workflowRecordingCoordinator) GetLLMClient() (llm.LLMClient, error)             { return nil, nil }
func (*workflowRecordingCoordinator) GetToolRegistryWithoutSubagent() tools.ToolRegistry { return nil }
func (*workflowRecordingCoordinator) GetParser() tools.FunctionCallParser                { return nil }
func (*workflowRecordingCoordinator) GetContextManager() agent.ContextManager            { return nil }
func (*workflowRecordingCoordinator) GetSystemPrompt() string                            { return "" }

func TestSubagentUsesParentSessionID(t *testing.T) {
	recorder := &sessionIDRecorder{}
	tool := NewSubAgent(recorder, 1)

	sessionID := "session-root"
	ctx := id.WithIDs(context.Background(), id.IDs{SessionID: sessionID, TaskID: "task-root"})

	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "subagent",
		Arguments: map[string]any{"prompt": "capture session"},
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

func TestSubagentExposesWorkflowMetadata(t *testing.T) {
	coordinator := &workflowRecordingCoordinator{}
	tool := NewSubAgent(coordinator, 1)

	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "subagent",
		Arguments: map[string]any{"prompt": "task a\n- task b"},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("subagent execute failed: %v", err)
	}

	structured, ok := result.Metadata["results_struct"].([]subtaskMetadata)
	if !ok {
		t.Fatalf("expected structured results metadata to be available, got %T", result.Metadata["results_struct"])
	}

	if len(structured) != 1 {
		t.Fatalf("expected one structured result, got %d", len(structured))
	}

	entry := structured[0]
	if entry.Workflow == nil {
		t.Fatalf("expected workflow snapshot for result")
	}
	if entry.Workflow.Phase != workflow.PhaseSucceeded {
		t.Fatalf("unexpected workflow phase %s for result", entry.Workflow.Phase)
	}

	workflows, ok := result.Metadata["workflows"].([]*workflow.WorkflowSnapshot)
	if !ok {
		t.Fatalf("expected workflows metadata to be available, got %T", result.Metadata["workflows"])
	}

	if len(workflows) != len(structured) {
		t.Fatalf("expected %d workflows, got %d", len(structured), len(workflows))
	}
}

func TestSubtaskPreviewIsUTF8Safe(t *testing.T) {
	task := strings.Repeat("测试", 40)
	listener := newSubtaskListener(0, 1, task, nil, 1)

	if !utf8.ValidString(listener.taskPreview) {
		t.Fatalf("expected preview to be valid UTF-8, got %q", listener.taskPreview)
	}

	if len([]rune(task)) > 60 {
		if !strings.HasSuffix(listener.taskPreview, "...") {
			t.Fatalf("expected preview to be truncated with ellipsis, got %q", listener.taskPreview)
		}
		if got := len([]rune(listener.taskPreview)); got != 60 {
			t.Fatalf("expected preview to be 60 runes, got %d", got)
		}
	}
}
