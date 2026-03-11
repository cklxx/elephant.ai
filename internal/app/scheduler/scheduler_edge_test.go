package scheduler

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/testutil"
)

// ---------------------------------------------------------------------------
// Drain tests
// ---------------------------------------------------------------------------

func TestScheduler_Drain_CompletesWhenNoInFlight(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	drainCtx, drainCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer drainCancel()

	if err := sched.Drain(drainCtx); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	// After drain, Done channel should be closed.
	select {
	case <-sched.Done():
	default:
		t.Fatal("expected Done() to be closed after Drain")
	}
}

func TestScheduler_Drain_IdempotentWithStop(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Drain first.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer drainCancel()

	if err := sched.Drain(drainCtx); err != nil {
		t.Fatalf("Drain: %v", err)
	}

	// Stop after drain should not panic (stopOnce).
	sched.Stop()

	select {
	case <-sched.Done():
	default:
		t.Fatal("expected Done() to be closed")
	}
}

// ---------------------------------------------------------------------------
// RegisterDynamicTrigger tests
// ---------------------------------------------------------------------------

func TestScheduler_RegisterDynamicTrigger_Success(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	sched := New(Config{Enabled: true, JobStore: store}, &mockCoordinator{answer: "ok"}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	dto, err := sched.RegisterDynamicTrigger(ctx, "dynamic-test", "*/10 * * * *", "do something", "lark")
	if err != nil {
		t.Fatalf("RegisterDynamicTrigger: %v", err)
	}
	if dto.ID != "dynamic-test" {
		t.Errorf("ID = %q, want dynamic-test", dto.ID)
	}
	if dto.NextRun.IsZero() {
		t.Error("expected NextRun to be set")
	}

	// Verify the trigger is registered.
	found := false
	for _, n := range sched.TriggerNames() {
		if n == "dynamic-test" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected dynamic-test in trigger names")
	}
}

func TestScheduler_RegisterDynamicTrigger_EmptyName(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)
	_, err := sched.RegisterDynamicTrigger(context.Background(), "", "* * * * *", "task", "")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestScheduler_RegisterDynamicTrigger_EmptySchedule(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)
	_, err := sched.RegisterDynamicTrigger(context.Background(), "test", "", "task", "")
	if err == nil {
		t.Fatal("expected error for empty schedule")
	}
}

func TestScheduler_RegisterDynamicTrigger_InvalidCron(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)
	_, err := sched.RegisterDynamicTrigger(context.Background(), "test", "bad-cron", "task", "")
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

// ---------------------------------------------------------------------------
// UnregisterTrigger tests
// ---------------------------------------------------------------------------

func TestScheduler_UnregisterTrigger_Success(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	sched := New(Config{Enabled: true, JobStore: store}, &mockCoordinator{answer: "ok"}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer sched.Stop()

	// Register first.
	if _, err := sched.RegisterDynamicTrigger(ctx, "unreg-test", "*/5 * * * *", "task", ""); err != nil {
		t.Fatalf("RegisterDynamicTrigger: %v", err)
	}

	// Unregister.
	if err := sched.UnregisterTrigger(ctx, "unreg-test"); err != nil {
		t.Fatalf("UnregisterTrigger: %v", err)
	}

	// Trigger should be gone.
	for _, n := range sched.TriggerNames() {
		if n == "unreg-test" {
			t.Fatal("trigger should be unregistered")
		}
	}

	// Job store should no longer have it.
	_, err := store.Load(ctx, "unreg-test")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

func TestScheduler_UnregisterTrigger_Nonexistent(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	// Should not error when trigger doesn't exist and no store.
	err := sched.UnregisterTrigger(context.Background(), "ghost")
	if err != nil {
		t.Fatalf("UnregisterTrigger for nonexistent: %v", err)
	}
}

func TestScheduler_UnregisterTrigger_StoreDeleteError(t *testing.T) {
	failStore := &failingJobStore{deleteErr: errors.New("disk full")}
	sched := New(Config{Enabled: true, JobStore: failStore}, &mockCoordinator{answer: "ok"}, nil, nil)

	err := sched.UnregisterTrigger(context.Background(), "some-trigger")
	if err == nil {
		t.Fatal("expected error from store delete")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("expected 'disk full' in error, got %v", err)
	}
}

// failingJobStore is a minimal JobStore that returns errors for targeted ops.
type failingJobStore struct {
	deleteErr error
}

func (f *failingJobStore) Save(_ context.Context, _ Job) error            { return nil }
func (f *failingJobStore) Load(_ context.Context, _ string) (*Job, error) { return nil, ErrJobNotFound }
func (f *failingJobStore) List(_ context.Context) ([]Job, error)          { return nil, nil }
func (f *failingJobStore) Delete(_ context.Context, _ string) error       { return f.deleteErr }
func (f *failingJobStore) UpdateStatus(_ context.Context, _ string, _ JobStatus) error {
	return nil
}

// ---------------------------------------------------------------------------
// ListJobs / LoadJob with nil store
// ---------------------------------------------------------------------------

func TestScheduler_ListJobs_NilStore(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)
	_, err := sched.ListJobs(context.Background())
	if err == nil {
		t.Fatal("expected error for nil job store")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured', got %v", err)
	}
}

func TestScheduler_LoadJob_NilStore(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)
	_, err := sched.LoadJob(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for nil job store")
	}
	if !strings.Contains(err.Error(), "not configured") {
		t.Errorf("expected 'not configured', got %v", err)
	}
}

func TestScheduler_ListJobs_Success(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	mustSave(t, store, newTestJob("list-a", "A"))
	mustSave(t, store, newTestJob("list-b", "B"))

	sched := New(Config{Enabled: true, JobStore: store}, &mockCoordinator{answer: "ok"}, nil, nil)
	jobs, err := sched.ListJobs(context.Background())
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
}

func TestScheduler_LoadJob_NotFound(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	sched := New(Config{Enabled: true, JobStore: store}, &mockCoordinator{answer: "ok"}, nil, nil)

	_, err := sched.LoadJob(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// validateLarkTrigger edge cases
// ---------------------------------------------------------------------------

func TestValidateLarkTrigger_WhitespaceOnlyUserID(t *testing.T) {
	err := validateLarkTrigger(Trigger{Channel: "lark", UserID: "   "})
	if err == nil {
		t.Fatal("expected error for whitespace-only user_id")
	}
}

func TestValidateLarkTrigger_NonLarkChannelPassesAlways(t *testing.T) {
	// Non-lark channel should pass regardless of user_id state.
	err := validateLarkTrigger(Trigger{Channel: "web", UserID: ""})
	if err != nil {
		t.Fatalf("non-lark channel should pass: %v", err)
	}
}

func TestValidateLarkTrigger_EmptyChannelPasses(t *testing.T) {
	err := validateLarkTrigger(Trigger{Channel: "", UserID: ""})
	if err != nil {
		t.Fatalf("empty channel should pass: %v", err)
	}
}

func TestValidateLarkTrigger_CaseInsensitiveChannel(t *testing.T) {
	err := validateLarkTrigger(Trigger{Channel: "LARK", UserID: "ou_test"})
	if err != nil {
		t.Fatalf("case insensitive lark should pass: %v", err)
	}
}

func TestValidateLarkTrigger_LarkMissingUserID(t *testing.T) {
	err := validateLarkTrigger(Trigger{Channel: "lark", UserID: ""})
	if err == nil {
		t.Fatal("expected error for missing user_id")
	}
}

// ---------------------------------------------------------------------------
// finishJob resets failure state on success
// ---------------------------------------------------------------------------

func TestScheduler_FinishJob_SuccessResetsFailureState(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	now := time.Now().UTC()
	sched.mu.Lock()
	sched.jobs["reset-job"] = &Job{
		ID:           "reset-job",
		Name:         "Reset",
		CronExpr:     "* * * * *",
		Status:       JobStatusActive,
		FailureCount: 3,
		LastFailure:  now.Add(-1 * time.Minute),
		LastError:    "previous error",
	}
	sched.inFlight["reset-job"] = 1
	sched.mu.Unlock()

	sched.finishJob("reset-job", nil)

	sched.mu.Lock()
	job := sched.jobs["reset-job"]
	sched.mu.Unlock()

	if job.FailureCount != 0 {
		t.Errorf("expected FailureCount=0 after success, got %d", job.FailureCount)
	}
	if !job.LastFailure.IsZero() {
		t.Errorf("expected LastFailure reset to zero, got %v", job.LastFailure)
	}
	if job.LastError != "" {
		t.Errorf("expected LastError cleared, got %q", job.LastError)
	}
}

func TestScheduler_FinishJob_ErrorIncrementsFailureCount(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	sched.mu.Lock()
	sched.jobs["fail-job"] = &Job{
		ID:           "fail-job",
		Name:         "Fail",
		CronExpr:     "* * * * *",
		Status:       JobStatusActive,
		FailureCount: 1,
	}
	sched.inFlight["fail-job"] = 1
	sched.mu.Unlock()

	sched.finishJob("fail-job", errors.New("boom"))

	sched.mu.Lock()
	job := sched.jobs["fail-job"]
	sched.mu.Unlock()

	if job.FailureCount != 2 {
		t.Errorf("expected FailureCount=2, got %d", job.FailureCount)
	}
	if job.LastError != "boom" {
		t.Errorf("expected LastError='boom', got %q", job.LastError)
	}
	if job.LastFailure.IsZero() {
		t.Error("expected LastFailure to be set")
	}
}

func TestScheduler_FinishJob_UnknownJobNoOp(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)
	// Should not panic.
	sched.finishJob("nonexistent", nil)
}

// ---------------------------------------------------------------------------
// startJob with paused/completed jobs
// ---------------------------------------------------------------------------

func TestScheduler_StartJob_SkipsPaused(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	sched.mu.Lock()
	sched.jobs["paused-job"] = &Job{
		ID:       "paused-job",
		Name:     "Paused",
		CronExpr: "* * * * *",
		Status:   JobStatusPaused,
	}
	sched.mu.Unlock()

	_, _, ok := sched.startJob("paused-job", jobRunOptions{})
	if ok {
		t.Fatal("expected startJob to skip paused job")
	}
}

func TestScheduler_StartJob_SkipsCompleted(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	sched.mu.Lock()
	sched.jobs["completed-job"] = &Job{
		ID:       "completed-job",
		Name:     "Completed",
		CronExpr: "* * * * *",
		Status:   JobStatusCompleted,
	}
	sched.mu.Unlock()

	_, _, ok := sched.startJob("completed-job", jobRunOptions{})
	if ok {
		t.Fatal("expected startJob to skip completed job")
	}
}

func TestScheduler_StartJob_SkipsUnknownJob(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	_, _, ok := sched.startJob("unknown", jobRunOptions{})
	if ok {
		t.Fatal("expected startJob to skip unknown job")
	}
}

// ---------------------------------------------------------------------------
// recoveryDelay edge cases
// ---------------------------------------------------------------------------

func TestRecoveryDelay_ZeroFailureCount(t *testing.T) {
	sched := &Scheduler{config: Config{RecoveryBackoff: time.Minute}}
	got := sched.recoveryDelay(0)
	// failureCount < 1 is clamped to 1, so delay = 1 * 2^0 = 1 minute.
	if got != time.Minute {
		t.Fatalf("expected 1m for failureCount=0, got %v", got)
	}
}

func TestRecoveryDelay_NegativeFailureCount(t *testing.T) {
	sched := &Scheduler{config: Config{RecoveryBackoff: time.Minute}}
	got := sched.recoveryDelay(-5)
	if got != time.Minute {
		t.Fatalf("expected 1m for negative failureCount, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// formatResult edge cases
// ---------------------------------------------------------------------------

func TestFormatResult_EmptyAnswer(t *testing.T) {
	trigger := Trigger{Name: "test"}
	result := &agent.TaskResult{Answer: "   "}
	got := formatResult(trigger, result, nil)
	if !strings.Contains(got, "已完成") {
		t.Errorf("expected '已完成' for whitespace answer, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Notify edge cases
// ---------------------------------------------------------------------------

func TestScheduler_NotifyTriggerResult_NoChannel(t *testing.T) {
	notifier := &testutil.StubNotifier{}
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, notifier, nil)

	// Empty channel should skip notification.
	sched.notifyTriggerResult(context.Background(), Trigger{Name: "test", Channel: ""}, "content")
	if notifier.Count() != 0 {
		t.Fatalf("expected no notification for empty channel, got %d", notifier.Count())
	}
}

// ---------------------------------------------------------------------------
// Concurrency: bypass cooldown in recovery
// ---------------------------------------------------------------------------

func TestScheduler_RunJob_BypassCooldown(t *testing.T) {
	coord := &mockCoordinator{answer: "ok"}
	sched := New(Config{
		Enabled:  true,
		Cooldown: 10 * time.Second, // long cooldown
	}, coord, nil, nil)

	sched.mu.Lock()
	if err := sched.registerTriggerLocked(context.Background(), Trigger{Name: "bypass", Schedule: "* * * * *", Task: "Task"}); err != nil {
		sched.mu.Unlock()
		t.Fatalf("registerTriggerLocked: %v", err)
	}
	sched.mu.Unlock()

	// First run sets LastRun.
	if !sched.runJob("bypass", jobRunOptions{}) {
		t.Fatal("expected first run to execute")
	}

	// Normal run should be blocked by cooldown.
	if sched.runJob("bypass", jobRunOptions{}) {
		t.Fatal("expected cooldown to skip execution")
	}

	// Bypass cooldown should succeed.
	if !sched.runJob("bypass", jobRunOptions{bypassCooldown: true}) {
		t.Fatal("expected bypass cooldown to execute")
	}

	if coord.callCount() != 2 {
		t.Fatalf("expected 2 calls, got %d", coord.callCount())
	}
}

// ---------------------------------------------------------------------------
// Done channel
// ---------------------------------------------------------------------------

func TestScheduler_Done_ClosedAfterStop(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	sched.Stop()

	select {
	case <-sched.Done():
	case <-time.After(time.Second):
		t.Fatal("expected Done() to be closed after Stop")
	}
}

// ---------------------------------------------------------------------------
// Stop idempotency
// ---------------------------------------------------------------------------

func TestScheduler_Stop_Idempotent(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Multiple stops should not panic.
	sched.Stop()
	sched.Stop()
	sched.Stop()
}

// ---------------------------------------------------------------------------
// validateTrigger
// ---------------------------------------------------------------------------

func TestValidateTrigger_EmptyName(t *testing.T) {
	err := validateTrigger(Trigger{Name: "", Schedule: "* * * * *"})
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestValidateTrigger_EmptySchedule(t *testing.T) {
	err := validateTrigger(Trigger{Name: "test", Schedule: ""})
	if err == nil {
		t.Fatal("expected error for empty schedule")
	}
}

func TestValidateTrigger_Valid(t *testing.T) {
	err := validateTrigger(Trigger{Name: "test", Schedule: "* * * * *"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Job payload round-trip
// ---------------------------------------------------------------------------

func TestPayloadFromTrigger_EmptyFields(t *testing.T) {
	data, err := payloadFromTrigger(Trigger{})
	if err != nil {
		t.Fatalf("payloadFromTrigger: %v", err)
	}
	if data != nil {
		t.Errorf("expected nil payload for empty trigger, got %s", data)
	}
}

func TestTriggerFromJob_EmptyPayload(t *testing.T) {
	job := Job{ID: "test", CronExpr: "* * * * *", Trigger: "task"}
	trigger, err := triggerFromJob(job)
	if err != nil {
		t.Fatalf("triggerFromJob: %v", err)
	}
	if trigger.Channel != "" {
		t.Errorf("expected empty channel, got %q", trigger.Channel)
	}
}

func TestTriggerFromJob_InvalidPayload(t *testing.T) {
	job := Job{ID: "test", CronExpr: "* * * * *", Trigger: "task", Payload: []byte("{{{bad")}
	_, err := triggerFromJob(job)
	if err == nil {
		t.Fatal("expected error for invalid payload")
	}
}

// ---------------------------------------------------------------------------
// releaseLeaderLock edge cases
// ---------------------------------------------------------------------------

func TestScheduler_ReleaseLeaderLock_NoLockConfigured(t *testing.T) {
	sched := New(Config{Enabled: true}, &mockCoordinator{answer: "ok"}, nil, nil)
	// Should not panic with nil LeaderLock.
	sched.releaseLeaderLock()
}

func TestScheduler_ReleaseLeaderLock_NotHeld(t *testing.T) {
	lock := &mockLeaderLock{acquireOK: true}
	sched := New(Config{Enabled: true, LeaderLock: lock}, &mockCoordinator{answer: "ok"}, nil, nil)
	// lockHeld is false by default — release should be a no-op.
	sched.releaseLeaderLock()
	_, releaseCalls := lock.stats()
	if releaseCalls != 0 {
		t.Fatalf("expected no release call, got %d", releaseCalls)
	}
}
