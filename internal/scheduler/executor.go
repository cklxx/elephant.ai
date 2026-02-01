package scheduler

import (
	"context"
	"fmt"

	agent "alex/internal/agent/ports/agent"
	id "alex/internal/utils/id"
)

// AgentCoordinator is the subset of the coordinator interface needed by the scheduler.
type AgentCoordinator interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// Notifier routes scheduler results to external channels.
type Notifier interface {
	SendLark(ctx context.Context, chatID string, content string) error
	SendMoltbook(ctx context.Context, content string) error
}

// executeTrigger runs a trigger's task via the agent coordinator and routes the result.
func (s *Scheduler) executeTrigger(trigger Trigger) {
	ctx := context.Background()
	if trigger.UserID != "" {
		ctx = id.WithUserID(ctx, trigger.UserID)
	}

	runID := id.NewRunID()
	sessionID := fmt.Sprintf("scheduler-%s-%s", trigger.Name, runID)
	ctx = id.WithSessionID(ctx, sessionID)
	ctx = id.WithRunID(ctx, runID)

	s.logger.Info("Scheduler: executing trigger %q (schedule=%s)", trigger.Name, trigger.Schedule)

	taskCtx := ctx
	if s.config.TriggerTimeout > 0 {
		var cancel context.CancelFunc
		taskCtx, cancel = context.WithTimeout(ctx, s.config.TriggerTimeout)
		defer cancel()
	}

	result, err := s.coordinator.ExecuteTask(taskCtx, trigger.Task, sessionID, nil)

	content := formatResult(trigger, result, err)

	if s.notifier != nil {
		switch trigger.Channel {
		case "lark":
			if trigger.ChatID != "" {
				if sendErr := s.notifier.SendLark(ctx, trigger.ChatID, content); sendErr != nil {
					s.logger.Warn("Scheduler: failed to send Lark notification for %q: %v", trigger.Name, sendErr)
				}
			}
		case "moltbook":
			if sendErr := s.notifier.SendMoltbook(ctx, content); sendErr != nil {
				s.logger.Warn("Scheduler: failed to send Moltbook notification for %q: %v", trigger.Name, sendErr)
			}
		}
	}
}

// formatResult produces a human-readable summary of the trigger execution.
func formatResult(trigger Trigger, result *agent.TaskResult, err error) string {
	if err != nil {
		return fmt.Sprintf("Scheduled task '%s' failed: %v", trigger.Name, err)
	}
	if result == nil {
		return fmt.Sprintf("Scheduled task '%s' completed (no result).", trigger.Name)
	}
	return result.Answer
}
