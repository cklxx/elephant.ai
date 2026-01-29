package orchestration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"alex/internal/agent/domain"
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
func (stubEvent) GetRunID() string                { return "task" }
func (stubEvent) GetParentRunID() string          { return "parent" }
func (stubEvent) GetCorrelationID() string        { return "" }
func (stubEvent) GetCausationID() string          { return "" }
func (stubEvent) GetEventID() string              { return "" }
func (stubEvent) GetSeq() uint64                  { return 0 }

type stubCoreEvent struct{}

func (stubCoreEvent) EventType() string               { return "workflow.node.output.delta" }
func (stubCoreEvent) Timestamp() time.Time            { return time.Unix(0, 0) }
func (stubCoreEvent) GetAgentLevel() agent.AgentLevel { return agent.LevelCore }
func (stubCoreEvent) GetSessionID() string            { return "session" }
func (stubCoreEvent) GetRunID() string                { return "task" }
func (stubCoreEvent) GetParentRunID() string          { return "parent" }
func (stubCoreEvent) GetCorrelationID() string        { return "" }
func (stubCoreEvent) GetCausationID() string          { return "" }
func (stubCoreEvent) GetEventID() string              { return "" }
func (stubCoreEvent) GetSeq() uint64                  { return 0 }

type attachmentEvent struct {
	attachments map[string]ports.Attachment
}

func (attachmentEvent) EventType() string               { return "workflow.tool.completed" }
func (attachmentEvent) Timestamp() time.Time            { return time.Unix(0, 0) }
func (attachmentEvent) GetAgentLevel() agent.AgentLevel { return agent.LevelSubagent }
func (attachmentEvent) GetSessionID() string            { return "session" }
func (attachmentEvent) GetRunID() string                { return "task" }
func (attachmentEvent) GetParentRunID() string          { return "parent" }
func (attachmentEvent) GetCorrelationID() string        { return "" }
func (attachmentEvent) GetCausationID() string          { return "" }
func (attachmentEvent) GetEventID() string              { return "" }
func (attachmentEvent) GetSeq() uint64                  { return 0 }
func (e attachmentEvent) GetAttachments() map[string]ports.Attachment {
	return e.attachments
}

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

type attachmentCoordinator struct {
	events []agent.AgentEvent
}

func (c *attachmentCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	for _, evt := range c.events {
		if listener != nil {
			listener.OnEvent(evt)
		}
	}
	return &agent.TaskResult{Answer: "done"}, nil
}

func (*attachmentCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	return nil, nil
}

func (*attachmentCoordinator) SaveSessionAfterExecution(ctx context.Context, _ *storage.Session, _ *agent.TaskResult) error {
	return nil
}

func (*attachmentCoordinator) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return nil, nil
}
func (*attachmentCoordinator) GetConfig() agent.AgentConfig                       { return agent.AgentConfig{} }
func (*attachmentCoordinator) GetLLMClient() (llm.LLMClient, error)             { return nil, nil }
func (*attachmentCoordinator) GetToolRegistryWithoutSubagent() tools.ToolRegistry { return nil }
func (*attachmentCoordinator) GetParser() tools.FunctionCallParser                { return nil }
func (*attachmentCoordinator) GetContextManager() agent.ContextManager            { return nil }
func (*attachmentCoordinator) GetSystemPrompt() string                            { return "" }

func TestSubagentUsesParentSessionID(t *testing.T) {
	recorder := &sessionIDRecorder{}
	tool := NewSubAgent(recorder, 1)

	sessionID := "session-root"
	ctx := id.WithIDs(context.Background(), id.IDs{SessionID: sessionID, RunID: "task-root"})

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

func TestSubagentCollectsGeneratedAttachments(t *testing.T) {
	inherited := map[string]ports.Attachment{
		"base.png": {
			Name:      "base.png",
			MediaType: "image/png",
			Data:      "YmFzZQ==",
			Source:    "seed",
		},
	}

	emitted := map[string]ports.Attachment{
		"base.png": {
			Name:      "base.png",
			MediaType: "image/png",
			Data:      "YmFzZQ==",
			Source:    "seed",
		},
		"report.md": {
			Name:      "report.md",
			MediaType: "text/markdown",
			Data:      "cmVwb3J0",
			Source:    "sandbox",
		},
	}

	coordinator := &attachmentCoordinator{
		events: []agent.AgentEvent{attachmentEvent{attachments: emitted}},
	}
	tool := NewSubAgent(coordinator, 1)

	ctx := tools.WithAttachmentContext(context.Background(), inherited, map[string]int{"base.png": 1})
	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "subagent",
		Arguments: map[string]any{"prompt": "summarize findings"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("subagent execute failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected tool result to be returned")
	}
	if len(result.Attachments) == 0 {
		t.Fatalf("expected attachments to be returned, got %#v", result.Attachments)
	}
	if _, ok := result.Attachments["base.png"]; ok {
		t.Fatalf("expected inherited attachment to be filtered, got base.png")
	}
	if _, ok := result.Attachments["report.md"]; !ok {
		t.Fatalf("expected generated attachment report.md to be returned, got %#v", result.Attachments)
	}
}

func TestSubtaskPreviewIsUTF8Safe(t *testing.T) {
	task := strings.Repeat("测试", 40)
	listener := newSubtaskListener(0, 1, task, nil, 1, nil)

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

func TestSubtaskListenerCapturesEnvelopeAttachments(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"report.md": {
			Name:      "report.md",
			MediaType: "text/markdown",
			Data:      "cmVwb3J0",
			Source:    "subagent",
		},
	}
	evt := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelSubagent, "session", "task", "parent", time.Now()),
		Event:     "workflow.tool.completed",
		Payload: map[string]any{
			"attachments": attachments,
		},
	}

	collector := newAttachmentCollector(nil)
	listener := newSubtaskListener(0, 1, "task", nil, 1, collector)
	listener.OnEvent(evt)

	got := collector.Snapshot()
	if len(got) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got))
	}
	if _, ok := got["report.md"]; !ok {
		t.Fatalf("expected report.md attachment, got %#v", got)
	}
}
