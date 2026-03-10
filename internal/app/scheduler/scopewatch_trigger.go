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
// external work items for scope drift. Must be called with s.mu held.
func (s *Scheduler) registerScopeWatchJob(ctx context.Context) {
	cfg := s.config.ScopeWatch
	svc := s.config.ScopeWatchService
	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "*/30 * * * *"
	}
	s.registerLeaderJob(ctx, leaderJobDef{
		name:         scopeWatchTriggerName,
		enabled:      cfg.Enabled,
		service:      svc,
		serviceLabel: "Scope watch",
		schedule:     schedule,
		run:          func(ctx context.Context) error { return svc.NotifyScopeChanges(ctx) },
	})
}
