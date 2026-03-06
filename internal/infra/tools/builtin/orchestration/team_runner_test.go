package orchestration

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

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
	for _, taskID := range ids {
		out = append(out, agent.BackgroundTaskResult{
			ID:     taskID,
			Status: agent.BackgroundTaskStatusCompleted,
			Answer: "done",
		})
	}
	return out
}

func TestTeamRunner_FileMode(t *testing.T) {
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
	runner := NewTeamRunner()
	result, err := runner.Run(context.Background(), RunRequest{
		Dispatcher: mock,
		FilePath:   taskFilePath,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result == nil || result.ExecuteResult == nil {
		t.Fatal("expected execute result")
	}

	mock.mu.Lock()
	count := len(mock.dispatched)
	mock.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 dispatched tasks, got %d", count)
	}
}

func TestTeamRunner_MissingFileAndTemplate(t *testing.T) {
	runner := NewTeamRunner()
	_, err := runner.Run(context.Background(), RunRequest{Dispatcher: &mockBGDispatcher{}})
	if err == nil || err.Error() != "exactly one of file or template is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTeamRunner_FileAndTemplateMutuallyExclusive(t *testing.T) {
	runner := NewTeamRunner()
	_, err := runner.Run(context.Background(), RunRequest{
		Dispatcher:   &mockBGDispatcher{},
		FilePath:     "tasks.yaml",
		TemplateName: "growth-team",
	})
	if err == nil || err.Error() != "file and template are mutually exclusive" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTeamRunner_NoDispatcher(t *testing.T) {
	runner := NewTeamRunner()
	_, err := runner.Run(context.Background(), RunRequest{FilePath: "/nonexistent"})
	if err == nil || err.Error() != "background task dispatch is not available in this context" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTeamRunner_WaitMode(t *testing.T) {
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

	runner := NewTeamRunner()
	result, err := runner.Run(context.Background(), RunRequest{
		Dispatcher: &mockBGDispatcher{},
		FilePath:   taskFilePath,
		Wait:       true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExecuteResult == nil {
		t.Fatal("expected execute result")
	}
}

func TestTeamRunner_TaskIDFilter(t *testing.T) {
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
	runner := NewTeamRunner()
	_, err := runner.Run(context.Background(), RunRequest{
		Dispatcher: mock,
		FilePath:   taskFilePath,
		TaskIDs:    []string{"a", "c"},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	mock.mu.Lock()
	count := len(mock.dispatched)
	mock.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 filtered tasks dispatched, got %d", count)
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

func TestTeamRunner_TemplateRecordsTeamRun(t *testing.T) {
	mock := &mockBGDispatcher{}
	recorder := &mockTeamRunRecorder{}
	ctx := agent.WithTeamRunRecorder(context.Background(), recorder)

	runner := NewTeamRunner()
	result, err := runner.Run(ctx, RunRequest{
		Dispatcher:   mock,
		TemplateName: "test-team",
		Goal:         "validate recorder",
		Wait:         true,
		TeamDefinitions: []agent.TeamDefinition{
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
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExecuteResult == nil {
		t.Fatal("expected execute result")
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.records) != 1 {
		t.Fatalf("expected 1 recorded team run, got %d", len(recorder.records))
	}
	rec := recorder.records[0]
	if rec.TeamName != "test-team" {
		t.Fatalf("expected team name test-team, got %s", rec.TeamName)
	}
	if rec.Goal != "validate recorder" {
		t.Fatalf("expected goal validate recorder, got %s", rec.Goal)
	}
	if rec.DispatchState != "completed" {
		t.Fatalf("expected dispatch state completed, got %s", rec.DispatchState)
	}
}

func TestTeamRunner_FileMode_NoRecorderCall(t *testing.T) {
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

	recorder := &mockTeamRunRecorder{}
	ctx := agent.WithTeamRunRecorder(context.Background(), recorder)
	runner := NewTeamRunner()
	_, err := runner.Run(ctx, RunRequest{
		Dispatcher: &mockBGDispatcher{},
		FilePath:   taskFilePath,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.records) != 0 {
		t.Fatalf("expected no recorded team runs for file mode, got %d", len(recorder.records))
	}
}

func TestTeamRunner_TemplateList(t *testing.T) {
	runner := NewTeamRunner()
	result, err := runner.Run(context.Background(), RunRequest{
		Dispatcher:   &mockBGDispatcher{},
		TemplateName: "list",
		TeamDefinitions: []agent.TeamDefinition{
			{
				Name:        "test-team",
				Description: "A test team",
				Roles:       []agent.TeamRoleDefinition{{Name: "worker", AgentType: "codex"}},
				Stages:      []agent.TeamStageDefinition{{Name: "do", Roles: []string{"worker"}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Content == "" {
		t.Fatal("expected non-empty template listing")
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

	meta := tf.Tasks[0].RuntimeMeta
	if meta.SelectedAgentType != "claude_code" {
		t.Fatalf("expected SelectedAgentType=claude_code, got %q", meta.SelectedAgentType)
	}
	if meta.TeamID != "team-1" {
		t.Fatalf("expected TeamID=team-1, got %q", meta.TeamID)
	}
	if meta.Binary != "/usr/local/bin/claude" {
		t.Fatalf("expected Binary=/usr/local/bin/claude, got %q", meta.Binary)
	}
	if tf.Tasks[1].RuntimeMeta.TeamID != "" {
		t.Fatalf("expected non-bound role untouched, got %+v", tf.Tasks[1].RuntimeMeta)
	}
}
