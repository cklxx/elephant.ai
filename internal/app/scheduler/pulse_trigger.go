package scheduler

import (
	"context"
	"strings"
)

const weeklyPulseTriggerName = "__weekly_pulse__"

// registerWeeklyPulseJob registers a cron job that periodically generates
// and sends the weekly pulse digest. Must be called with s.mu held.
func (s *Scheduler) registerWeeklyPulseJob(ctx context.Context) {
	cfg := s.config.WeeklyPulse
	svc := s.config.WeeklyPulseService
	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "0 9 * * 1"
	}
	s.registerLeaderJob(ctx, leaderJobDef{
		name:         weeklyPulseTriggerName,
		enabled:      cfg.Enabled,
		service:      svc,
		serviceLabel: "Weekly pulse",
		schedule:     schedule,
		run:          func(ctx context.Context) error { return svc.GenerateAndSend(ctx) },
	})
}
