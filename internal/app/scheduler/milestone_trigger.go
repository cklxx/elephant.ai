package scheduler

import (
	"context"
	"strings"
)

const milestoneTriggerName = "__milestone_checkin__"

// registerMilestoneCheckinJob registers a cron job that periodically calls
// the milestone check-in service. Must be called with s.mu held.
func (s *Scheduler) registerMilestoneCheckinJob(ctx context.Context) {
	cfg := s.config.MilestoneCheckin
	svc := s.config.MilestoneService
	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "0 */1 * * *"
	}
	s.registerLeaderJob(ctx, leaderJobDef{
		name:         milestoneTriggerName,
		enabled:      cfg.Enabled,
		service:      svc,
		serviceLabel: "Milestone check-in",
		schedule:     schedule,
		run:          func(ctx context.Context) error { return svc.SendCheckin(ctx) },
	})
}
