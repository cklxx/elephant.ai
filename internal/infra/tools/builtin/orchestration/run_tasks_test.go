package orchestration

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/teamruntime"
)

type mockBGDispatcher struct {
	mu         sync.Mutex
	dispatched []agent.BackgroundDispatchRequest
}

func (m *mockBGDispatcher) Dispatch(_ context.Context, req agent.BackgroundDispatchRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatched = append(m.dispatched, req)
	return nil
}

func (m *mockBGDispatcher) Status(ids []string) []agent.BackgroundTaskSummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []agent.BackgroundTaskSummary
	for _, req := range m.dispatched {
		out = append(out, agent.BackgroundTaskSummary{
			ID:     req.TaskID,
			Status: agent.BackgroundTaskStatusCompleted,
		})
	}
	return out
}

func (m *mockBGDispatcher) Collect(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
	var out []agent.BackgroundTaskResult
	for _, id := range ids {
		out = append(out, agent.BackgroundTaskResult{
			ID:     id,
			Status: agent.BackgroundTaskStatusCompleted,
			Answer: "done",
		})
	}
	return out
}

func TestRunTasks_FileMode(t *testing.T) {
	// Create a temp task file.
	dir := t.TempDir()
	taskFilePath := filepath.Join(dir, "tasks.yaml")
	content := `
version: "1"
plan_id: "test-plan"
tasks:
  - id: "task-1"
    description: "first task"
    prompt: "do something"
    agent_type: "internal"
  - id: "task-2"
    description: "second task"
    prompt: "do something else"
    agent_type: "internal"
    depends_on: ["task-1"]
`
	if err := os.WriteFile(taskFilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file": taskFilePath,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %s", result.Content)
	}

	mock.mu.Lock()
	count := len(mock.dispatched)
	mock.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 dispatched tasks, got %d", count)
	}
}

func TestRunTasks_MissingFileAndTemplate(t *testing.T) {
	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-1",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error for missing file/template")
	}
	if !strings.Contains(result.Content, "exactly one of file or template is required") {
		t.Fatalf("unexpected error content: %s", result.Content)
	}
}

func TestRunTasks_FileAndTemplateMutuallyExclusive(t *testing.T) {
	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file":     "tasks.yaml",
			"template": "growth-team",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when both file and template are provided")
	}
	if !strings.Contains(result.Content, "mutually exclusive") {
		t.Fatalf("unexpected error content: %s", result.Content)
	}
}

func TestRunTasks_NoDispatcher(t *testing.T) {
	tool := NewRunTasks()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file": "/nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error == nil {
		t.Error("expected error when no dispatcher")
	}
}

func TestRunTasks_WaitMode(t *testing.T) {
	dir := t.TempDir()
	taskFilePath := filepath.Join(dir, "tasks.yaml")
	content := `
version: "1"
plan_id: "wait-plan"
tasks:
  - id: "task-1"
    prompt: "do it"
`
	if err := os.WriteFile(taskFilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file": taskFilePath,
			"wait": true,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %s", result.Content)
	}
}

func TestRunTasks_TaskIDFilter(t *testing.T) {
	dir := t.TempDir()
	taskFilePath := filepath.Join(dir, "tasks.yaml")
	content := `
version: "1"
plan_id: "filter-plan"
tasks:
  - id: "a"
    prompt: "do A"
  - id: "b"
    prompt: "do B"
  - id: "c"
    prompt: "do C"
`
	if err := os.WriteFile(taskFilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file":     taskFilePath,
			"task_ids": []any{"a", "c"},
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %s", result.Content)
	}

	mock.mu.Lock()
	count := len(mock.dispatched)
	mock.mu.Unlock()

	if count != 2 {
		t.Errorf("expected 2 filtered tasks dispatched, got %d", count)
	}
}

type mockTeamRunRecorder struct {
	mu      sync.Mutex
	records []agent.TeamRunRecord
}

func (m *mockTeamRunRecorder) RecordTeamRun(_ context.Context, record agent.TeamRunRecord) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, record)
	return "rec-1", nil
}

func TestRunTasks_TemplateRecordsTeamRun(t *testing.T) {
	mock := &mockBGDispatcher{}
	recorder := &mockTeamRunRecorder{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)
	ctx = agent.WithTeamRunRecorder(ctx, recorder)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{
		{
			Name:        "test-team",
			Description: "A test team",
			Roles: []agent.TeamRoleDefinition{
				{Name: "worker-a", AgentType: "kimi", PromptTemplate: "Do A: {GOAL}"},
				{Name: "worker-b", AgentType: "codex", PromptTemplate: "Do B: {GOAL}"},
			},
			Stages: []agent.TeamStageDefinition{
				{Name: "parallel", Roles: []string{"worker-a", "worker-b"}},
			},
		},
	})

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"template": "test-team",
			"goal":     "validate recorder",
			"wait":     true,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %s", result.Content)
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.records) != 1 {
		t.Fatalf("expected 1 recorded team run, got %d", len(recorder.records))
	}
	rec := recorder.records[0]
	if rec.TeamName != "test-team" {
		t.Errorf("expected team name test-team, got %s", rec.TeamName)
	}
	if rec.Goal != "validate recorder" {
		t.Errorf("expected goal 'validate recorder', got %s", rec.Goal)
	}
	if len(rec.Roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(rec.Roles))
	}
	if rec.DispatchState != "completed" {
		t.Errorf("expected dispatch state 'completed', got %s", rec.DispatchState)
	}
	if len(rec.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(rec.Stages))
	}
	if rec.Stages[0].Name != "parallel" {
		t.Errorf("expected stage name 'parallel', got %s", rec.Stages[0].Name)
	}
}

func TestRunTasks_FileMode_NoRecorderCall(t *testing.T) {
	dir := t.TempDir()
	taskFilePath := filepath.Join(dir, "tasks.yaml")
	content := `
version: "1"
plan_id: "test-plan"
tasks:
  - id: "task-1"
    prompt: "do something"
`
	if err := os.WriteFile(taskFilePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write task file: %v", err)
	}

	mock := &mockBGDispatcher{}
	recorder := &mockTeamRunRecorder{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)
	ctx = agent.WithTeamRunRecorder(ctx, recorder)

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"file": taskFilePath,
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %s", result.Content)
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.records) != 0 {
		t.Fatalf("expected no recorded team runs for file mode, got %d", len(recorder.records))
	}
}

func TestFilterTasks_CleansDependsOn(t *testing.T) {
	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "test",
		Tasks: []taskfile.TaskSpec{
			{ID: "a", Prompt: "A"},
			{ID: "b", Prompt: "B", DependsOn: []string{"a"}},
			{ID: "c", Prompt: "C", DependsOn: []string{"a", "b"}},
		},
	}
	// Filter to only "a" and "c" — "c" should lose its "b" dependency.
	filtered := filterTasks(tf, []string{"a", "c"})
	if len(filtered.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(filtered.Tasks))
	}
	for _, task := range filtered.Tasks {
		if task.ID == "c" {
			if len(task.DependsOn) != 1 || task.DependsOn[0] != "a" {
				t.Fatalf("expected c.DependsOn=[a], got %v", task.DependsOn)
			}
		}
	}
}

func TestDispatchStateFromStatus_UnknownOnError(t *testing.T) {
	state := dispatchStateFromStatus("/nonexistent/path.yaml")
	if state != "unknown" {
		t.Fatalf("expected 'unknown' for unreadable status file, got %q", state)
	}
}

func TestRunTasks_TemplateList(t *testing.T) {
	mock := &mockBGDispatcher{}
	ctx := agent.WithBackgroundDispatcher(context.Background(), mock)
	ctx = agent.WithTeamDefinitions(ctx, []agent.TeamDefinition{
		{
			Name:        "test-team",
			Description: "A test team",
			Roles:       []agent.TeamRoleDefinition{{Name: "worker", AgentType: "codex"}},
			Stages:      []agent.TeamStageDefinition{{Name: "do", Roles: []string{"worker"}}},
		},
	})

	tool := NewRunTasks()
	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"template": "list",
		},
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected error: %s", result.Content)
	}
	if result.Content == "" {
		t.Error("expected non-empty template listing")
	}
}

func TestExtractRoleIDFromTaskID(t *testing.T) {
	tests := []struct {
		taskID string
		want   string
	}{
		{taskID: "team-planner", want: "planner"},
		{taskID: "team-planner-debate", want: "planner"},
		{taskID: "team-planner-retry-2", want: "planner"},
		{taskID: "other", want: ""},
	}
	for _, tc := range tests {
		if got := extractRoleIDFromTaskID(tc.taskID); got != tc.want {
			t.Fatalf("extractRoleIDFromTaskID(%q)=%q want=%q", tc.taskID, got, tc.want)
		}
	}
}

func TestApplyBootstrapToTaskFile_InjectsRoleRuntimeConfig(t *testing.T) {
	tf := &taskfile.TaskFile{
		Version: "1",
		PlanID:  "team-plan",
		Tasks: []taskfile.TaskSpec{
			{ID: "team-planner", AgentType: "internal", Prompt: "p"},
			{ID: "team-executor", AgentType: "internal", Prompt: "e"},
		},
	}
	bootstrap := &teamruntime.EnsureResult{
		BaseDir:      "/tmp/team-runtime",
		EventLogPath: "/tmp/team-runtime/events.jsonl",
		Bootstrap: teamruntime.BootstrapState{
			TeamID:      "team-1",
			TmuxSession: "elephant-team-team-1",
		},
		RoleBindings: map[string]teamruntime.RoleBinding{
			"planner": {
				RoleID:            "planner",
				CapabilityProfile: "planning",
				TargetCLI:         "claude_code",
				SelectedCLI:       "claude_code",
				SelectedPath:      "/usr/local/bin/claude",
				SelectedAgentType: "claude_code",
				FallbackCLIs:      []string{"codex"},
				TmuxPane:          "%11",
				RoleLogPath:       "/tmp/team-runtime/logs/planner.log",
			},
		},
	}

	applyBootstrapToTaskFile(tf, bootstrap)

	planner := tf.Tasks[0]
	if planner.AgentType != "claude_code" {
		t.Fatalf("expected planner agent_type=claude_code, got %q", planner.AgentType)
	}
	if planner.Config["team_id"] != "team-1" {
		t.Fatalf("expected team_id injected, got %+v", planner.Config)
	}
	if planner.Config["binary"] != "/usr/local/bin/claude" {
		t.Fatalf("expected binary injected, got %+v", planner.Config)
	}
	if planner.Config["tmux_pane"] != "%11" {
		t.Fatalf("expected tmux pane injected, got %+v", planner.Config)
	}
	if planner.Config["fallback_clis"] != "codex" {
		t.Fatalf("expected fallback_clis injected, got %+v", planner.Config)
	}

	executor := tf.Tasks[1]
	if executor.Config != nil && executor.Config["team_id"] != "" {
		t.Fatalf("expected non-bound role untouched, got %+v", executor.Config)
	}
}
