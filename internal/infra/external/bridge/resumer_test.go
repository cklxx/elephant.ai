package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	taskdomain "alex/internal/domain/task"
)

func TestClassifyOrphan_Running(t *testing.T) {
	t.Parallel()
	action := ClassifyOrphan(OrphanedBridge{IsRunning: true, HasDone: false}, nil)
	if action != ResumeAdopt {
		t.Errorf("expected adopt, got %s", action)
	}
}

func TestClassifyOrphan_Done(t *testing.T) {
	t.Parallel()
	action := ClassifyOrphan(OrphanedBridge{IsRunning: false, HasDone: true}, nil)
	if action != ResumeHarvest {
		t.Errorf("expected harvest, got %s", action)
	}
}

func TestClassifyOrphan_DeadWithFiles(t *testing.T) {
	t.Parallel()
	task := &taskdomain.Task{
		Prompt: "test",
		BridgeMeta: &taskdomain.BridgeMeta{
			FilesTouched: []string{"/a.go"},
		},
	}
	action := ClassifyOrphan(OrphanedBridge{IsRunning: false, HasDone: false}, task)
	if action != ResumeRetryWithContext {
		t.Errorf("expected retry_with_context, got %s", action)
	}
}

func TestClassifyOrphan_DeadNoProgress(t *testing.T) {
	t.Parallel()
	task := &taskdomain.Task{Prompt: "test"}
	action := ClassifyOrphan(OrphanedBridge{IsRunning: false, HasDone: false}, task)
	if action != ResumeRetryFresh {
		t.Errorf("expected retry_fresh, got %s", action)
	}
}

func TestClassifyOrphan_DeadNoTask(t *testing.T) {
	t.Parallel()
	action := ClassifyOrphan(OrphanedBridge{IsRunning: false, HasDone: false}, nil)
	if action != ResumeMarkFailed {
		t.Errorf("expected mark_failed, got %s", action)
	}
}

func TestBuildResumePrompt_NoMeta(t *testing.T) {
	t.Parallel()
	task := &taskdomain.Task{Prompt: "fix the bug"}
	prompt := buildResumePrompt(task)
	if prompt != "fix the bug" {
		t.Errorf("expected original prompt, got %q", prompt)
	}
}

func TestBuildResumePrompt_WithMeta(t *testing.T) {
	t.Parallel()
	task := &taskdomain.Task{
		Prompt: "fix the bug",
		BridgeMeta: &taskdomain.BridgeMeta{
			LastIteration: 5,
			FilesTouched:  []string{"main.go", "handler.go"},
			TokensUsed:    15000,
		},
	}
	prompt := buildResumePrompt(task)

	if prompt == "fix the bug" {
		t.Error("expected enriched prompt")
	}
	for _, want := range []string{
		"[Resume Context]",
		"iteration 5",
		"main.go",
		"handler.go",
		"15000",
		"[Original Task]",
		"fix the bug",
	} {
		if !contains(prompt, want) {
			t.Errorf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBuildResumePrompt_Nil(t *testing.T) {
	t.Parallel()
	prompt := buildResumePrompt(nil)
	if prompt != "" {
		t.Errorf("expected empty, got %q", prompt)
	}
}

func TestResumer_HarvestOrphan(t *testing.T) {
	dir := t.TempDir()
	taskID := "harvest-test"

	// Create bridge output dir with completed task.
	taskDir := filepath.Join(dir, ".elephant", "bridge", taskID)
	_ = os.MkdirAll(taskDir, 0o755)

	events := `{"type":"tool","tool_name":"Bash","summary":"command=ls","files":[],"iter":1}
{"type":"result","answer":"harvested answer","tokens":500,"cost":0.01,"iters":1,"is_error":false}
`
	_ = os.WriteFile(filepath.Join(taskDir, "output.jsonl"), []byte(events), 0o644)
	_ = os.WriteFile(filepath.Join(taskDir, ".done"), nil, 0o644)

	store := &mockTaskStore{
		tasks: map[string]*taskdomain.Task{
			taskID: {
				TaskID: taskID,
				Status: taskdomain.StatusRunning,
				Prompt: "test",
			},
		},
	}

	resumer := NewResumer(store, New(BridgeConfig{}), nil)
	results := resumer.ResumeOrphans(context.Background(), dir)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ResumeHarvest {
		t.Errorf("action = %s, want harvest", results[0].Action)
	}
	if results[0].Error != nil {
		t.Errorf("unexpected error: %v", results[0].Error)
	}

	// Verify task was updated.
	task, _ := store.Get(context.Background(), taskID)
	if task.Status != taskdomain.StatusCompleted {
		t.Errorf("status = %s, want completed", task.Status)
	}
	if task.AnswerPreview != "harvested answer" {
		t.Errorf("answer = %q, want 'harvested answer'", task.AnswerPreview)
	}

	// Verify bridge dir was cleaned up.
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Error("expected bridge dir to be cleaned up")
	}
}

func TestResumer_MarkFailedOrphan(t *testing.T) {
	dir := t.TempDir()
	taskID := "failed-test"

	// Create bridge output dir without .done (process died).
	taskDir := filepath.Join(dir, ".elephant", "bridge", taskID)
	_ = os.MkdirAll(taskDir, 0o755)
	_ = os.WriteFile(filepath.Join(taskDir, "output.jsonl"), []byte("{}"), 0o644)

	store := &mockTaskStore{
		tasks: map[string]*taskdomain.Task{
			taskID: {
				TaskID: taskID,
				Status: taskdomain.StatusRunning,
			},
		},
	}

	resumer := NewResumer(store, New(BridgeConfig{}), nil)
	results := resumer.ResumeOrphans(context.Background(), dir)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Action != ResumeMarkFailed {
		t.Errorf("action = %s, want mark_failed", results[0].Action)
	}

	task, _ := store.Get(context.Background(), taskID)
	if task.Status != taskdomain.StatusFailed {
		t.Errorf("status = %s, want failed", task.Status)
	}
}

func TestResumer_SkipsTerminalTasks(t *testing.T) {
	dir := t.TempDir()
	taskID := "terminal-test"

	taskDir := filepath.Join(dir, ".elephant", "bridge", taskID)
	_ = os.MkdirAll(taskDir, 0o755)
	_ = os.WriteFile(filepath.Join(taskDir, "output.jsonl"), []byte("{}"), 0o644)
	_ = os.WriteFile(filepath.Join(taskDir, ".done"), nil, 0o644)

	store := &mockTaskStore{
		tasks: map[string]*taskdomain.Task{
			taskID: {
				TaskID: taskID,
				Status: taskdomain.StatusCompleted, // Already terminal.
			},
		},
	}

	resumer := NewResumer(store, New(BridgeConfig{}), nil)
	results := resumer.ResumeOrphans(context.Background(), dir)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should be harvest (skip) since already terminal.
	if results[0].Action != ResumeHarvest {
		t.Errorf("action = %s, want harvest", results[0].Action)
	}

	// Bridge dir should be cleaned up.
	if _, err := os.Stat(taskDir); !os.IsNotExist(err) {
		t.Error("expected bridge dir to be cleaned up")
	}
}

func TestResumer_NoOrphans(t *testing.T) {
	dir := t.TempDir()
	store := &mockTaskStore{tasks: make(map[string]*taskdomain.Task)}
	resumer := NewResumer(store, New(BridgeConfig{}), nil)
	results := resumer.ResumeOrphans(context.Background(), dir)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// contains is a helper for substring checks.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockTaskStore is a simple in-memory implementation for testing.
type mockTaskStore struct {
	tasks       map[string]*taskdomain.Task
	transitions []taskdomain.Transition
}

func (m *mockTaskStore) EnsureSchema(ctx context.Context) error { return nil }

func (m *mockTaskStore) Create(ctx context.Context, task *taskdomain.Task) error {
	m.tasks[task.TaskID] = task
	return nil
}

func (m *mockTaskStore) Get(ctx context.Context, taskID string) (*taskdomain.Task, error) {
	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	copy := *task
	return &copy, nil
}

func (m *mockTaskStore) SetStatus(ctx context.Context, taskID string, status taskdomain.Status, opts ...taskdomain.TransitionOption) error {
	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Status = status
	return nil
}

func (m *mockTaskStore) UpdateProgress(ctx context.Context, taskID string, iteration int, tokensUsed int, costUSD float64) error {
	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.CurrentIteration = iteration
	task.TokensUsed = tokensUsed
	task.CostUSD = costUSD
	return nil
}

func (m *mockTaskStore) SetResult(ctx context.Context, taskID string, answer string, resultJSON json.RawMessage, tokensUsed int) error {
	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.AnswerPreview = answer
	task.ResultJSON = resultJSON
	task.TokensUsed = tokensUsed
	return nil
}

func (m *mockTaskStore) SetError(ctx context.Context, taskID string, errText string) error {
	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.Error = errText
	return nil
}

func (m *mockTaskStore) SetBridgeMeta(ctx context.Context, taskID string, meta taskdomain.BridgeMeta) error {
	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}
	task.BridgeMeta = &meta
	return nil
}

func (m *mockTaskStore) ListBySession(ctx context.Context, sessionID string, limit int) ([]*taskdomain.Task, error) {
	return nil, nil
}

func (m *mockTaskStore) ListByChat(ctx context.Context, chatID string, activeOnly bool, limit int) ([]*taskdomain.Task, error) {
	return nil, nil
}

func (m *mockTaskStore) ListByStatus(ctx context.Context, statuses ...taskdomain.Status) ([]*taskdomain.Task, error) {
	var result []*taskdomain.Task
	for _, task := range m.tasks {
		for _, s := range statuses {
			if task.Status == s {
				copy := *task
				result = append(result, &copy)
				break
			}
		}
	}
	return result, nil
}

func (m *mockTaskStore) ListActive(ctx context.Context) ([]*taskdomain.Task, error) {
	return m.ListByStatus(ctx, taskdomain.StatusPending, taskdomain.StatusRunning, taskdomain.StatusWaitingInput)
}

func (m *mockTaskStore) List(ctx context.Context, limit int, offset int) ([]*taskdomain.Task, int, error) {
	return nil, 0, nil
}

func (m *mockTaskStore) Delete(ctx context.Context, taskID string) error {
	delete(m.tasks, taskID)
	return nil
}

func (m *mockTaskStore) Transitions(ctx context.Context, taskID string) ([]taskdomain.Transition, error) {
	return m.transitions, nil
}

func (m *mockTaskStore) MarkStaleRunning(ctx context.Context, reason string) error { return nil }

func (m *mockTaskStore) DeleteExpired(ctx context.Context, before time.Time) error { return nil }
