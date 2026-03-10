package http

import (
	"context"
	"net/http"
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
