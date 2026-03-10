package taskstore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/domain/task"
)

func newTestStore(t *testing.T) *LocalStore {
	t.Helper()
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")
	s := New(WithFilePath(fp), WithRetention(time.Hour), WithMaxTasks(100))
	t.Cleanup(func() { s.Close() })
	return s
}

func makeTask(id, sessionID, chatID string, status task.Status) *task.Task {
	return &task.Task{
		TaskID:    id,
		SessionID: sessionID,
		ChatID:    chatID,
		Status:    status,
		Channel:   "test",
	}
}

func TestCreateAndGet(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "s1", "c1", task.StatusPending)
	if err := s.Create(ctx, tk); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := s.Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.TaskID != "t1" {
		t.Errorf("TaskID = %q, want t1", got.TaskID)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestCreateDuplicate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	tk := makeTask("t1", "s1", "", task.StatusPending)
	_ = s.Create(ctx, tk)
	if err := s.Create(ctx, tk); err == nil {
		t.Fatal("expected error on duplicate create")
	}
}

func TestGetNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent task")
	}
}

func TestSetStatusAndTransitions(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusPending))

	err := s.SetStatus(ctx, "t1", task.StatusRunning, task.WithTransitionReason("starting"))
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	got, _ := s.Get(ctx, "t1")
	if got.Status != task.StatusRunning {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("StartedAt should be set on running")
	}

	err = s.SetStatus(ctx, "t1", task.StatusCompleted)
	if err != nil {
		t.Fatalf("SetStatus completed: %v", err)
	}

	got, _ = s.Get(ctx, "t1")
	if got.CompletedAt == nil {
		t.Error("CompletedAt should be set")
	}
	if got.TerminationReason != task.TerminationCompleted {
		t.Errorf("TerminationReason = %q, want completed", got.TerminationReason)
	}

	trs, err := s.Transitions(ctx, "t1")
	if err != nil {
		t.Fatalf("Transitions: %v", err)
	}
	if len(trs) != 2 {
		t.Fatalf("transitions count = %d, want 2", len(trs))
	}
	if trs[0].Reason != "starting" {
		t.Errorf("first transition reason = %q, want starting", trs[0].Reason)
	}
}

func TestUpdateProgress(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusRunning))

	err := s.UpdateProgress(ctx, "t1", 5, 1000, 0.05)
	if err != nil {
		t.Fatalf("UpdateProgress: %v", err)
	}

	got, _ := s.Get(ctx, "t1")
	if got.CurrentIteration != 5 {
		t.Errorf("CurrentIteration = %d, want 5", got.CurrentIteration)
	}
	if got.TokensUsed != 1000 {
		t.Errorf("TokensUsed = %d, want 1000", got.TokensUsed)
	}
}

func TestSetResultAndSetError(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusRunning))

	_ = s.SetResult(ctx, "t1", "answer here", nil, 500)
	got, _ := s.Get(ctx, "t1")
	if got.AnswerPreview != "answer here" {
		t.Errorf("AnswerPreview = %q", got.AnswerPreview)
	}

	_ = s.Create(ctx, makeTask("t2", "s1", "", task.StatusRunning))
	_ = s.SetError(ctx, "t2", "something failed")
	got2, _ := s.Get(ctx, "t2")
	if got2.Error != "something failed" {
		t.Errorf("Error = %q", got2.Error)
	}
}

func TestSetBridgeMeta(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusRunning))

	meta := task.BridgeMeta{PID: 12345, OutputFile: "/tmp/out.json"}
	if err := s.SetBridgeMeta(ctx, "t1", meta); err != nil {
		t.Fatalf("SetBridgeMeta: %v", err)
	}

	got, _ := s.Get(ctx, "t1")
	if got.BridgeMeta == nil {
		t.Fatal("BridgeMeta should not be nil")
	}
	if got.BridgeMeta.PID != 12345 {
		t.Errorf("PID = %d, want 12345", got.BridgeMeta.PID)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusPending))
	if err := s.Delete(ctx, "t1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(ctx, "t1"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestListBySession(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusPending))
	_ = s.Create(ctx, makeTask("t2", "s1", "", task.StatusRunning))
	_ = s.Create(ctx, makeTask("t3", "s2", "", task.StatusPending))

	tasks, err := s.ListBySession(ctx, "s1", 10)
	if err != nil {
		t.Fatalf("ListBySession: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("count = %d, want 2", len(tasks))
	}
}

func TestListByChat(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "c1", task.StatusPending))
	_ = s.Create(ctx, makeTask("t2", "s1", "c1", task.StatusCompleted))
	_ = s.Create(ctx, makeTask("t3", "s1", "c2", task.StatusPending))

	all, _ := s.ListByChat(ctx, "c1", false, 10)
	if len(all) != 2 {
		t.Fatalf("all count = %d, want 2", len(all))
	}

	active, _ := s.ListByChat(ctx, "c1", true, 10)
	if len(active) != 1 {
		t.Fatalf("active count = %d, want 1", len(active))
	}
}

func TestListByStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusPending))
	_ = s.Create(ctx, makeTask("t2", "s1", "", task.StatusRunning))
	_ = s.Create(ctx, makeTask("t3", "s1", "", task.StatusCompleted))

	tasks, _ := s.ListByStatus(ctx, task.StatusPending, task.StatusRunning)
	if len(tasks) != 2 {
		t.Fatalf("count = %d, want 2", len(tasks))
	}
}

func TestListActive(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusPending))
	_ = s.Create(ctx, makeTask("t2", "s1", "", task.StatusRunning))
	_ = s.Create(ctx, makeTask("t3", "s1", "", task.StatusCompleted))
	_ = s.Create(ctx, makeTask("t4", "s1", "", task.StatusFailed))

	tasks, _ := s.ListActive(ctx)
	if len(tasks) != 2 {
		t.Fatalf("count = %d, want 2", len(tasks))
	}
}

func TestListPagination(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		tk := makeTask(fmt.Sprintf("t%d", i), "s1", "", task.StatusPending)
		_ = s.Create(ctx, tk)
	}

	tasks, total, _ := s.List(ctx, 2, 0)
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(tasks) != 2 {
		t.Errorf("page size = %d, want 2", len(tasks))
	}

	tasks2, _, _ := s.List(ctx, 2, 4)
	if len(tasks2) != 1 {
		t.Errorf("last page = %d, want 1", len(tasks2))
	}
}

func TestTryClaimAndRelease(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusPending))

	lease := time.Now().Add(5 * time.Minute)

	ok, err := s.TryClaimTask(ctx, "t1", "w1", lease)
	if err != nil || !ok {
		t.Fatalf("TryClaimTask: ok=%v, err=%v", ok, err)
	}

	// Another worker can't claim.
	ok2, err := s.TryClaimTask(ctx, "t1", "w2", lease)
	if err != nil {
		t.Fatalf("TryClaimTask w2: %v", err)
	}
	if ok2 {
		t.Error("expected claim to fail for w2")
	}

	// Release and reclaim.
	_ = s.ReleaseTaskLease(ctx, "t1", "w1")
	ok3, _ := s.TryClaimTask(ctx, "t1", "w2", lease)
	if !ok3 {
		t.Error("expected claim to succeed after release")
	}
}

func TestRenewLease(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusRunning))
	lease := time.Now().Add(5 * time.Minute)
	_, _ = s.TryClaimTask(ctx, "t1", "w1", lease)

	newLease := time.Now().Add(10 * time.Minute)
	ok, err := s.RenewTaskLease(ctx, "t1", "w1", newLease)
	if err != nil || !ok {
		t.Fatalf("RenewTaskLease: ok=%v, err=%v", ok, err)
	}

	// Wrong owner can't renew.
	ok2, _ := s.RenewTaskLease(ctx, "t1", "w2", newLease)
	if ok2 {
		t.Error("expected renew to fail for wrong owner")
	}
}

func TestClaimResumableTasks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusRunning))
	_ = s.Create(ctx, makeTask("t2", "s1", "", task.StatusPending))
	_ = s.Create(ctx, makeTask("t3", "s1", "", task.StatusCompleted))

	lease := time.Now().Add(5 * time.Minute)
	claimed, err := s.ClaimResumableTasks(ctx, "w1", lease, 10, task.StatusRunning, task.StatusPending)
	if err != nil {
		t.Fatalf("ClaimResumableTasks: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("claimed = %d, want 2", len(claimed))
	}
}

func TestMarkStaleRunning(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_ = s.Create(ctx, makeTask("t1", "s1", "", task.StatusRunning))
	_ = s.Create(ctx, makeTask("t2", "s1", "", task.StatusPending))
	_ = s.Create(ctx, makeTask("t3", "s1", "", task.StatusCompleted))

	if err := s.MarkStaleRunning(ctx, "server restart"); err != nil {
		t.Fatalf("MarkStaleRunning: %v", err)
	}

	t1, _ := s.Get(ctx, "t1")
	if t1.Status != task.StatusFailed {
		t.Errorf("t1 status = %q, want failed", t1.Status)
	}
	if t1.Error != "server restart" {
		t.Errorf("t1 error = %q", t1.Error)
	}

	t3, _ := s.Get(ctx, "t3")
	if t3.Status != task.StatusCompleted {
		t.Error("completed task should not be affected")
	}
}

func TestDeleteExpired(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	old := makeTask("t1", "s1", "", task.StatusCompleted)
	oldTime := time.Now().Add(-48 * time.Hour)
	old.CompletedAt = &oldTime
	_ = s.Create(ctx, old)

	recent := makeTask("t2", "s1", "", task.StatusCompleted)
	recentTime := time.Now().Add(-1 * time.Hour)
	recent.CompletedAt = &recentTime
	_ = s.Create(ctx, recent)

	_ = s.Create(ctx, makeTask("t3", "s1", "", task.StatusRunning))

	cutoff := time.Now().Add(-24 * time.Hour)
	if err := s.DeleteExpired(ctx, cutoff); err != nil {
		t.Fatalf("DeleteExpired: %v", err)
	}

	if _, err := s.Get(ctx, "t1"); err == nil {
		t.Error("t1 should be deleted")
	}
	if _, err := s.Get(ctx, "t2"); err != nil {
		t.Error("t2 should still exist")
	}
	if _, err := s.Get(ctx, "t3"); err != nil {
		t.Error("t3 should still exist")
	}
}

func TestFileReload(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "tasks.json")

	// Create store, add tasks, close.
	s1 := New(WithFilePath(fp))
	ctx := context.Background()
	_ = s1.Create(ctx, makeTask("t1", "s1", "", task.StatusRunning))
	_ = s1.SetBridgeMeta(ctx, "t1", task.BridgeMeta{PID: 999})
	s1.Close()

	// Re-open and verify.
	s2 := New(WithFilePath(fp))
	defer s2.Close()

	got, err := s2.Get(ctx, "t1")
	if err != nil {
		t.Fatalf("Get after reload: %v", err)
	}
	if got.Status != task.StatusRunning {
		t.Errorf("Status = %q, want running", got.Status)
	}
	if got.BridgeMeta == nil || got.BridgeMeta.PID != 999 {
		t.Error("BridgeMeta should survive reload")
	}

	// Verify file exists.
	if _, err := os.Stat(fp); err != nil {
		t.Errorf("task file should exist: %v", err)
	}
}
