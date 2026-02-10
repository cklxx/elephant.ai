package scheduler

import (
	"context"
	"strings"
)

const (
	heartbeatTriggerName = "__heartbeat__"
	defaultHeartbeatTask = "Read HEARTBEAT.md if it exists. Follow it strictly. If nothing needs attention, reply HEARTBEAT_OK."
)

// registerHeartbeatTrigger creates and registers the heartbeat trigger
// when enabled in config. Must be called with s.mu held.
func (s *Scheduler) registerHeartbeatTrigger(ctx context.Context) {
	cfg := s.config.Heartbeat
	if !cfg.Enabled {
		return
	}

	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		schedule = "*/30 * * * *"
	}
	task := strings.TrimSpace(cfg.Task)
	if task == "" {
		task = defaultHeartbeatTask
	}

	trigger := Trigger{
		Name:     heartbeatTriggerName,
		Schedule: schedule,
		Task:     task,
		Channel:  strings.TrimSpace(cfg.Channel),
		UserID:   strings.TrimSpace(cfg.UserID),
		ChatID:   strings.TrimSpace(cfg.ChatID),
	}

	if err := s.registerTriggerLocked(ctx, trigger); err != nil {
		s.logger.Warn("Scheduler: failed to register heartbeat trigger: %v", err)
	}
}

