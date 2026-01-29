package react

import (
	"fmt"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
)

// injectBackgroundNotifications drains completed background tasks and injects
// system messages into the conversation so the LLM can decide when to collect
// results.
func (r *reactRuntime) injectBackgroundNotifications() {
	if r.bgManager == nil {
		return
	}

	completed := r.bgManager.DrainCompletions()
	if len(completed) == 0 {
		return
	}

	for _, taskID := range completed {
		summaries := r.bgManager.Status([]string{taskID})
		if len(summaries) == 0 {
			continue
		}
		s := summaries[0]

		var msg string
		switch s.Status {
		case agent.BackgroundTaskStatusCompleted:
			msg = fmt.Sprintf(
				"[Background Task Completed] task_id=%q description=%q\nUse bg_collect(task_ids=[%q]) to retrieve the full result.",
				s.ID, s.Description, s.ID,
			)
		case agent.BackgroundTaskStatusFailed:
			msg = fmt.Sprintf(
				"[Background Task Failed] task_id=%q description=%q error=%q\nUse bg_collect(task_ids=[%q]) to see details.",
				s.ID, s.Description, s.Error, s.ID,
			)
		case agent.BackgroundTaskStatusCancelled:
			msg = fmt.Sprintf(
				"[Background Task Cancelled] task_id=%q description=%q\nUse bg_collect(task_ids=[%q]) to see details.",
				s.ID, s.Description, s.ID,
			)
		default:
			continue
		}

		r.state.Messages = append(r.state.Messages, ports.Message{
			Role:    "user",
			Content: msg,
			Source:  ports.MessageSourceSystemPrompt,
		})

		r.engine.logger.Info("Injected background notification for task %q (status=%s)", taskID, s.Status)

		// Emit domain event.
		r.emitBackgroundCompletedEvent(s)
	}
}

// emitBackgroundCompletedEvent emits a BackgroundTaskCompletedEvent.
func (r *reactRuntime) emitBackgroundCompletedEvent(s agent.BackgroundTaskSummary) {
	results := r.bgManager.Collect([]string{s.ID}, false, 0)
	if len(results) == 0 {
		return
	}
	result := results[0]

	r.engine.emitEvent(&domain.BackgroundTaskCompletedEvent{
		BaseEvent:   r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
		TaskID:      result.ID,
		Description: result.Description,
		Status:      string(result.Status),
		Answer:      result.Answer,
		Error:       result.Error,
		Duration:    result.Duration,
		Iterations:  result.Iterations,
		TokensUsed:  result.TokensUsed,
	})
}

// cleanupBackgroundTasks waits briefly for pending background tasks and then
// shuts down the manager.
func (r *reactRuntime) cleanupBackgroundTasks() {
	if r.bgManager == nil {
		return
	}
	if r.bgManager.TaskCount() == 0 {
		r.bgManager.Shutdown()
		return
	}

	r.engine.logger.Info("Waiting up to 10s for %d background task(s) to complete...", r.bgManager.TaskCount())
	r.bgManager.AwaitAll(10 * time.Second)
	r.bgManager.Shutdown()
}
