package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"alex/internal/delivery/channels/lark"
	"alex/internal/delivery/server/ports"
	taskdomain "alex/internal/domain/task"
	agent "alex/internal/domain/agent/ports/agent"
)

// ── Mock store ──────────────────────────────────────────────────────────────

type mockStore struct {
	tasks       map[string]*taskdomain.Task
	transitions map[string][]taskdomain.Transition
}

func newMockStore() *mockStore {
	return &mockStore{
		tasks:       make(map[string]*taskdomain.Task),
		transitions: make(map[string][]taskdomain.Transition),
	}
}

func (m *mockStore) EnsureSchema(context.Context) error { return nil }

func (m *mockStore) Create(_ context.Context, t *taskdomain.Task) error {
	if t.TaskID == "" {
		return fmt.Errorf("task_id required")
	}
	if _, exists := m.tasks[t.TaskID]; exists {
		return fmt.Errorf("task %s: already exists", t.TaskID)
	}
	copy := *t
	m.tasks[t.TaskID] = &copy
	return nil
}

func (m *mockStore) Get(_ context.Context, taskID string) (*taskdomain.Task, error) {
	t, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s: not found", taskID)
	}
	copy := *t
	return &copy, nil
}

func (m *mockStore) SetStatus(_ context.Context, taskID string, status taskdomain.Status, opts ...taskdomain.TransitionOption) error {
	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s: not found", taskID)
	}
	p := taskdomain.ApplyTransitionOptions(opts)
	t.Status = status
	now := time.Now()
	t.UpdatedAt = now
	if status.IsTerminal() {
		t.CompletedAt = &now
	}
	if status == taskdomain.StatusRunning && t.StartedAt == nil {
		t.StartedAt = &now
	}
	if p.AnswerPreview != nil {
		t.AnswerPreview = *p.AnswerPreview
	}
	if p.ErrorText != nil {
		t.Error = *p.ErrorText
	}
	if p.TokensUsed != nil {
		t.TokensUsed = *p.TokensUsed
	}
	return nil
}

func (m *mockStore) UpdateProgress(_ context.Context, taskID string, iteration int, tokensUsed int, costUSD float64) error {
	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s: not found", taskID)
	}
	t.CurrentIteration = iteration
	t.TokensUsed = tokensUsed
	t.CostUSD = costUSD
	return nil
}

func (m *mockStore) SetResult(_ context.Context, taskID string, answer string, resultJSON json.RawMessage, tokensUsed int) error {
	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s: not found", taskID)
	}
	t.Status = taskdomain.StatusCompleted
	t.AnswerPreview = answer
	if len(answer) > 500 {
		t.AnswerPreview = answer[:500]
	}
	t.ResultJSON = resultJSON
	t.TokensUsed = tokensUsed
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

func (m *mockStore) SetError(_ context.Context, taskID string, errText string) error {
	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s: not found", taskID)
	}
	t.Status = taskdomain.StatusFailed
	t.Error = errText
	now := time.Now()
	t.CompletedAt = &now
	return nil
}

func (m *mockStore) SetBridgeMeta(_ context.Context, taskID string, meta taskdomain.BridgeMeta) error {
	t, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s: not found", taskID)
	}
	t.BridgeMeta = &meta
	return nil
}

func (m *mockStore) ListBySession(_ context.Context, sessionID string, limit int) ([]*taskdomain.Task, error) {
	var tasks []*taskdomain.Task
	for _, t := range m.tasks {
		if t.SessionID == sessionID {
			copy := *t
			tasks = append(tasks, &copy)
		}
	}
	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks, nil
}

func (m *mockStore) ListByChat(_ context.Context, chatID string, activeOnly bool, limit int) ([]*taskdomain.Task, error) {
	var tasks []*taskdomain.Task
	for _, t := range m.tasks {
		if t.ChatID != chatID {
			continue
		}
		if activeOnly && t.Status.IsTerminal() {
			continue
		}
		copy := *t
		tasks = append(tasks, &copy)
	}
	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}
	return tasks, nil
}

func (m *mockStore) ListByStatus(_ context.Context, statuses ...taskdomain.Status) ([]*taskdomain.Task, error) {
	statusSet := make(map[taskdomain.Status]bool)
	for _, s := range statuses {
		statusSet[s] = true
	}
	var tasks []*taskdomain.Task
	for _, t := range m.tasks {
		if statusSet[t.Status] {
			copy := *t
			tasks = append(tasks, &copy)
		}
	}
	return tasks, nil
}

func (m *mockStore) ListActive(ctx context.Context) ([]*taskdomain.Task, error) {
	return m.ListByStatus(ctx, taskdomain.StatusPending, taskdomain.StatusRunning, taskdomain.StatusWaitingInput)
}

func (m *mockStore) List(_ context.Context, limit int, offset int) ([]*taskdomain.Task, int, error) {
	var tasks []*taskdomain.Task
	for _, t := range m.tasks {
		copy := *t
		tasks = append(tasks, &copy)
	}
	total := len(tasks)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return tasks[offset:end], total, nil
}

func (m *mockStore) Delete(_ context.Context, taskID string) error {
	if _, ok := m.tasks[taskID]; !ok {
		return fmt.Errorf("task %s: not found", taskID)
	}
	delete(m.tasks, taskID)
	return nil
}

func (m *mockStore) Transitions(_ context.Context, taskID string) ([]taskdomain.Transition, error) {
	return m.transitions[taskID], nil
}

func (m *mockStore) MarkStaleRunning(_ context.Context, reason string) error {
	for _, t := range m.tasks {
		if !t.Status.IsTerminal() {
			t.Status = taskdomain.StatusFailed
			t.Error = reason
			now := time.Now()
			t.CompletedAt = &now
		}
	}
	return nil
}

func (m *mockStore) DeleteExpired(_ context.Context, before time.Time) error {
	for id, t := range m.tasks {
		if t.CreatedAt.Before(before) {
			delete(m.tasks, id)
		}
	}
	return nil
}

// ── Server Adapter Tests ────────────────────────────────────────────────────

func TestServerAdapter_CreateAndGet(t *testing.T) {
	store := newMockStore()
	adapter := NewServerAdapter(store)
	ctx := context.Background()

	task, err := adapter.Create(ctx, "session-1", "test task", "default", "full")
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if task.ID == "" {
		t.Fatal("Create() returned task with empty ID")
	}
	if task.SessionID != "session-1" {
		t.Errorf("SessionID = %q, want %q", task.SessionID, "session-1")
	}
	if task.Status != ports.TaskStatusPending {
		t.Errorf("Status = %q, want %q", task.Status, ports.TaskStatusPending)
	}

	got, err := adapter.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Description != "test task" {
		t.Errorf("Description = %q, want %q", got.Description, "test task")
	}
}

func TestServerAdapter_SetStatus(t *testing.T) {
	store := newMockStore()
	adapter := NewServerAdapter(store)
	ctx := context.Background()

	task, _ := adapter.Create(ctx, "s1", "task", "", "")
	if err := adapter.SetStatus(ctx, task.ID, ports.TaskStatusRunning); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}

	got, _ := adapter.Get(ctx, task.ID)
	if got.Status != ports.TaskStatusRunning {
		t.Errorf("Status = %q, want %q", got.Status, ports.TaskStatusRunning)
	}
}

func TestServerAdapter_SetResult(t *testing.T) {
	store := newMockStore()
	adapter := NewServerAdapter(store)
	ctx := context.Background()

	task, _ := adapter.Create(ctx, "s1", "task", "", "")
	result := &agent.TaskResult{
		Answer:     "the answer",
		TokensUsed: 1000,
		Iterations: 5,
	}
	if err := adapter.SetResult(ctx, task.ID, result); err != nil {
		t.Fatalf("SetResult() error = %v", err)
	}

	got, _ := adapter.Get(ctx, task.ID)
	if got.Status != ports.TaskStatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, ports.TaskStatusCompleted)
	}
	if got.TokensUsed != 1000 {
		t.Errorf("TokensUsed = %d, want %d", got.TokensUsed, 1000)
	}
}

func TestServerAdapter_SetError(t *testing.T) {
	store := newMockStore()
	adapter := NewServerAdapter(store)
	ctx := context.Background()

	task, _ := adapter.Create(ctx, "s1", "task", "", "")
	if err := adapter.SetError(ctx, task.ID, errors.New("something broke")); err != nil {
		t.Fatalf("SetError() error = %v", err)
	}

	got, _ := adapter.Get(ctx, task.ID)
	if got.Status != ports.TaskStatusFailed {
		t.Errorf("Status = %q, want %q", got.Status, ports.TaskStatusFailed)
	}
	if got.Error != "something broke" {
		t.Errorf("Error = %q, want %q", got.Error, "something broke")
	}
}

func TestServerAdapter_ListBySession(t *testing.T) {
	store := newMockStore()
	adapter := NewServerAdapter(store)
	ctx := context.Background()

	adapter.Create(ctx, "s1", "task1", "", "")
	adapter.Create(ctx, "s1", "task2", "", "")
	adapter.Create(ctx, "s2", "task3", "", "")

	tasks, err := adapter.ListBySession(ctx, "s1")
	if err != nil {
		t.Fatalf("ListBySession() error = %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("got %d tasks, want 2", len(tasks))
	}
}

func TestServerAdapter_Delete(t *testing.T) {
	store := newMockStore()
	adapter := NewServerAdapter(store)
	ctx := context.Background()

	task, _ := adapter.Create(ctx, "s1", "task", "", "")
	if err := adapter.Delete(ctx, task.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := adapter.Get(ctx, task.ID)
	if err == nil {
		t.Fatal("Get() after Delete() should return error")
	}
}

func TestServerAdapter_NotFound(t *testing.T) {
	store := newMockStore()
	adapter := NewServerAdapter(store)
	ctx := context.Background()

	_, err := adapter.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Get() should return error for nonexistent task")
	}
}

// ── Lark Adapter Tests ──────────────────────────────────────────────────────

func TestLarkAdapter_SaveAndGet(t *testing.T) {
	store := newMockStore()
	adapter := NewLarkAdapter(store)
	ctx := context.Background()

	rec := lark.TaskRecord{
		TaskID:      "lark-task-1",
		ChatID:      "chat-123",
		UserID:      "user-456",
		AgentType:   "claude_code",
		Description: "fix the bug",
		Status:      "pending",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := adapter.SaveTask(ctx, rec); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}

	got, found, err := adapter.GetTask(ctx, "lark-task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !found {
		t.Fatal("GetTask() found = false, want true")
	}
	if got.ChatID != "chat-123" {
		t.Errorf("ChatID = %q, want %q", got.ChatID, "chat-123")
	}
	if got.AgentType != "claude_code" {
		t.Errorf("AgentType = %q, want %q", got.AgentType, "claude_code")
	}
}

func TestLarkAdapter_UpdateStatus(t *testing.T) {
	store := newMockStore()
	adapter := NewLarkAdapter(store)
	ctx := context.Background()

	rec := lark.TaskRecord{
		TaskID:    "lark-task-2",
		ChatID:    "chat-1",
		Status:    "pending",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	adapter.SaveTask(ctx, rec)

	err := adapter.UpdateStatus(ctx, "lark-task-2", "running")
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	got, _, _ := adapter.GetTask(ctx, "lark-task-2")
	if got.Status != "running" {
		t.Errorf("Status = %q, want %q", got.Status, "running")
	}
}

func TestLarkAdapter_UpdateStatusWithOptions(t *testing.T) {
	store := newMockStore()
	adapter := NewLarkAdapter(store)
	ctx := context.Background()

	rec := lark.TaskRecord{
		TaskID:    "lark-task-3",
		ChatID:    "chat-1",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	adapter.SaveTask(ctx, rec)

	err := adapter.UpdateStatus(ctx, "lark-task-3", "completed",
		lark.WithAnswerPreview("done!"),
		lark.WithTokensUsed(2000),
	)
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	got, _, _ := adapter.GetTask(ctx, "lark-task-3")
	if got.Status != "completed" {
		t.Errorf("Status = %q, want %q", got.Status, "completed")
	}
	if got.AnswerPreview != "done!" {
		t.Errorf("AnswerPreview = %q, want %q", got.AnswerPreview, "done!")
	}
	if got.TokensUsed != 2000 {
		t.Errorf("TokensUsed = %d, want %d", got.TokensUsed, 2000)
	}
}

func TestLarkAdapter_ListByChat(t *testing.T) {
	store := newMockStore()
	adapter := NewLarkAdapter(store)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		rec := lark.TaskRecord{
			TaskID:    fmt.Sprintf("task-%d", i),
			ChatID:    "chat-1",
			Status:    "running",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		adapter.SaveTask(ctx, rec)
	}

	tasks, err := adapter.ListByChat(ctx, "chat-1", false, 10)
	if err != nil {
		t.Fatalf("ListByChat() error = %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("got %d tasks, want 3", len(tasks))
	}
}

func TestLarkAdapter_MarkStaleRunning(t *testing.T) {
	store := newMockStore()
	adapter := NewLarkAdapter(store)
	ctx := context.Background()

	rec := lark.TaskRecord{
		TaskID:    "stale-1",
		ChatID:    "chat-1",
		Status:    "running",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	adapter.SaveTask(ctx, rec)

	if err := adapter.MarkStaleRunning(ctx, "gateway restart"); err != nil {
		t.Fatalf("MarkStaleRunning() error = %v", err)
	}

	got, _, _ := adapter.GetTask(ctx, "stale-1")
	if got.Status != "failed" {
		t.Errorf("Status = %q, want %q", got.Status, "failed")
	}
}

func TestLarkAdapter_GetNotFound(t *testing.T) {
	store := newMockStore()
	adapter := NewLarkAdapter(store)
	ctx := context.Background()

	_, found, err := adapter.GetTask(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetTask() unexpected error = %v", err)
	}
	if found {
		t.Fatal("GetTask() found = true, want false")
	}
}
