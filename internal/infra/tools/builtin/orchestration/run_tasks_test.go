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
	meta := planner.RuntimeMeta
	if meta.SelectedAgentType != "claude_code" {
		t.Fatalf("expected SelectedAgentType=claude_code, got %q", meta.SelectedAgentType)
	}
	if meta.TeamID != "team-1" {
		t.Fatalf("expected TeamID=team-1, got %q", meta.TeamID)
	}
	if meta.Binary != "/usr/local/bin/claude" {
		t.Fatalf("expected Binary=/usr/local/bin/claude, got %q", meta.Binary)
	}
	if meta.TmuxPane != "%11" {
		t.Fatalf("expected TmuxPane=%%11, got %q", meta.TmuxPane)
	}
	if len(meta.FallbackCLIs) != 1 || meta.FallbackCLIs[0] != "codex" {
		t.Fatalf("expected FallbackCLIs=[codex], got %v", meta.FallbackCLIs)
	}
	if meta.RoleLogPath != "/tmp/team-runtime/logs/planner.log" {
		t.Fatalf("expected RoleLogPath set, got %q", meta.RoleLogPath)
	}
	if meta.TmuxSession != "elephant-team-team-1" {
		t.Fatalf("expected TmuxSession set, got %q", meta.TmuxSession)
	}

	executor := tf.Tasks[1]
	if executor.RuntimeMeta.TeamID != "" {
		t.Fatalf("expected non-bound role untouched, got %+v", executor.RuntimeMeta)
	}
}

func TestBootstrapTargetCLI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role agent.TeamRoleDefinition
		want string
	}{
		{
			name: "explicit target cli wins",
			role: agent.TeamRoleDefinition{
				AgentType: "kimi",
				TargetCLI: "codex",
			},
			want: "codex",
		},
		{
			name: "external agent type becomes target",
			role: agent.TeamRoleDefinition{
				AgentType: "kimi",
			},
			want: "kimi",
		},
		{
			name: "non-external role has no target",
			role: agent.TeamRoleDefinition{
				AgentType: "internal",
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := bootstrapTargetCLI(tt.role)
			if got != tt.want {
				t.Fatalf("bootstrapTargetCLI() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShouldBootstrapRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role agent.TeamRoleDefinition
		want bool
	}{
		{
			name: "external role bootstrapped",
			role: agent.TeamRoleDefinition{
				AgentType: "kimi",
			},
			want: true,
		},
		{
			name: "internal role without target skipped",
			role: agent.TeamRoleDefinition{
				AgentType: "internal",
			},
			want: false,
		},
		{
			name: "explicit target forces bootstrap",
			role: agent.TeamRoleDefinition{
				AgentType: "internal",
				TargetCLI: "codex",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := shouldBootstrapRole(tt.role)
			if got != tt.want {
				t.Fatalf("shouldBootstrapRole() = %v, want %v", got, tt.want)
			}
		})
	}
}
