package app

import (
	"context"
	"time"

	"alex/internal/app/scheduler"
	"alex/internal/delivery/server/ports"
)

// SchedulerHealthProvider is the interface a scheduler must satisfy for health checking.
type SchedulerHealthProvider interface {
	Running() bool
	LeaderJobsHealth() []scheduler.LeaderJobStatus
}

// SchedulerProbe checks the health of the leader agent scheduler and its
// registered cron jobs (blocker_radar, weekly_pulse, milestone, prep_brief).
type SchedulerProbe struct {
	provider     SchedulerHealthProvider
	overdueGrace time.Duration // how long past NextRun before a job is considered overdue
}

// NewSchedulerProbe creates a scheduler health probe. The provider may be nil
// (e.g. when the scheduler is disabled), in which case the probe reports "disabled".
// overdueGrace controls how long after a job's NextRun we wait before flagging
// it as overdue (default 10 minutes if zero).
func NewSchedulerProbe(provider SchedulerHealthProvider, overdueGrace time.Duration) *SchedulerProbe {
	if overdueGrace <= 0 {
		overdueGrace = 10 * time.Minute
	}
	return &SchedulerProbe{provider: provider, overdueGrace: overdueGrace}
}

// NewSchedulerProbeFromScheduler creates a scheduler health probe from a
// *scheduler.Scheduler, handling the nil case safely.
func NewSchedulerProbeFromScheduler(sched *scheduler.Scheduler, overdueGrace time.Duration) *SchedulerProbe {
	var provider SchedulerHealthProvider
	if sched != nil {
		provider = sched
	}
	return NewSchedulerProbe(provider, overdueGrace)
}

// Check returns the aggregate health of the scheduler and per-job details.
func (p *SchedulerProbe) Check(ctx context.Context) ports.ComponentHealth {
	if p.provider == nil {
		return ports.ComponentHealth{
			Name:    "scheduler",
			Status:  ports.HealthStatusDisabled,
			Message: "Scheduler not configured",
		}
	}

	if !p.provider.Running() {
		return ports.ComponentHealth{
			Name:    "scheduler",
			Status:  ports.HealthStatusNotReady,
			Message: "Scheduler is not running",
		}
	}

	statuses := p.provider.LeaderJobsHealth()
	now := time.Now()

	allHealthy := true
	jobDetails := make(map[string]interface{}, len(statuses))
	for _, s := range statuses {
		detail := map[string]interface{}{
			"registered": s.Registered,
			"healthy":    s.Healthy,
		}
		if !s.LastRun.IsZero() {
			detail["last_run"] = s.LastRun.Format(time.RFC3339)
		}
		if !s.NextRun.IsZero() {
			detail["next_run"] = s.NextRun.Format(time.RFC3339)
		}
		if s.LastError != "" {
			detail["last_error"] = s.LastError
			detail["healthy"] = false
			allHealthy = false
		}

		// Check overdue: registered job whose NextRun is in the past beyond grace.
		if s.Registered && !s.NextRun.IsZero() && now.After(s.NextRun.Add(p.overdueGrace)) {
			detail["overdue"] = true
			detail["healthy"] = false
			allHealthy = false
		}

		if !s.Healthy {
			allHealthy = false
		}

		jobDetails[scheduler.DisplayName(s.Name)] = detail
	}

	status := ports.HealthStatusReady
	if !allHealthy {
		status = ports.HealthStatusNotReady
	}

	return ports.ComponentHealth{
		Name:    "scheduler",
		Status:  status,
		Message: scheduler.HealthSummary(statuses),
		Details: jobDetails,
	}
}
