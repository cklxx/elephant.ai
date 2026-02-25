package bridge

import (
	"context"
	"fmt"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	taskdomain "alex/internal/domain/task"
	"alex/internal/shared/logging"
)

// ResumeAction describes what to do with an orphaned bridge.
type ResumeAction string

const (
	// ResumeAdopt means the bridge process is still running — tail its output.
	ResumeAdopt ResumeAction = "adopt"
	// ResumeHarvest means the bridge completed — read remaining events and store result.
	ResumeHarvest ResumeAction = "harvest"
	// ResumeRetryWithContext means the bridge died after partial progress — retry with context.
	ResumeRetryWithContext ResumeAction = "retry_with_context"
	// ResumeRetryFresh means the bridge died with no progress — retry from scratch.
	ResumeRetryFresh ResumeAction = "retry_fresh"
	// ResumeMarkFailed means the bridge is unrecoverable — mark task as failed.
	ResumeMarkFailed ResumeAction = "mark_failed"
)

// ClassifyOrphan determines the appropriate resume action for an orphaned bridge.
func ClassifyOrphan(orphan OrphanedBridge, task *taskdomain.Task) ResumeAction {
	if orphan.IsRunning && !orphan.HasDone {
		return ResumeAdopt
	}
	if orphan.HasDone {
		return ResumeHarvest
	}
	// Process is dead, no .done sentinel.
	if task != nil && task.BridgeMeta != nil && len(task.BridgeMeta.FilesTouched) > 0 {
		return ResumeRetryWithContext
	}
	if task != nil && task.Prompt != "" {
		return ResumeRetryFresh
	}
	return ResumeMarkFailed
}

// Resumer handles bridge orphan adoption and task resumption on restart.
type Resumer struct {
	store    taskdomain.Store
	executor *Executor
	logger   logging.Logger
}

// NewResumer creates a bridge resumer.
func NewResumer(store taskdomain.Store, executor *Executor, logger logging.Logger) *Resumer {
	return &Resumer{
		store:    store,
		executor: executor,
		logger:   logging.OrNop(logger),
	}
}

// ResumeResult captures the outcome of a resume attempt.
type ResumeResult struct {
	TaskID string
	Action ResumeAction
	Error  error
}

// ResumeOrphans scans the working directory for orphaned bridges and processes them.
// Returns the number of tasks successfully resumed/harvested.
func (r *Resumer) ResumeOrphans(ctx context.Context, workDir string) []ResumeResult {
	orphans := DetectOrphanedBridges(workDir)
	if len(orphans) == 0 {
		return nil
	}

	r.logger.Info("[BridgeResumer] Found %d orphaned bridge(s)", len(orphans))

	var results []ResumeResult
	for _, orphan := range orphans {
		result := r.processOrphan(ctx, orphan, workDir)
		results = append(results, result)
	}

	return results
}

// processOrphan handles a single orphaned bridge.
func (r *Resumer) processOrphan(ctx context.Context, orphan OrphanedBridge, workDir string) ResumeResult {
	// Look up task in store.
	task, err := r.store.Get(ctx, orphan.TaskID)
	if err != nil {
		// Task not in store — this is a stale bridge dir. Clean up.
		r.logger.Info("[BridgeResumer] Orphan %s: no matching task in store, cleaning up", orphan.TaskID)
		_ = CleanupBridgeDir(workDir, orphan.TaskID)
		return ResumeResult{TaskID: orphan.TaskID, Action: ResumeMarkFailed, Error: err}
	}

	// Skip tasks already in terminal state.
	if task.Status.IsTerminal() {
		r.logger.Info("[BridgeResumer] Orphan %s: task already terminal (%s), cleaning up", orphan.TaskID, task.Status)
		_ = CleanupBridgeDir(workDir, orphan.TaskID)
		return ResumeResult{TaskID: orphan.TaskID, Action: ResumeHarvest}
	}

	action := ClassifyOrphan(orphan, task)
	r.logger.Info("[BridgeResumer] Orphan %s: action=%s running=%v done=%v pid=%d",
		orphan.TaskID, action, orphan.IsRunning, orphan.HasDone, orphan.PID)

	switch action {
	case ResumeAdopt:
		return r.adoptOrphan(ctx, orphan, task)
	case ResumeHarvest:
		return r.harvestOrphan(ctx, orphan, task, workDir)
	case ResumeRetryWithContext:
		return r.markForRetry(ctx, orphan, task, workDir, true)
	case ResumeRetryFresh:
		return r.markForRetry(ctx, orphan, task, workDir, false)
	default:
		return r.markFailed(ctx, orphan, task, workDir)
	}
}

// adoptOrphan tails a still-running bridge's output file from the last checkpoint.
func (r *Resumer) adoptOrphan(ctx context.Context, orphan OrphanedBridge, task *taskdomain.Task) ResumeResult {
	var startOffset int64
	if task.BridgeMeta != nil {
		startOffset = task.BridgeMeta.LastOffset
	}

	reader := NewOutputReader(orphan.OutputFile, orphan.DoneFile)
	reader.SetOffset(startOffset)

	result := &agent.ExternalAgentResult{}
	events := reader.Read(ctx)
	var lastErr error

	for ev := range events {
		r.executor.applyEvent(ev, result, nil)
		if ev.Type == SDKEventError {
			lastErr = fmt.Errorf("%s", ev.Message)
		}
		// Update checkpoint.
		if ev.Type == SDKEventTool || ev.Type == SDKEventResult {
			r.updateCheckpoint(ctx, task.TaskID, reader.Offset(), result)
		}
	}

	// Store final result.
	if lastErr != nil {
		_ = r.store.SetError(ctx, task.TaskID, lastErr.Error())
		_ = r.store.SetStatus(ctx, task.TaskID, taskdomain.StatusFailed,
			taskdomain.WithTransitionReason("adopted orphan completed with error"))
		return ResumeResult{TaskID: task.TaskID, Action: ResumeAdopt, Error: lastErr}
	}

	if result.Answer != "" {
		_ = r.store.SetResult(ctx, task.TaskID, result.Answer, nil, result.TokensUsed)
		_ = r.store.SetStatus(ctx, task.TaskID, taskdomain.StatusCompleted,
			taskdomain.WithTransitionReason("adopted orphan completed"))
	}

	return ResumeResult{TaskID: task.TaskID, Action: ResumeAdopt}
}

// harvestOrphan reads remaining events from a completed bridge.
func (r *Resumer) harvestOrphan(ctx context.Context, orphan OrphanedBridge, task *taskdomain.Task, workDir string) ResumeResult {
	var startOffset int64
	if task.BridgeMeta != nil {
		startOffset = task.BridgeMeta.LastOffset
	}

	reader := NewOutputReader(orphan.OutputFile, orphan.DoneFile)
	reader.SetOffset(startOffset)

	// Short timeout — the data should already be there.
	harvestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result := &agent.ExternalAgentResult{}
	events := reader.Read(harvestCtx)
	var lastErr error

	for ev := range events {
		r.executor.applyEvent(ev, result, nil)
		if ev.Type == SDKEventError {
			lastErr = fmt.Errorf("%s", ev.Message)
		}
	}

	if lastErr != nil {
		_ = r.store.SetError(ctx, task.TaskID, lastErr.Error())
		_ = r.store.SetStatus(ctx, task.TaskID, taskdomain.StatusFailed,
			taskdomain.WithTransitionReason("harvested from completed orphan"))
	} else if result.Answer != "" {
		_ = r.store.SetResult(ctx, task.TaskID, result.Answer, nil, result.TokensUsed)
		_ = r.store.SetStatus(ctx, task.TaskID, taskdomain.StatusCompleted,
			taskdomain.WithTransitionReason("harvested from completed orphan"))
	} else {
		// Done sentinel exists but no result — mark as failed.
		_ = r.store.SetStatus(ctx, task.TaskID, taskdomain.StatusFailed,
			taskdomain.WithTransitionReason("completed orphan produced no result"))
	}

	_ = CleanupBridgeDir(workDir, task.TaskID)
	return ResumeResult{TaskID: task.TaskID, Action: ResumeHarvest, Error: lastErr}
}

// markForRetry sets the task back to pending with a resume-enriched prompt.
func (r *Resumer) markForRetry(ctx context.Context, orphan OrphanedBridge, task *taskdomain.Task, workDir string, withContext bool) ResumeResult {
	action := ResumeRetryFresh
	reason := "bridge died with no progress — retry from scratch"

	if withContext && task.BridgeMeta != nil {
		action = ResumeRetryWithContext
		reason = "bridge died after partial progress — retry with context"

		// Enrich the prompt with resume context.
		resumePrompt := buildResumePrompt(task)
		if resumePrompt != task.Prompt {
			// Update the task's prompt field for retry.
			// The caller (ResumePendingTasks) will re-dispatch with this prompt.
			task.Prompt = resumePrompt
		}
	}

	_ = r.store.SetStatus(ctx, task.TaskID, taskdomain.StatusPending,
		taskdomain.WithTransitionReason(reason))
	_ = CleanupBridgeDir(workDir, task.TaskID)

	return ResumeResult{TaskID: task.TaskID, Action: action}
}

// markFailed marks an unrecoverable orphan task as failed.
func (r *Resumer) markFailed(ctx context.Context, orphan OrphanedBridge, task *taskdomain.Task, workDir string) ResumeResult {
	_ = r.store.SetError(ctx, task.TaskID, "bridge process died unexpectedly")
	_ = r.store.SetStatus(ctx, task.TaskID, taskdomain.StatusFailed,
		taskdomain.WithTransitionReason("bridge process died unexpectedly"))
	_ = CleanupBridgeDir(workDir, task.TaskID)

	return ResumeResult{TaskID: task.TaskID, Action: ResumeMarkFailed}
}

// updateCheckpoint saves progress checkpoint to the unified store.
func (r *Resumer) updateCheckpoint(ctx context.Context, taskID string, offset int64, result *agent.ExternalAgentResult) {
	meta := taskdomain.BridgeMeta{
		LastOffset:    offset,
		LastIteration: result.Iterations,
		TokensUsed:    result.TokensUsed,
	}
	_ = r.store.SetBridgeMeta(ctx, taskID, meta)
}

// buildResumePrompt constructs a prompt with resume context for a partially-completed task.
func buildResumePrompt(task *taskdomain.Task) string {
	if task == nil || task.Prompt == "" {
		return ""
	}

	var meta *taskdomain.BridgeMeta
	if task.BridgeMeta != nil {
		meta = task.BridgeMeta
	}

	if meta == nil || (meta.LastIteration == 0 && len(meta.FilesTouched) == 0) {
		return task.Prompt
	}

	resumeCtx := fmt.Sprintf(`[Resume Context]
This task was previously attempted but interrupted after iteration %d.`, meta.LastIteration)

	if len(meta.FilesTouched) > 0 {
		resumeCtx += "\nFiles modified in previous attempt:"
		for _, f := range meta.FilesTouched {
			resumeCtx += "\n  - " + f
		}
	}
	if meta.TokensUsed > 0 {
		resumeCtx += fmt.Sprintf("\nTokens used in previous attempt: %d", meta.TokensUsed)
	}

	resumeCtx += "\nPlease review what was done and continue from where it left off.\n\n[Original Task]\n"

	return resumeCtx + task.Prompt
}
