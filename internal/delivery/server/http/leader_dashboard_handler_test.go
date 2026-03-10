package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"alex/internal/app/blocker"
	"alex/internal/app/scheduler"
	"alex/internal/app/summary"
	"alex/internal/domain/task"
	"alex/internal/infra/taskstore"
)

func newDashboardTestStore(t *testing.T) task.Store {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "tasks.json")
	s := taskstore.New(taskstore.WithFilePath(fp))
	t.Cleanup(func() { s.Close() })
	return s
}

func makeDashboardTask(id, desc string, status task.Status) *task.Task {
	now := time.Now()
	return &task.Task{
		TaskID:      id,
		SessionID:   "s1",
		Description: desc,
		Status:      status,
		Channel:     "test",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// --- Handler construction tests ---

func TestNewLeaderDashboardHandler_NilStore(t *testing.T) {
	h := NewLeaderDashboardHandler(nil, nil, nil, nil)
	if h != nil {
		t.Fatal("expected nil handler for nil store")
	}
}

func TestLeaderDashboardHandler_NilReceiver(t *testing.T) {
	var h *LeaderDashboardHandler
	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// --- Empty store tests ---

func TestDashboard_EmptyStore(t *testing.T) {
	store := newDashboardTestStore(t)
	h := NewLeaderDashboardHandler(store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp DashboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.TasksByStatus.Pending != 0 || resp.TasksByStatus.InProgress != 0 ||
		resp.TasksByStatus.Blocked != 0 || resp.TasksByStatus.Completed != 0 {
		t.Errorf("expected zero counts, got %+v", resp.TasksByStatus)
	}
	if resp.RecentBlockers != nil {
		t.Errorf("expected nil blockers, got %v", resp.RecentBlockers)
	}
	if resp.DailySummary != nil {
		t.Errorf("expected nil daily summary without generator")
	}
	if resp.ScheduledJobs != nil {
		t.Errorf("expected nil jobs without scheduler")
	}
}

// --- Task counts tests ---

func TestDashboard_TaskCounts(t *testing.T) {
	store := newDashboardTestStore(t)
	ctx := context.Background()

	// Create tasks in various statuses.
	pending := makeDashboardTask("p1", "pending task", task.StatusPending)
	_ = store.Create(ctx, pending)

	running := makeDashboardTask("r1", "running task", task.StatusPending)
	_ = store.Create(ctx, running)
	_ = store.SetStatus(ctx, "r1", task.StatusRunning)

	failed := makeDashboardTask("f1", "failed task", task.StatusPending)
	_ = store.Create(ctx, failed)
	_ = store.SetStatus(ctx, "f1", task.StatusFailed)

	waiting := makeDashboardTask("w1", "waiting task", task.StatusPending)
	_ = store.Create(ctx, waiting)
	_ = store.SetStatus(ctx, "w1", task.StatusWaitingInput)

	completed := makeDashboardTask("c1", "done task", task.StatusPending)
	_ = store.Create(ctx, completed)
	_ = store.SetStatus(ctx, "c1", task.StatusCompleted)

	h := NewLeaderDashboardHandler(store, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp DashboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.TasksByStatus.Pending != 1 {
		t.Errorf("pending = %d, want 1", resp.TasksByStatus.Pending)
	}
	if resp.TasksByStatus.InProgress != 1 {
		t.Errorf("in_progress = %d, want 1", resp.TasksByStatus.InProgress)
	}
	if resp.TasksByStatus.Blocked != 2 {
		t.Errorf("blocked = %d, want 2 (failed + waiting)", resp.TasksByStatus.Blocked)
	}
	if resp.TasksByStatus.Completed != 1 {
		t.Errorf("completed = %d, want 1", resp.TasksByStatus.Completed)
	}
}

// --- Blocker alerts tests ---

func TestDashboard_BlockerAlerts(t *testing.T) {
	store := newDashboardTestStore(t)
	ctx := context.Background()

	// Create a stale running task (updated long ago).
	stale := makeDashboardTask("stale1", "stale task", task.StatusPending)
	stale.UpdatedAt = time.Now().Add(-2 * time.Hour)
	_ = store.Create(ctx, stale)
	_ = store.SetStatus(ctx, "stale1", task.StatusRunning)
	// Manually backdate UpdatedAt by re-creating — the radar checks UpdatedAt.
	// Since SetStatus updates UpdatedAt, we need a very short threshold instead.

	cfg := blocker.Config{
		Enabled:               true,
		StaleThresholdSeconds: 1, // 1 second threshold for testing
		StaleThreshold:        time.Second,
		InputWaitSeconds:      1,
		InputWaitThreshold:    time.Second,
	}
	radar := blocker.NewRadar(store, nil, cfg)

	// Wait a moment for the threshold to be exceeded.
	time.Sleep(2 * time.Second)

	h := NewLeaderDashboardHandler(store, radar, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp DashboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.RecentBlockers) == 0 {
		t.Fatal("expected at least 1 blocker alert")
	}

	found := false
	for _, a := range resp.RecentBlockers {
		if a.TaskID == "stale1" && a.Reason == "stale_progress" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected stale_progress alert for stale1, got %+v", resp.RecentBlockers)
	}
}

// --- Daily summary tests ---

func TestDashboard_DailySummary(t *testing.T) {
	store := newDashboardTestStore(t)
	ctx := context.Background()

	tk := makeDashboardTask("t1", "completed task", task.StatusPending)
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "t1", task.StatusCompleted)

	gen := summary.NewGenerator(store)
	h := NewLeaderDashboardHandler(store, nil, gen, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp DashboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.DailySummary == nil {
		t.Fatal("expected daily summary")
	}
	if resp.DailySummary.Completed != 1 {
		t.Errorf("completed = %d, want 1", resp.DailySummary.Completed)
	}
	if resp.DailySummary.NewTasks != 1 {
		t.Errorf("new_tasks = %d, want 1", resp.DailySummary.NewTasks)
	}
}

// --- Scheduled jobs tests ---

type mockScheduler struct {
	jobs []scheduler.JobDTO
	err  error
}

func (m *mockScheduler) ListJobs(_ context.Context) ([]scheduler.JobDTO, error) {
	return m.jobs, m.err
}

func TestDashboard_ScheduledJobs(t *testing.T) {
	store := newDashboardTestStore(t)
	nextRun := time.Date(2026, 3, 17, 9, 0, 0, 0, time.UTC)
	lastRun := time.Date(2026, 3, 10, 9, 0, 0, 0, time.UTC)

	mock := &mockScheduler{
		jobs: []scheduler.JobDTO{
			{
				Name:     "weekly_pulse",
				CronExpr: "0 9 * * 1",
				Status:   "active",
				NextRun:  nextRun,
				LastRun:  lastRun,
			},
			{
				Name:     "milestone_checkin",
				CronExpr: "0 * * * *",
				Status:   "active",
				NextRun:  nextRun,
			},
		},
	}

	h := NewLeaderDashboardHandler(store, nil, nil, mock)
	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp DashboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(resp.ScheduledJobs) != 2 {
		t.Fatalf("jobs = %d, want 2", len(resp.ScheduledJobs))
	}
	if resp.ScheduledJobs[0].Name != "weekly_pulse" {
		t.Errorf("first job = %q, want weekly_pulse", resp.ScheduledJobs[0].Name)
	}
	if resp.ScheduledJobs[0].CronExpr != "0 9 * * 1" {
		t.Errorf("cron = %q, want '0 9 * * 1'", resp.ScheduledJobs[0].CronExpr)
	}
}

func TestDashboard_SchedulerError(t *testing.T) {
	store := newDashboardTestStore(t)
	mock := &mockScheduler{err: context.DeadlineExceeded}

	h := NewLeaderDashboardHandler(store, nil, nil, mock)
	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 (graceful degradation), got %d", rr.Code)
	}

	var resp DashboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.ScheduledJobs != nil {
		t.Errorf("expected nil jobs on error, got %v", resp.ScheduledJobs)
	}
}

// --- Full integration test ---

func TestDashboard_FullIntegration(t *testing.T) {
	store := newDashboardTestStore(t)
	ctx := context.Background()

	// Populate various task states.
	for _, id := range []string{"p1", "p2"} {
		_ = store.Create(ctx, makeDashboardTask(id, "pending "+id, task.StatusPending))
	}
	running := makeDashboardTask("r1", "running", task.StatusPending)
	_ = store.Create(ctx, running)
	_ = store.SetStatus(ctx, "r1", task.StatusRunning)

	completed := makeDashboardTask("c1", "done", task.StatusPending)
	_ = store.Create(ctx, completed)
	_ = store.SetStatus(ctx, "c1", task.StatusCompleted)

	gen := summary.NewGenerator(store)
	mock := &mockScheduler{
		jobs: []scheduler.JobDTO{
			{Name: "daily_summary", CronExpr: "0 8 * * *", Status: "active"},
		},
	}

	h := NewLeaderDashboardHandler(store, nil, gen, mock)
	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp DashboardResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Verify all sections populated.
	if resp.TasksByStatus.Pending != 2 {
		t.Errorf("pending = %d, want 2", resp.TasksByStatus.Pending)
	}
	if resp.TasksByStatus.InProgress != 1 {
		t.Errorf("in_progress = %d, want 1", resp.TasksByStatus.InProgress)
	}
	if resp.TasksByStatus.Completed != 1 {
		t.Errorf("completed = %d, want 1", resp.TasksByStatus.Completed)
	}
	if resp.DailySummary == nil {
		t.Fatal("expected daily summary")
	}
	if len(resp.ScheduledJobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(resp.ScheduledJobs))
	}
}

// --- HandleListTasks tests ---

func TestHandleListTasks_NoFilter(t *testing.T) {
	store := newDashboardTestStore(t)
	ctx := context.Background()

	for i, status := range []task.Status{task.StatusPending, task.StatusRunning, task.StatusCompleted} {
		id := "task-" + strconv.Itoa(i)
		tk := makeDashboardTask(id, "task "+id, task.StatusPending)
		_ = store.Create(ctx, tk)
		if status != task.StatusPending {
			_ = store.SetStatus(ctx, id, status)
		}
	}

	h := NewLeaderDashboardHandler(store, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/leader/tasks?limit=10&offset=0", nil)
	rr := httptest.NewRecorder()
	h.HandleListTasks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp TaskListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	if len(resp.Tasks) != 3 {
		t.Errorf("tasks len = %d, want 3", len(resp.Tasks))
	}
	if resp.Limit != 10 {
		t.Errorf("limit = %d, want 10", resp.Limit)
	}
	if resp.Offset != 0 {
		t.Errorf("offset = %d, want 0", resp.Offset)
	}
}

func TestHandleListTasks_StatusFilter(t *testing.T) {
	store := newDashboardTestStore(t)
	ctx := context.Background()

	_ = store.Create(ctx, makeDashboardTask("p1", "pending", task.StatusPending))
	running := makeDashboardTask("r1", "running", task.StatusPending)
	_ = store.Create(ctx, running)
	_ = store.SetStatus(ctx, "r1", task.StatusRunning)

	h := NewLeaderDashboardHandler(store, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/leader/tasks?status=running", nil)
	rr := httptest.NewRecorder()
	h.HandleListTasks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp TaskListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}
	if len(resp.Tasks) != 1 {
		t.Errorf("tasks len = %d, want 1", len(resp.Tasks))
	}
	if resp.Tasks[0].Status != "running" {
		t.Errorf("status = %q, want running", resp.Tasks[0].Status)
	}
}

func TestHandleListTasks_DefaultLimits(t *testing.T) {
	store := newDashboardTestStore(t)
	h := NewLeaderDashboardHandler(store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/leader/tasks", nil)
	rr := httptest.NewRecorder()
	h.HandleListTasks(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp TaskListResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Limit != 50 {
		t.Errorf("default limit = %d, want 50", resp.Limit)
	}
	if resp.Offset != 0 {
		t.Errorf("default offset = %d, want 0", resp.Offset)
	}
}

func TestHandleListTasks_NilHandler(t *testing.T) {
	var h *LeaderDashboardHandler
	req := httptest.NewRequest(http.MethodGet, "/api/leader/tasks", nil)
	rr := httptest.NewRecorder()
	h.HandleListTasks(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

// --- HandleUnblockTask tests ---

func TestHandleUnblockTask_Success(t *testing.T) {
	store := newDashboardTestStore(t)
	ctx := context.Background()

	tk := makeDashboardTask("blocked1", "blocked task", task.StatusPending)
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "blocked1", task.StatusFailed)

	h := NewLeaderDashboardHandler(store, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/leader/tasks/blocked1/unblock", nil)
	req.SetPathValue("id", "blocked1")
	rr := httptest.NewRecorder()
	h.HandleUnblockTask(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp UnblockResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.TaskID != "blocked1" {
		t.Errorf("task_id = %q, want blocked1", resp.TaskID)
	}
	if resp.Action != "escalated" {
		t.Errorf("action = %q, want escalated", resp.Action)
	}
}

func TestHandleUnblockTask_NotBlocked(t *testing.T) {
	store := newDashboardTestStore(t)
	ctx := context.Background()

	tk := makeDashboardTask("running1", "running task", task.StatusPending)
	_ = store.Create(ctx, tk)
	_ = store.SetStatus(ctx, "running1", task.StatusRunning)

	h := NewLeaderDashboardHandler(store, nil, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/leader/tasks/running1/unblock", nil)
	req.SetPathValue("id", "running1")
	rr := httptest.NewRecorder()
	h.HandleUnblockTask(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp UnblockResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.Action != "no_action" {
		t.Errorf("action = %q, want no_action", resp.Action)
	}
}

func TestHandleUnblockTask_NotFound(t *testing.T) {
	store := newDashboardTestStore(t)
	h := NewLeaderDashboardHandler(store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/leader/tasks/nonexistent/unblock", nil)
	req.SetPathValue("id", "nonexistent")
	rr := httptest.NewRecorder()
	h.HandleUnblockTask(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestHandleUnblockTask_EmptyID(t *testing.T) {
	store := newDashboardTestStore(t)
	h := NewLeaderDashboardHandler(store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/leader/tasks//unblock", nil)
	// PathValue("id") returns "" when not set
	rr := httptest.NewRecorder()
	h.HandleUnblockTask(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- JSON response format test ---

func TestDashboard_JSONContentType(t *testing.T) {
	store := newDashboardTestStore(t)
	h := NewLeaderDashboardHandler(store, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/leader/dashboard", nil)
	rr := httptest.NewRecorder()
	h.HandleGetDashboard(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
