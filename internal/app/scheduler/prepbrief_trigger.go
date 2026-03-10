package scheduler

import (
	"context"
	"strings"
)

const prepBriefTriggerName = "__prep_brief__"

// registerPrepBriefJob registers a cron job that periodically generates and
// sends 1:1 meeting prep briefs. Like the other direct-service triggers,
// this calls the service without dispatching an agent task.
// Must be called with s.mu held.
func (s *Scheduler) registerPrepBriefJob(ctx context.Context) {
	cfg := s.config.PrepBrief
	if !cfg.Enabled {
		return
	}
	svc := s.config.PrepBriefService
	if svc == nil {
		s.logger.Warn("Scheduler: prep brief enabled but no service wired; skipping")
		return
	}

	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "30 8 * * 1-5" // weekdays 8:30am default
	}

	memberID := strings.TrimSpace(cfg.MemberID)

	entryID, err := s.cron.AddFunc(schedule, func() {
		s.logger.Info("Prep brief triggered (schedule=%s, member=%s)", schedule, memberID)
		if err := svc.GenerateAndSend(ctx, memberID); err != nil {
			s.logger.Warn("Prep brief failed: %v", err)
		}
	})
	if err != nil {
		s.logger.Warn("Scheduler: failed to register prep brief: %v", err)
		return
	}
	s.entryIDs[prepBriefTriggerName] = entryID
	s.logger.Info("Prep brief registered (schedule=%s)", schedule)
}
