package lark

import (
	"context"
	"testing"
	"time"
)

func TestTaskLocalStore_FilePersistsAcrossReload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewTaskFileStore(dir, time.Hour, 50)
	if err != nil {
		t.Fatalf("NewTaskFileStore() error = %v", err)
	}
	if err := store.SaveTask(ctx, TaskRecord{
		TaskID:      "task-1",
		ChatID:      "chat-1",
		Description: "hello",
		Status:      "pending",
		CreatedAt:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveTask() error = %v", err)
	}
	if err := store.UpdateStatus(ctx, "task-1", "completed", WithAnswerPreview("ok")); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	reloaded, err := NewTaskFileStore(dir, time.Hour, 50)
	if err != nil {
		t.Fatalf("reload NewTaskFileStore() error = %v", err)
	}
	got, ok, err := reloaded.GetTask(ctx, "task-1")
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected task to exist after reload")
	}
	if got.Status != "completed" || got.AnswerPreview != "ok" {
		t.Fatalf("unexpected task after reload: %+v", got)
	}
}

func TestTaskLocalStore_MaxTasksPerChat_KeepsActiveTasks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewTaskMemoryStore(time.Hour, 2)
	now := time.Now().UTC()

	for i := 0; i < 3; i++ {
		taskID := "task-" + string(rune('a'+i))
		if err := store.SaveTask(ctx, TaskRecord{
			TaskID:    taskID,
			ChatID:    "chat-1",
			Status:    "pending",
			CreatedAt: now.Add(time.Duration(i) * time.Second),
		}); err != nil {
			t.Fatalf("SaveTask(%d) error = %v", i, err)
		}
	}

	list, err := store.ListByChat(ctx, "chat-1", false, 10)
	if err != nil {
		t.Fatalf("ListByChat() error = %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected active tasks to be retained even over cap, got %d", len(list))
	}
	if list[0].TaskID != "task-c" || list[1].TaskID != "task-b" || list[2].TaskID != "task-a" {
		t.Fatalf("unexpected order after cap eviction: %+v", list)
	}
}

func TestTaskLocalStore_MaxTasksPerChat_TrimsTerminalTasks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewTaskMemoryStore(time.Hour, 2)
	now := time.Now().UTC()

	if err := store.SaveTask(ctx, TaskRecord{
		TaskID:      "task-a",
		ChatID:      "chat-1",
		Status:      "completed",
		CreatedAt:   now,
		CompletedAt: now,
	}); err != nil {
		t.Fatalf("SaveTask(task-a) error = %v", err)
	}
	if err := store.SaveTask(ctx, TaskRecord{
		TaskID:      "task-b",
		ChatID:      "chat-1",
		Status:      "failed",
		CreatedAt:   now.Add(1 * time.Second),
		CompletedAt: now.Add(1 * time.Second),
	}); err != nil {
		t.Fatalf("SaveTask(task-b) error = %v", err)
	}
	if err := store.SaveTask(ctx, TaskRecord{
		TaskID:    "task-c",
		ChatID:    "chat-1",
		Status:    "running",
		CreatedAt: now.Add(2 * time.Second),
	}); err != nil {
		t.Fatalf("SaveTask(task-c) error = %v", err)
	}

	list, err := store.ListByChat(ctx, "chat-1", false, 10)
	if err != nil {
		t.Fatalf("ListByChat() error = %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected one active + one newest terminal task after cap, got %d", len(list))
	}
	if list[0].TaskID != "task-c" || list[1].TaskID != "task-b" {
		t.Fatalf("unexpected tasks after cap eviction: %+v", list)
	}
	if _, ok, err := store.GetTask(ctx, "task-a"); err != nil {
		t.Fatalf("GetTask(task-a) error = %v", err)
	} else if ok {
		t.Fatalf("expected oldest terminal task to be evicted")
	}
}

func TestPlanReviewLocalStore_Expiry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewPlanReviewMemoryStore(10 * time.Millisecond)
	start := time.Now().UTC()
	store.now = func() time.Time { return start }

	if err := store.SavePending(ctx, PlanReviewPending{
		UserID:        "u1",
		ChatID:        "c1",
		RunID:         "r1",
		OverallGoalUI: "goal",
	}); err != nil {
		t.Fatalf("SavePending() error = %v", err)
	}

	store.now = func() time.Time { return start.Add(20 * time.Millisecond) }
	_, ok, err := store.GetPending(ctx, "u1", "c1")
	if err != nil {
		t.Fatalf("GetPending() error = %v", err)
	}
	if ok {
		t.Fatalf("expected pending to expire")
	}
}

func TestChatSessionBindingLocalStore_FilePersistsAcrossReload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewChatSessionBindingFileStore(dir)
	if err != nil {
		t.Fatalf("NewChatSessionBindingFileStore() error = %v", err)
	}
	if err := store.SaveBinding(ctx, ChatSessionBinding{
		Channel:   "lark",
		ChatID:    "oc_1",
		SessionID: "sess-1",
		UpdatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("SaveBinding() error = %v", err)
	}

	reloaded, err := NewChatSessionBindingFileStore(dir)
	if err != nil {
		t.Fatalf("reload NewChatSessionBindingFileStore() error = %v", err)
	}
	binding, ok, err := reloaded.GetBinding(ctx, "lark", "oc_1")
	if err != nil {
		t.Fatalf("GetBinding() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected binding to exist")
	}
	if binding.SessionID != "sess-1" {
		t.Fatalf("unexpected session id: %+v", binding)
	}
}
