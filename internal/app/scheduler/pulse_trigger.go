package scheduler

import (
	"context"
	"strings"
)

const weeklyPulseTriggerName = "__weekly_pulse__"

// registerWeeklyPulseJob registers a cron job that periodically generates
// and sends the weekly pulse digest. Like the milestone check-in, this
// does not dispatch an agent task — it queries the task store directly.
// Must be called with s.mu held.
func (s *Scheduler) registerWeeklyPulseJob(ctx context.Context) {
	cfg := s.config.WeeklyPulse
	if !cfg.Enabled {
		return
	}
	svc := s.config.WeeklyPulseService
	if svc == nil {
		s.logger.Warn("Scheduler: weekly pulse enabled but no service wired; skipping")
		return
	}

	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "0 9 * * 1" // Monday 9am default
	}

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.logger.Info("Weekly pulse triggered (schedule=%s)", schedule)
		err := svc.GenerateAndSend(ctx)
		s.recordLeaderResult(weeklyPulseTriggerName, err)
		if err != nil {
			s.logger.Warn("Weekly pulse failed: %v", err)
		}
	})
	if err != nil {
		s.logger.Warn("Scheduler: failed to register weekly pulse: %v", err)
		return
	}
	s.entryIDs[weeklyPulseTriggerName] = entryID
	s.logger.Info("Weekly pulse registered (schedule=%s)", schedule)
}
