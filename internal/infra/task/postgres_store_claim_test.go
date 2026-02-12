package task

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	taskdomain "alex/internal/domain/task"
	"alex/internal/shared/testutil"
)

func TestPostgresStore_TryClaimTaskSingleWinner(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	store := NewPostgresStore(pool)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	task := &taskdomain.Task{
		TaskID:      "claim-single-winner",
		SessionID:   "session-1",
		Description: "claim me",
		Channel:     "web",
		Status:      taskdomain.StatusPending,
	}
	if err := store.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	leaseUntil := time.Now().Add(45 * time.Second)
	var wins int32
	var wg sync.WaitGroup
	for _, owner := range []string{"owner-a", "owner-b"} {
		owner := owner
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := store.TryClaimTask(ctx, task.TaskID, owner, leaseUntil)
			if err != nil {
				t.Errorf("try claim (%s): %v", owner, err)
				return
			}
			if ok {
				atomic.AddInt32(&wins, 1)
			}
		}()
	}
	wg.Wait()

	if wins != 1 {
		t.Fatalf("expected exactly one winner, got %d", wins)
	}
}

func TestPostgresStore_TaskLeaseLifecycle(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	store := NewPostgresStore(pool)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	task := &taskdomain.Task{
		TaskID:      "claim-lease-lifecycle",
		SessionID:   "session-1",
		Description: "lease me",
		Channel:     "web",
		Status:      taskdomain.StatusPending,
	}
	if err := store.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	ok, err := store.TryClaimTask(ctx, task.TaskID, "owner-a", time.Now().Add(45*time.Second))
	if err != nil || !ok {
		t.Fatalf("owner-a claim failed ok=%v err=%v", ok, err)
	}

	ok, err = store.RenewTaskLease(ctx, task.TaskID, "owner-b", time.Now().Add(45*time.Second))
	if err != nil {
		t.Fatalf("renew with wrong owner error: %v", err)
	}
	if ok {
		t.Fatal("renew with wrong owner should fail")
	}

	ok, err = store.RenewTaskLease(ctx, task.TaskID, "owner-a", time.Now().Add(45*time.Second))
	if err != nil || !ok {
		t.Fatalf("renew with owner-a failed ok=%v err=%v", ok, err)
	}

	if err := store.ReleaseTaskLease(ctx, task.TaskID, "owner-a"); err != nil {
		t.Fatalf("release lease: %v", err)
	}

	ok, err = store.TryClaimTask(ctx, task.TaskID, "owner-b", time.Now().Add(45*time.Second))
	if err != nil || !ok {
		t.Fatalf("owner-b claim after release failed ok=%v err=%v", ok, err)
	}
}

func TestPostgresStore_ClaimResumableTasks(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	t.Cleanup(cleanup)

	store := NewPostgresStore(pool)
	ctx := context.Background()
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	pending := &taskdomain.Task{
		TaskID:      "resume-pending",
		SessionID:   "session-1",
		Description: "pending",
		Channel:     "web",
		Status:      taskdomain.StatusPending,
	}
	running := &taskdomain.Task{
		TaskID:      "resume-running",
		SessionID:   "session-1",
		Description: "running",
		Channel:     "web",
		Status:      taskdomain.StatusPending,
	}
	completed := &taskdomain.Task{
		TaskID:      "resume-completed",
		SessionID:   "session-1",
		Description: "completed",
		Channel:     "web",
		Status:      taskdomain.StatusPending,
	}
	for _, task := range []*taskdomain.Task{pending, running, completed} {
		if err := store.Create(ctx, task); err != nil {
			t.Fatalf("create task %s: %v", task.TaskID, err)
		}
	}
	if err := store.SetStatus(ctx, running.TaskID, taskdomain.StatusRunning); err != nil {
		t.Fatalf("set running: %v", err)
	}
	if err := store.SetStatus(ctx, completed.TaskID, taskdomain.StatusCompleted); err != nil {
		t.Fatalf("set completed: %v", err)
	}

	claimed, err := store.ClaimResumableTasks(
		ctx,
		"resume-owner",
		time.Now().Add(45*time.Second),
		10,
		taskdomain.StatusPending,
		taskdomain.StatusRunning,
	)
	if err != nil {
		t.Fatalf("claim resumable: %v", err)
	}
	if len(claimed) != 2 {
		t.Fatalf("expected 2 claimed tasks, got %d", len(claimed))
	}

	seen := map[string]bool{}
	for _, task := range claimed {
		seen[task.TaskID] = true
	}
	if !seen[pending.TaskID] {
		t.Fatalf("expected pending task %s to be claimed", pending.TaskID)
	}
	if !seen[running.TaskID] {
		t.Fatalf("expected running task %s to be claimed", running.TaskID)
	}
	if seen[completed.TaskID] {
		t.Fatalf("did not expect completed task %s to be claimed", completed.TaskID)
	}
}
