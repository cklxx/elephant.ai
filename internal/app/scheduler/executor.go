package scheduler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appcontext "alex/internal/app/agent/context"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/errsanitize"
	"alex/internal/shared/notification"
	id "alex/internal/shared/utils/id"
)

var errSchedulerStopped = errors.New("scheduler stopped")

// AgentCoordinator is the subset of the coordinator interface needed by the scheduler.
type AgentCoordinator interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// Notifier routes scheduler results to external channels.
type Notifier = notification.Notifier

// executeTrigger runs a trigger's task via the agent coordinator and routes the result.
func (s *Scheduler) executeTrigger(trigger Trigger) error {
	select {
	case <-s.stopped:
		s.logger.Debug("Scheduler: skipping trigger %q because scheduler is stopped", trigger.Name)
		return errSchedulerStopped
	default:
	}

	if err := validateLarkTrigger(trigger); err != nil {
		s.logger.Warn("Scheduler: %v (trigger=%q)", err, trigger.Name)
		return err
	}

	ctx := s.buildTriggerContext(trigger)

	s.logger.Info("Scheduler: executing trigger %q (schedule=%s)", trigger.Name, trigger.Schedule)

	taskCtx, cancel := s.withTriggerTimeout(ctx)
	defer cancel()
	sessionID := id.SessionIDFromContext(ctx)
	result, err := s.coordinator.ExecuteTask(taskCtx, trigger.Task, sessionID, nil)

	s.notifyTriggerResult(ctx, trigger, formatResult(trigger, result, err))

	return err
}

// validateLarkTrigger checks Lark-specific preconditions.
func validateLarkTrigger(trigger Trigger) error {
	if !strings.EqualFold(trigger.Channel, "lark") {
		return nil
	}
	uid := strings.TrimSpace(trigger.UserID)
	if uid == "" {
		return fmt.Errorf("lark trigger requires user_id as open_id")
	}
	if !strings.HasPrefix(uid, "ou_") {
		return fmt.Errorf("lark trigger user_id must be open_id (ou_ prefix), got %q", trigger.UserID)
	}
	return nil
}

// buildTriggerContext constructs the execution context for a trigger.
func (s *Scheduler) buildTriggerContext(trigger Trigger) context.Context {
	ctx := s.runCtx
	if ctx == nil {
		ctx = context.Background()
	}
	ctx = appcontext.MarkUnattendedContext(ctx)
	if trigger.UserID != "" {
		ctx = id.WithUserID(ctx, trigger.UserID)
	}

	runID := id.NewRunID()
	sessionID := fmt.Sprintf("scheduler-%s-%s", trigger.Name, runID)
	ctx = id.WithSessionID(ctx, sessionID)
	ctx = id.WithRunID(ctx, runID)
	return ctx
}

func (s *Scheduler) withTriggerTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.config.TriggerTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, s.config.TriggerTimeout)
}

func (s *Scheduler) notifyTriggerResult(ctx context.Context, trigger Trigger, content string) {
	if s.notifier == nil || trigger.Channel == "" {
		return
	}
	target := notification.Target{Channel: trigger.Channel, ChatID: trigger.ChatID}
	if err := s.notifier.Send(ctx, target, content); err != nil {
		s.logger.Warn("Scheduler: failed to send notification for %q: %v", trigger.Name, err)
	}
}

// formatResult produces a human-readable summary of the trigger execution.
func formatResult(trigger Trigger, result *agent.TaskResult, err error) string {
	if err != nil {
		return fmt.Sprintf("定时任务「%s」执行失败：%s", trigger.Name, errsanitize.ForUser(err.Error()))
	}
	if result == nil {
		return fmt.Sprintf("定时任务「%s」已完成。", trigger.Name)
	}
	answer := strings.TrimSpace(result.Answer)
	if answer == "" {
		return fmt.Sprintf("定时任务「%s」已完成。", trigger.Name)
	}
	return fmt.Sprintf("定时任务「%s」完成：\n%s", trigger.Name, answer)
}
