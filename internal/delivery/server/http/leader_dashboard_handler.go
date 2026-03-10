package http

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"alex/internal/app/blocker"
	"alex/internal/app/scheduler"
	"alex/internal/app/summary"
	"alex/internal/domain/task"
)

// LeaderDashboardHandler serves the leader agent dashboard API.
type LeaderDashboardHandler struct {
	taskStore task.Store
	radar     *blocker.Radar
	summaryGen *summary.Generator
	scheduler  schedulerJobLister
}

// schedulerJobLister is the subset of the scheduler API needed by the dashboard.
type schedulerJobLister interface {
	ListJobs(ctx context.Context) ([]scheduler.JobDTO, error)
}

// NewLeaderDashboardHandler creates a dashboard handler. Returns nil if
// the required task store is not available.
func NewLeaderDashboardHandler(
	store task.Store,
	radar *blocker.Radar,
	summaryGen *summary.Generator,
	sched schedulerJobLister,
) *LeaderDashboardHandler {
	if store == nil {
		return nil
	}
	return &LeaderDashboardHandler{
		taskStore:  store,
		radar:      radar,
		summaryGen: summaryGen,
		scheduler:  sched,
	}
}

// DashboardResponse is the JSON response for GET /api/leader/dashboard.
type DashboardResponse struct {
	TasksByStatus   TaskStatusCounts  `json:"tasks_by_status"`
	RecentBlockers  []BlockerAlert    `json:"recent_blockers"`
	DailySummary    *DailySummaryDTO  `json:"daily_summary,omitempty"`
	ScheduledJobs   []ScheduledJobDTO `json:"scheduled_jobs,omitempty"`
}

// TaskStatusCounts holds counts of tasks by status.
type TaskStatusCounts struct {
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Completed  int `json:"completed"`
}

// BlockerAlert is a simplified blocker alert for the dashboard.
type BlockerAlert struct {
	TaskID      string `json:"task_id"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
	Detail      string `json:"detail"`
	Status      string `json:"status"`
}

// DailySummaryDTO is the dashboard representation of the daily summary.
type DailySummaryDTO struct {
	NewTasks       int     `json:"new_tasks"`
	Completed      int     `json:"completed"`
	InProgress     int     `json:"in_progress"`
	Blocked        int     `json:"blocked"`
	CompletionRate float64 `json:"completion_rate"`
}

// ScheduledJobDTO is a simplified scheduled job for the dashboard.
type ScheduledJobDTO struct {
	Name     string    `json:"name"`
	CronExpr string   `json:"cron_expr"`
	Status   string    `json:"status"`
	NextRun  time.Time `json:"next_run,omitempty"`
	LastRun  time.Time `json:"last_run,omitempty"`
}

// HandleGetDashboard handles GET /api/leader/dashboard.
func (h *LeaderDashboardHandler) HandleGetDashboard(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()

	resp := DashboardResponse{}

	// 1. Task counts by status.
	resp.TasksByStatus = h.gatherTaskCounts(ctx)

	// 2. Recent blocker alerts (last 24h).
	resp.RecentBlockers = h.gatherBlockerAlerts(ctx)

	// 3. Latest daily summary.
	resp.DailySummary = h.gatherDailySummary(ctx)

	// 4. Scheduled jobs with next run times.
	resp.ScheduledJobs = h.gatherScheduledJobs(ctx)

	writeJSON(w, http.StatusOK, resp)
}

func (h *LeaderDashboardHandler) gatherTaskCounts(ctx context.Context) TaskStatusCounts {
	var counts TaskStatusCounts

	if pending, err := h.taskStore.ListByStatus(ctx, task.StatusPending); err == nil {
		counts.Pending = len(pending)
	}
	if running, err := h.taskStore.ListByStatus(ctx, task.StatusRunning); err == nil {
		counts.InProgress = len(running)
	}
	if failed, err := h.taskStore.ListByStatus(ctx, task.StatusFailed); err == nil {
		counts.Blocked = len(failed)
	}
	if waiting, err := h.taskStore.ListByStatus(ctx, task.StatusWaitingInput); err == nil {
		counts.Blocked += len(waiting)
	}
	if completed, err := h.taskStore.ListByStatus(ctx, task.StatusCompleted); err == nil {
		counts.Completed = len(completed)
	}

	return counts
}

func (h *LeaderDashboardHandler) gatherBlockerAlerts(ctx context.Context) []BlockerAlert {
	if h.radar == nil {
		return nil
	}

	result, err := h.radar.Scan(ctx)
	if err != nil || len(result.Alerts) == 0 {
		return nil
	}

	alerts := make([]BlockerAlert, 0, len(result.Alerts))
	for _, a := range result.Alerts {
		desc := a.Task.Description
		if desc == "" {
			desc = a.Task.TaskID
		}
		alerts = append(alerts, BlockerAlert{
			TaskID:      a.Task.TaskID,
			Description: desc,
			Reason:      string(a.Reason),
			Detail:      a.Detail,
			Status:      string(a.Task.Status),
		})
	}
	return alerts
}

func (h *LeaderDashboardHandler) gatherDailySummary(ctx context.Context) *DailySummaryDTO {
	if h.summaryGen == nil {
		return nil
	}

	s, err := h.summaryGen.Generate(ctx)
	if err != nil {
		return nil
	}

	return &DailySummaryDTO{
		NewTasks:       len(s.New),
		Completed:      len(s.Completed),
		InProgress:     len(s.InProgress),
		Blocked:        len(s.Blocked),
		CompletionRate: s.CompletionRate,
	}
}

// TaskSummaryDTO is a lightweight task representation for list responses.
type TaskSummaryDTO struct {
	TaskID           string    `json:"task_id"`
	Description      string    `json:"description"`
	Status           string    `json:"status"`
	UserID           string    `json:"user_id"`
	CurrentIteration int       `json:"current_iteration"`
	TokensUsed       int       `json:"tokens_used"`
	Error            string    `json:"error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// TaskListResponse is the JSON response for GET /api/leader/tasks.
type TaskListResponse struct {
	Tasks  []TaskSummaryDTO `json:"tasks"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

// UnblockRequest is the optional JSON body for POST /api/leader/tasks/{id}/unblock.
type UnblockRequest struct {
	Reason string `json:"reason"`
}

// UnblockResponse is the JSON response for POST /api/leader/tasks/{id}/unblock.
type UnblockResponse struct {
	TaskID string `json:"task_id"`
	Action string `json:"action"`
	Detail string `json:"detail"`
}

func taskToSummary(t *task.Task) TaskSummaryDTO {
	return TaskSummaryDTO{
		TaskID:           t.TaskID,
		Description:      t.Description,
		Status:           string(t.Status),
		UserID:           t.UserID,
		CurrentIteration: t.CurrentIteration,
		TokensUsed:       t.TokensUsed,
		Error:            t.Error,
		CreatedAt:        t.CreatedAt,
		UpdatedAt:        t.UpdatedAt,
	}
}

// HandleListTasks handles GET /api/leader/tasks.
func (h *LeaderDashboardHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}
	ctx := r.Context()

	// Parse query params.
	statusFilter := r.URL.Query().Get("status")
	limit := 50
	offset := 0

	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 200 {
		limit = 200
	}

	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	var resp TaskListResponse
	resp.Limit = limit
	resp.Offset = offset

	if statusFilter != "" {
		// Filter by status — store returns all matching, apply pagination in-memory.
		tasks, err := h.taskStore.ListByStatus(ctx, task.Status(statusFilter))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tasks"})
			return
		}
		resp.Total = len(tasks)

		// Apply offset/limit.
		if offset >= len(tasks) {
			tasks = nil
		} else {
			end := offset + limit
			if end > len(tasks) {
				end = len(tasks)
			}
			tasks = tasks[offset:end]
		}

		resp.Tasks = make([]TaskSummaryDTO, 0, len(tasks))
		for _, t := range tasks {
			resp.Tasks = append(resp.Tasks, taskToSummary(t))
		}
	} else {
		// No filter — use paginated List.
		tasks, total, err := h.taskStore.List(ctx, limit, offset)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tasks"})
			return
		}
		resp.Total = total
		resp.Tasks = make([]TaskSummaryDTO, 0, len(tasks))
		for _, t := range tasks {
			resp.Tasks = append(resp.Tasks, taskToSummary(t))
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleUnblockTask handles POST /api/leader/tasks/{id}/unblock.
func (h *LeaderDashboardHandler) HandleUnblockTask(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.NotFound(w, r)
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing task ID"})
		return
	}

	ctx := r.Context()

	// Parse optional request body.
	var body UnblockRequest
	if r.Body != nil && r.ContentLength != 0 {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}

	// Look up the task.
	t, err := h.taskStore.Get(ctx, id)
	if err != nil || t == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "task not found"})
		return
	}

	// Determine if the task is actually blocked.
	isBlocked := t.Status == task.StatusFailed || t.Status == task.StatusWaitingInput
	if !isBlocked {
		writeJSON(w, http.StatusOK, UnblockResponse{
			TaskID: id,
			Action: "no_action",
			Detail: "task is not blocked (status: " + string(t.Status) + ")",
		})
		return
	}

	detail := "unblock request escalated for task " + id
	if body.Reason != "" {
		detail += "; reason: " + body.Reason
	}

	writeJSON(w, http.StatusOK, UnblockResponse{
		TaskID: id,
		Action: "escalated",
		Detail: detail,
	})
}

func (h *LeaderDashboardHandler) gatherScheduledJobs(ctx context.Context) []ScheduledJobDTO {
	if h.scheduler == nil {
		return nil
	}

	jobs, err := h.scheduler.ListJobs(ctx)
	if err != nil {
		return nil
	}

	dtos := make([]ScheduledJobDTO, 0, len(jobs))
	for _, j := range jobs {
		dtos = append(dtos, ScheduledJobDTO{
			Name:     j.Name,
			CronExpr: j.CronExpr,
			Status:   j.Status,
			NextRun:  j.NextRun,
			LastRun:  j.LastRun,
		})
	}
	return dtos
}
