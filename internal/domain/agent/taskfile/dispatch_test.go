package taskfile

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
)

type dispatchMockDispatcher struct {
	mu         sync.Mutex
	dispatched []agent.BackgroundDispatchRequest
}

func (m *dispatchMockDispatcher) Dispatch(_ context.Context, req agent.BackgroundDispatchRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dispatched = append(m.dispatched, req)
	return nil
}

func (m *dispatchMockDispatcher) Status(ids []string) []agent.BackgroundTaskSummary {
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

func (m *dispatchMockDispatcher) Collect(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
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

func (m *dispatchMockDispatcher) dispatchedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.dispatched)
}

func simpleTeamDef() *agent.TeamDefinition {
	return &agent.TeamDefinition{
		Name:        "test-team",
		Description: "A test team",
		Roles: []agent.TeamRoleDefinition{
			{Name: "worker-a", AgentType: "codex", PromptTemplate: "Do A: {GOAL}"},
			{Name: "worker-b", AgentType: "codex", PromptTemplate: "Do B: {GOAL}"},
		},
		Stages: []agent.TeamStageDefinition{
			{Name: "execute", Roles: []string{"worker-a", "worker-b"}},
		},
	}
}

func TestDispatchTeamRun_BasicFlow(t *testing.T) {
	mock := &dispatchMockDispatcher{}
	result, err := DispatchTeamRun(context.Background(), TeamRunRequest{
		Dispatcher:  mock,
		TeamDef:     simpleTeamDef(),
		Goal:        "build feature X",
		CausationID: "test-cause",
		StatusPath:  t.TempDir() + "/status.yaml",
		Mode:        ModeAuto,
		Wait:        true,
		Timeout:     10 * time.Second,
	})
	if err != nil {
		t.Fatalf("DispatchTeamRun: %v", err)
	}
	if result.PlanID != "team-test-team" {
		t.Errorf("PlanID: got %q, want team-test-team", result.PlanID)
	}
	if len(result.TaskIDs) != 2 {
		t.Errorf("expected 2 task IDs, got %d", len(result.TaskIDs))
	}
	if result.Record.TeamName != "test-team" {
		t.Errorf("record team name: got %q", result.Record.TeamName)
	}
	if result.Record.Goal != "build feature X" {
		t.Errorf("record goal: got %q", result.Record.Goal)
	}
	if len(result.Record.Roles) != 2 {
		t.Errorf("record roles: got %d", len(result.Record.Roles))
	}
}

func TestDispatchTeamRun_BootstrapCallback(t *testing.T) {
	mock := &dispatchMockDispatcher{}
	bootstrapCalled := false
	_, err := DispatchTeamRun(context.Background(), TeamRunRequest{
		Dispatcher:  mock,
		TeamDef:     simpleTeamDef(),
		Goal:        "test bootstrap",
		CausationID: "test-cause",
		StatusPath:  t.TempDir() + "/status.yaml",
		Wait:        true,
		Timeout:     10 * time.Second,
		BootstrapFn: func(ctx context.Context, tf *TaskFile) error {
			bootstrapCalled = true
			if len(tf.Tasks) != 2 {
				t.Errorf("bootstrap received %d tasks, expected 2", len(tf.Tasks))
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("DispatchTeamRun: %v", err)
	}
	if !bootstrapCalled {
		t.Error("bootstrap function was not called")
	}
}

func TestDispatchTeamRun_BootstrapError(t *testing.T) {
	mock := &dispatchMockDispatcher{}
	_, err := DispatchTeamRun(context.Background(), TeamRunRequest{
		Dispatcher:  mock,
		TeamDef:     simpleTeamDef(),
		Goal:        "test error",
		CausationID: "test-cause",
		StatusPath:  t.TempDir() + "/status.yaml",
		Wait:        true,
		Timeout:     10 * time.Second,
		BootstrapFn: func(ctx context.Context, tf *TaskFile) error {
			return fmt.Errorf("bootstrap failed")
		},
	})
	if err == nil {
		t.Fatal("expected error from bootstrap")
	}
	if mock.dispatchedCount() != 0 {
		t.Error("should not have dispatched any tasks after bootstrap error")
	}
}

func TestDispatchTeamRun_NilTeamDef(t *testing.T) {
	mock := &dispatchMockDispatcher{}
	_, err := DispatchTeamRun(context.Background(), TeamRunRequest{
		Dispatcher: mock,
		Goal:       "test",
	})
	if err == nil {
		t.Fatal("expected error for nil team def")
	}
}

func TestDispatchTeamRun_EmptyGoal(t *testing.T) {
	mock := &dispatchMockDispatcher{}
	_, err := DispatchTeamRun(context.Background(), TeamRunRequest{
		Dispatcher: mock,
		TeamDef:    simpleTeamDef(),
		Goal:       "",
	})
	if err == nil {
		t.Fatal("expected error for empty goal")
	}
}

func TestDispatchTeamRun_NoWaitMode(t *testing.T) {
	mock := &dispatchMockDispatcher{}
	result, err := DispatchTeamRun(context.Background(), TeamRunRequest{
		Dispatcher:  mock,
		TeamDef:     simpleTeamDef(),
		Goal:        "fire and forget",
		CausationID: "test-cause",
		StatusPath:  t.TempDir() + "/status.yaml",
		Wait:        false,
	})
	if err != nil {
		t.Fatalf("DispatchTeamRun: %v", err)
	}
	if result.Record.DispatchState != "dispatched" {
		t.Errorf("expected dispatch_state=dispatched, got %q", result.Record.DispatchState)
	}
}

func TestDispatchTeamRun_TaskIDFilter(t *testing.T) {
	mock := &dispatchMockDispatcher{}
	result, err := DispatchTeamRun(context.Background(), TeamRunRequest{
		Dispatcher:  mock,
		TeamDef:     simpleTeamDef(),
		Goal:        "filtered",
		CausationID: "test-cause",
		StatusPath:  t.TempDir() + "/status.yaml",
		Wait:        true,
		Timeout:     10 * time.Second,
		TaskIDs:     []string{"team-worker-a"},
	})
	if err != nil {
		t.Fatalf("DispatchTeamRun: %v", err)
	}
	if len(result.TaskIDs) != 1 {
		t.Errorf("expected 1 task after filter, got %d", len(result.TaskIDs))
	}
}

func TestDispatchStateFromStatus_UnknownOnError(t *testing.T) {
	state := DispatchStateFromStatus("/nonexistent/path.yaml")
	if state != "unknown" {
		t.Fatalf("expected 'unknown', got %q", state)
	}
}

func TestFilterTasks_CleansDeps(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "A"},
			{ID: "b", Prompt: "B", DependsOn: []string{"a"}},
			{ID: "c", Prompt: "C", DependsOn: []string{"a", "b"}},
		},
	}
	filtered := FilterTasks(tf, []string{"a", "c"})
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

func TestBuildTeamRunRecord_DoesNotOverrideInternalRoleAgentType(t *testing.T) {
	tf := &TaskFile{
		Version: "1",
		PlanID:  "team-mixed",
		Tasks: []TaskSpec{
			{
				ID:        "team-internal-reviewer",
				AgentType: "internal",
				RuntimeMeta: TeamRuntimeMeta{
					SelectedAgentType: "codex",
				},
			},
			{
				ID:        "team-coder",
				AgentType: "generic_cli",
				RuntimeMeta: TeamRuntimeMeta{
					SelectedAgentType: "kimi",
				},
			},
		},
	}
	result := &ExecuteResult{
		PlanID: "team-mixed",
	}

	record := BuildTeamRunRecord(tf, nil, "mixed-team", "goal", result, "", true)
	if len(record.Roles) != 2 {
		t.Fatalf("expected 2 role records, got %d", len(record.Roles))
	}

	got := map[string]string{}
	for _, role := range record.Roles {
		got[role.Name] = role.AgentType
	}
	if got["team-internal-reviewer"] != agent.AgentTypeInternal {
		t.Fatalf("internal role agent type: got %q, want %q", got["team-internal-reviewer"], agent.AgentTypeInternal)
	}
	if got["team-coder"] != agent.AgentTypeKimi {
		t.Fatalf("external role agent type: got %q, want %q", got["team-coder"], agent.AgentTypeKimi)
	}
}
