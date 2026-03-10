package scheduler

import (
	"context"
	"strings"
)

const milestoneTriggerName = "__milestone_checkin__"

// registerMilestoneCheckinJob registers a cron job that periodically calls
// the milestone check-in service. Unlike other triggers, this does not
// dispatch an agent task — it queries the task store directly and sends
// a formatted summary via the notification system.
// Must be called with s.mu held.
func (s *Scheduler) registerMilestoneCheckinJob(ctx context.Context) {
	cfg := s.config.MilestoneCheckin
	if !cfg.Enabled {
		return
	}
	svc := s.config.MilestoneService
	if svc == nil {
		s.logger.Warn("Scheduler: milestone check-in enabled but no service wired; skipping")
		return
	}

	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "0 */1 * * *" // hourly default
	}

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.logger.Info("Milestone check-in triggered (schedule=%s)", schedule)
		if err := svc.SendCheckin(ctx); err != nil {
			s.logger.Warn("Milestone check-in failed: %v", err)
		}
	})
	if err != nil {
		s.logger.Warn("Scheduler: failed to register milestone check-in: %v", err)
		return
	}
	s.entryIDs[milestoneTriggerName] = entryID
	s.logger.Info("Milestone check-in registered (schedule=%s)", schedule)
}
