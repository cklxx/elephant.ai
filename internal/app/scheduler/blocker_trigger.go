package scheduler

import (
	"context"
	"strings"
)

const blockerRadarTriggerName = "__blocker_radar__"

// registerBlockerRadarJob registers a cron job that periodically runs the
// blocker radar scan and sends notifications for stuck tasks. Like the
// milestone and pulse triggers, this calls the service directly without
// dispatching an agent task.
// Must be called with s.mu held.
func (s *Scheduler) registerBlockerRadarJob(ctx context.Context) {
	cfg := s.config.BlockerRadar
	if !cfg.Enabled {
		return
	}
	svc := s.config.BlockerRadarService
	if svc == nil {
		s.logger.Warn("Scheduler: blocker radar enabled but no service wired; skipping")
		return
	}

	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "0 */4 * * *" // every 4 hours default
	}

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.logger.Info("Blocker radar triggered (schedule=%s)", schedule)
		err := svc.NotifyBlockedTasks(ctx)
		s.recordLeaderResult(blockerRadarTriggerName, err)
		if err != nil {
			s.logger.Warn("Blocker radar failed: %v", err)
		}
	})
	if err != nil {
		s.logger.Warn("Scheduler: failed to register blocker radar: %v", err)
		return
	}
	s.entryIDs[blockerRadarTriggerName] = entryID
	s.logger.Info("Blocker radar registered (schedule=%s)", schedule)
}
