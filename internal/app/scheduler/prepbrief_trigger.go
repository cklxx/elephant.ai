package scheduler

import (
	"context"
	"strings"
	"time"

	"alex/internal/domain/calendar"
)

const (
	prepBriefTriggerName = "__prep_brief__"

	// defaultPrepBriefLookAhead is the window before a meeting starts
	// during which a prep brief is triggered.
	defaultPrepBriefLookAhead = 30 * time.Minute
)

// registerPrepBriefJob registers a cron job that generates and sends 1:1
// meeting prep briefs. When a CalendarPort is configured, each tick checks
// for upcoming 1:1 meetings within the look-ahead window and triggers a
// brief only when one is found. Without a CalendarPort, it falls back to
// the original fixed-schedule behavior.
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
	calPort := s.config.CalendarPort

	entryID, err := s.cron.AddFunc(schedule, func() {
		if calPort != nil {
			s.handleCalendarDrivenPrepBrief(ctx, calPort, svc, memberID, schedule)
		} else {
			s.handleFixedPrepBrief(ctx, svc, memberID, schedule)
		}
	})
	if err != nil {
		s.logger.Warn("Scheduler: failed to register prep brief: %v", err)
		return
	}
	s.entryIDs[prepBriefTriggerName] = entryID
	if calPort != nil {
		s.logger.Info("Prep brief registered (schedule=%s, calendar-driven)", schedule)
	} else {
		s.logger.Info("Prep brief registered (schedule=%s, fixed)", schedule)
	}
}

// handleCalendarDrivenPrepBrief checks for upcoming 1:1 meetings and only
// generates a brief when at least one is found within the look-ahead window.
func (s *Scheduler) handleCalendarDrivenPrepBrief(
	ctx context.Context,
	calPort calendar.CalendarPort,
	svc PrepBriefService,
	memberID, schedule string,
) {
	meetings, err := calPort.ListUpcoming1on1s(ctx, memberID, defaultPrepBriefLookAhead)
	if err != nil {
		s.logger.Warn("Prep brief: calendar lookup failed: %v (falling back to fixed)", err)
		s.handleFixedPrepBrief(ctx, svc, memberID, schedule)
		return
	}

	if len(meetings) == 0 {
		s.logger.Info("Prep brief: no upcoming 1:1s in next %s, skipping", defaultPrepBriefLookAhead)
		return
	}

	s.logger.Info("Prep brief triggered: %d upcoming 1:1(s) for %s (first: %s at %s)",
		len(meetings), memberID, meetings[0].Title, meetings[0].StartTime.Format("15:04"))
	err = svc.GenerateAndSend(ctx, memberID)
	s.recordLeaderResult(prepBriefTriggerName, err)
	if err != nil {
		s.logger.Warn("Prep brief failed: %v", err)
	}
}

// handleFixedPrepBrief sends a prep brief unconditionally (original behavior).
func (s *Scheduler) handleFixedPrepBrief(ctx context.Context, svc PrepBriefService, memberID, schedule string) {
	s.logger.Info("Prep brief triggered (schedule=%s, member=%s)", schedule, memberID)
	err := svc.GenerateAndSend(ctx, memberID)
	s.recordLeaderResult(prepBriefTriggerName, err)
	if err != nil {
		s.logger.Warn("Prep brief failed: %v", err)
	}
}
