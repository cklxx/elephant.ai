package scheduler

import (
	"context"
	"fmt"
)

const (
	calendarTriggerName     = "calendar:reminder"
	defaultCalendarSchedule = "*/15 * * * *"
	defaultLookAheadMinutes = 120
)

// registerCalendarTrigger creates and registers the calendar reminder trigger
// if enabled in config. Must be called with s.mu held.
func (s *Scheduler) registerCalendarTrigger(ctx context.Context) {
	cfg := s.config.CalendarReminder
	if !cfg.Enabled {
		return
	}

	schedule := cfg.Schedule
	if schedule == "" {
		schedule = defaultCalendarSchedule
	}

	lookAhead := cfg.LookAheadMinutes
	if lookAhead <= 0 {
		lookAhead = defaultLookAheadMinutes
	}

	task := fmt.Sprintf(
		"Check upcoming calendar events and tasks, remind me of anything in the next %d minutes that needs attention.",
		lookAhead,
	)

	trigger := Trigger{
		Name:     calendarTriggerName,
		Schedule: schedule,
		Task:     task,
		Channel:  cfg.Channel,
		UserID:   cfg.UserID,
		ChatID:   cfg.ChatID,
	}

	if err := s.registerTriggerLocked(ctx, trigger); err != nil {
		s.logger.Warn("Scheduler: failed to register calendar reminder trigger: %v", err)
	}
}
