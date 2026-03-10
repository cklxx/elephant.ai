package scheduler

import (
	"context"
	"strings"
)

const scopeWatchTriggerName = "__scope_watch__"

// ScopeWatchService is the interface satisfied by scopewatch.Detector.
type ScopeWatchService interface {
	NotifyScopeChanges(ctx context.Context) error
}

// registerScopeWatchJob registers a cron job that periodically scans
// external work items for scope drift and sends notifications.
// Must be called with s.mu held.
func (s *Scheduler) registerScopeWatchJob(ctx context.Context) {
	cfg := s.config.ScopeWatch
	if !cfg.Enabled {
		return
	}
	svc := s.config.ScopeWatchService
	if svc == nil {
		s.logger.Warn("Scheduler: scope watch enabled but no service wired; skipping")
		return
	}

	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "*/30 * * * *" // every 30 minutes default
	}

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.logger.Info("Scope watch triggered (schedule=%s)", schedule)
		err := svc.NotifyScopeChanges(ctx)
		s.recordLeaderResult(scopeWatchTriggerName, err)
		if err != nil {
			s.logger.Warn("Scope watch failed: %v", err)
		}
	})
	if err != nil {
		s.logger.Warn("Scheduler: failed to register scope watch: %v", err)
		return
	}
	s.entryIDs[scopeWatchTriggerName] = entryID
	s.logger.Info("Scope watch registered (schedule=%s)", schedule)
}
