package scheduler

import (
	"context"
	"strings"
)

const blockerRadarTriggerName = "__blocker_radar__"

// registerBlockerRadarJob registers a cron job that periodically runs the
// blocker radar scan. Must be called with s.mu held.
func (s *Scheduler) registerBlockerRadarJob(ctx context.Context) {
	cfg := s.config.BlockerRadar
	svc := s.config.BlockerRadarService
	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "0 */4 * * *"
	}
	s.registerLeaderJob(ctx, leaderJobDef{
		name:         blockerRadarTriggerName,
		enabled:      cfg.Enabled,
		service:      svc,
		serviceLabel: "Blocker radar",
		schedule:     schedule,
		run:          func(ctx context.Context) error { return svc.NotifyBlockedTasks(ctx) },
	})
}
