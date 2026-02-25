package lark

import (
	"context"
	"testing"
	"time"
)

// --- isTerminalTaskStatus ---

func TestIsTerminalTaskStatus_Terminal(t *testing.T) {
	for _, status := range []string{"completed", "failed", "cancelled"} {
		if !isTerminalTaskStatus(status) {
			t.Fatalf("expected %q to be terminal", status)
		}
	}
}

func TestIsTerminalTaskStatus_CaseInsensitive(t *testing.T) {
	if !isTerminalTaskStatus("COMPLETED") {
		t.Fatal("expected case-insensitive match")
	}
	if !isTerminalTaskStatus("  Failed  ") {
		t.Fatal("expected trimmed match")
	}
}

func TestIsTerminalTaskStatus_NonTerminal(t *testing.T) {
	for _, status := range []string{"pending", "running", "waiting_input", ""} {
		if isTerminalTaskStatus(status) {
			t.Fatalf("expected %q to not be terminal", status)
		}
	}
}

// --- SaveTask ---

func TestSaveTask_ValidTask(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	err := store.SaveTask(context.Background(), TaskRecord{
		TaskID: "t1", ChatID: "c1",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rec, ok, _ := store.GetTask(context.Background(), "t1")
	if !ok {
		t.Fatal("expected task found")
	}
	if rec.Status != "pending" {
		t.Fatalf("expected default status 'pending', got %q", rec.Status)
	}
	if rec.CreatedAt.IsZero() || rec.UpdatedAt.IsZero() {
		t.Fatal("expected timestamps set")
	}
}

func TestSaveTask_BlankTaskID(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	err := store.SaveTask(context.Background(), TaskRecord{ChatID: "c1"})
	if err == nil {
		t.Fatal("expected error for blank task_id")
	}
}

func TestSaveTask_BlankChatID(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	err := store.SaveTask(context.Background(), TaskRecord{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected error for blank chat_id")
	}
}

func TestSaveTask_PreservesExistingTimestamps(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	err := store.SaveTask(context.Background(), TaskRecord{
		TaskID:    "t1",
		ChatID:    "c1",
		CreatedAt: fixed,
		UpdatedAt: fixed,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rec, _, _ := store.GetTask(context.Background(), "t1")
	if !rec.CreatedAt.Equal(fixed) {
		t.Fatal("expected CreatedAt preserved")
	}
}

// --- UpdateStatus ---

func TestUpdateStatus_SetsStatus(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	fixedTime := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return fixedTime }

	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1", Status: "pending"})
	_ = store.UpdateStatus(context.Background(), "t1", "running")

	rec, _, _ := store.GetTask(context.Background(), "t1")
	if rec.Status != "running" {
		t.Fatalf("expected running, got %q", rec.Status)
	}
	if !rec.UpdatedAt.Equal(fixedTime) {
		t.Fatal("expected UpdatedAt set")
	}
}

func TestUpdateStatus_SetsCompletedAtOnTerminal(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	fixedTime := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return fixedTime }

	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1", Status: "running"})
	_ = store.UpdateStatus(context.Background(), "t1", "completed")

	rec, _, _ := store.GetTask(context.Background(), "t1")
	if !rec.CompletedAt.Equal(fixedTime) {
		t.Fatal("expected CompletedAt set for terminal status")
	}
}

func TestUpdateStatus_UnknownTaskNoOp(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	err := store.UpdateStatus(context.Background(), "nonexistent", "failed")
	if err != nil {
		t.Fatalf("expected no error for unknown task, got %v", err)
	}
}

func TestUpdateStatus_BlankTaskID(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	err := store.UpdateStatus(context.Background(), "", "failed")
	if err == nil {
		t.Fatal("expected error for blank task_id")
	}
}

func TestUpdateStatus_WithOptions(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1"})

	_ = store.UpdateStatus(context.Background(), "t1", "completed",
		WithAnswerPreview("answer preview"),
		WithErrorText("some error"),
		WithTokensUsed(500),
		WithMergeStatus("merged"),
	)

	rec, _, _ := store.GetTask(context.Background(), "t1")
	if rec.AnswerPreview != "answer preview" {
		t.Fatalf("expected answer preview, got %q", rec.AnswerPreview)
	}
	if rec.Error != "some error" {
		t.Fatalf("expected error text, got %q", rec.Error)
	}
	if rec.TokensUsed != 500 {
		t.Fatalf("expected tokens 500, got %d", rec.TokensUsed)
	}
	if rec.MergeStatus != "merged" {
		t.Fatalf("expected merge status, got %q", rec.MergeStatus)
	}
}

// --- GetTask ---

func TestGetTask_NotFound(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_, ok, err := store.GetTask(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected not found")
	}
}

func TestGetTask_EmptyID(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_, ok, err := store.GetTask(context.Background(), "")
	if err != nil || ok {
		t.Fatalf("expected false for empty ID, got ok=%v err=%v", ok, err)
	}
}

// --- ListByChat ---

func TestListByChat_FiltersByChat(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1"})
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t2", ChatID: "c2"})

	got, _ := store.ListByChat(context.Background(), "c1", false, 10)
	if len(got) != 1 || got[0].TaskID != "t1" {
		t.Fatalf("expected [t1], got %v", got)
	}
}

func TestListByChat_ActiveOnly(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1", Status: "running"})
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t2", ChatID: "c1", Status: "completed"})

	got, _ := store.ListByChat(context.Background(), "c1", true, 10)
	if len(got) != 1 || got[0].TaskID != "t1" {
		t.Fatalf("expected [t1] (active only), got %v", got)
	}
}

func TestListByChat_Limit(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	for i := 0; i < 5; i++ {
		_ = store.SaveTask(context.Background(), TaskRecord{
			TaskID:    "t" + string(rune('a'+i)),
			ChatID:    "c1",
			CreatedAt: time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC),
		})
	}
	got, _ := store.ListByChat(context.Background(), "c1", false, 3)
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
}

func TestListByChat_SortedNewestFirst(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_ = store.SaveTask(context.Background(), TaskRecord{
		TaskID: "old", ChatID: "c1", CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	_ = store.SaveTask(context.Background(), TaskRecord{
		TaskID: "new", ChatID: "c1", CreatedAt: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
	})
	got, _ := store.ListByChat(context.Background(), "c1", false, 10)
	if len(got) != 2 || got[0].TaskID != "new" {
		t.Fatalf("expected newest first, got %v", got)
	}
}

func TestListByChat_EmptyChatID(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	got, _ := store.ListByChat(context.Background(), "", false, 10)
	if got != nil {
		t.Fatalf("expected nil for empty chat_id, got %v", got)
	}
}

// --- DeleteExpired ---

func TestDeleteExpired_RemovesOld(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	old := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1", CreatedAt: old})

	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = store.DeleteExpired(context.Background(), cutoff)

	_, ok, _ := store.GetTask(context.Background(), "t1")
	if ok {
		t.Fatal("expected task deleted")
	}
}

func TestDeleteExpired_KeepsRecent(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	recent := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1", CreatedAt: recent})

	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = store.DeleteExpired(context.Background(), cutoff)

	_, ok, _ := store.GetTask(context.Background(), "t1")
	if !ok {
		t.Fatal("expected task retained")
	}
}

// --- MarkStaleRunning ---

func TestMarkStaleRunning_MarksRunningAsFailed(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1", Status: "running"})
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t2", ChatID: "c1", Status: "completed"})

	_ = store.MarkStaleRunning(context.Background(), "server restart")

	t1, _, _ := store.GetTask(context.Background(), "t1")
	if t1.Status != "failed" {
		t.Fatalf("expected running→failed, got %q", t1.Status)
	}
	if t1.Error != "server restart" {
		t.Fatalf("expected error reason, got %q", t1.Error)
	}

	t2, _, _ := store.GetTask(context.Background(), "t2")
	if t2.Status != "completed" {
		t.Fatalf("expected completed unchanged, got %q", t2.Status)
	}
}

func TestMarkStaleRunning_MarksPendingAsFailed(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1", Status: "pending"})
	_ = store.MarkStaleRunning(context.Background(), "crash recovery")

	t1, _, _ := store.GetTask(context.Background(), "t1")
	if t1.Status != "failed" {
		t.Fatalf("expected pending→failed, got %q", t1.Status)
	}
}

func TestMarkStaleRunning_MarksWaitingInputAsFailed(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t1", ChatID: "c1", Status: "waiting_input"})
	_ = store.MarkStaleRunning(context.Background(), "restart")

	t1, _, _ := store.GetTask(context.Background(), "t1")
	if t1.Status != "failed" {
		t.Fatalf("expected waiting_input→failed, got %q", t1.Status)
	}
}

// --- eviction ---

func TestEviction_RetentionExpiry(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	completed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	_ = store.SaveTask(context.Background(), TaskRecord{
		TaskID:      "t1",
		ChatID:      "c1",
		Status:      "completed",
		CompletedAt: completed,
	})

	// Advance time past retention
	store.now = func() time.Time { return completed.Add(2 * time.Hour) }
	// Trigger eviction by saving another task
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t2", ChatID: "c1"})

	_, ok, _ := store.GetTask(context.Background(), "t1")
	if ok {
		t.Fatal("expected completed task evicted after retention")
	}
}

func TestEviction_ActiveTasksNeverEvictedByRetention(t *testing.T) {
	store := NewTaskMemoryStore(time.Hour, 100)
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store.now = func() time.Time { return old }
	_ = store.SaveTask(context.Background(), TaskRecord{
		TaskID: "t1", ChatID: "c1", Status: "running",
	})

	// Advance time past retention
	store.now = func() time.Time { return old.Add(2 * time.Hour) }
	_ = store.SaveTask(context.Background(), TaskRecord{TaskID: "t2", ChatID: "c2"})

	_, ok, _ := store.GetTask(context.Background(), "t1")
	if !ok {
		t.Fatal("expected active task preserved")
	}
}

// --- File persistence ---

func TestNewTaskFileStore_EmptyDir(t *testing.T) {
	_, err := NewTaskFileStore("", 0, 0)
	if err == nil {
		t.Fatal("expected error for empty dir")
	}
}
