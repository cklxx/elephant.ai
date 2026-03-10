package app

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	serverPorts "alex/internal/delivery/server/ports"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

// ResumePendingTasks re-dispatches persisted pending/running tasks after restart.
// It first detects and processes orphaned bridge subprocesses (adopt/harvest/fail),
// then re-dispatches any remaining pending/running tasks.
func (svc *TaskExecutionService) ResumePendingTasks(ctx context.Context) (int, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	logger := logging.FromContext(ctx, svc.logger)
	if svc.agentCoordinator == nil {
		return 0, UnavailableError("agent coordinator not initialized")
	}
	if svc.broadcaster == nil {
		return 0, UnavailableError("broadcaster not initialized")
	}

	// Phase 1: Detect and process orphaned bridge subprocesses.
	svc.resumeOrphanedBridges(ctx, logger)

	leaseUntil := svc.nextLeaseDeadline(time.Now())
	tasks, err := svc.taskStore.ClaimResumableTasks(
		ctx,
		svc.ownerID,
		leaseUntil,
		svc.resumeClaimBatchSize,
		serverPorts.TaskStatusPending,
		serverPorts.TaskStatusRunning,
	)
	if err != nil {
		return 0, fmt.Errorf("claim resumable tasks: %w", err)
	}
	if len(tasks) == 0 {
		logger.Debug("No pending/running tasks to resume")
		return 0, nil
	}

	resumed := 0
	skipped := 0
	for _, task := range tasks {
		if task == nil || task.ID == "" {
			skipped++
			continue
		}
		if task.Description == "" {
			logger.Warn("Skipping task %s during resume: empty description", task.ID)
			svc.releaseTaskLease(task.ID, logger)
			skipped++
			continue
		}
		if task.SessionID == "" {
			logger.Warn("Skipping task %s during resume: empty session_id", task.ID)
			svc.releaseTaskLease(task.ID, logger)
			skipped++
			continue
		}

		session, err := svc.agentCoordinator.GetSession(ctx, task.SessionID)
		if err != nil {
			logger.Warn("Skipping task %s during resume: failed to load session %s: %v", task.ID, task.SessionID, err)
			svc.releaseTaskLease(task.ID, logger)
			skipped++
			continue
		}
		if svc.stateStore != nil {
			if err := svc.stateStore.Init(ctx, session.ID); err != nil {
				logger.Warn("Resume state store init failed for session %s: %v", session.ID, err)
			}
		}

		svc.cancelMu.RLock()
		_, alreadyRunning := svc.cancelFuncs[task.ID]
		svc.cancelMu.RUnlock()
		if alreadyRunning {
			logger.Warn("Skipping task %s during resume: already has active cancel function", task.ID)
			skipped++
			continue
		}

		taskCtx := id.WithIDs(context.Background(), id.IDs{
			SessionID:   session.ID,
			RunID:       task.ID,
			ParentRunID: task.ParentTaskID,
		})
		taskCtx, _ = id.EnsureLogID(taskCtx, id.NewLogID)
		taskCtx = context.WithoutCancel(taskCtx)

		cancelCtx, cancelFunc := context.WithCancelCause(taskCtx)
		svc.cancelMu.Lock()
		svc.cancelFuncs[task.ID] = cancelFunc
		svc.cancelMu.Unlock()

		taskID := task.ID
		description := task.Description
		agentPreset := task.AgentPreset
		toolPreset := task.ToolPreset
		resumeSessionID := session.ID
		async.Go(svc.logger, "server.resumeTask", func() {
			svc.executeTaskInBackground(cancelCtx, taskID, description, resumeSessionID, agentPreset, toolPreset, nil)
		})

		logger.Debug("Resumed task: task_id=%s session_id=%s", taskID, resumeSessionID)
		resumed++
	}

	logger.Info("Resume complete: claimed=%d resumed=%d skipped=%d", len(tasks), resumed, skipped)
	return resumed, nil
}

// resumeOrphanedBridges detects and processes orphaned bridge subprocesses.
// This runs as the first step of ResumePendingTasks to adopt running bridges,
// harvest completed results, and mark dead bridges as failed before re-dispatching
// persisted tasks.
func (svc *TaskExecutionService) resumeOrphanedBridges(ctx context.Context, logger logging.Logger) {
	if svc.bridgeResumer == nil || svc.bridgeWorkDir == "" {
		return
	}

	results := svc.bridgeResumer.ResumeOrphans(ctx, svc.bridgeWorkDir)
	if len(results) == 0 {
		return
	}

	adopted, harvested, failed := 0, 0, 0
	for _, r := range results {
		switch r.Action {
		case "adopt":
			adopted++
		case "harvest":
			harvested++
		default:
			failed++
		}
		if r.Error != nil {
			logger.Warn("Resume orphan task %s (%s): %v", r.TaskID, r.Action, r.Error)
		}
	}

	logger.Info("Resume orphan bridge scan: adopted=%d harvested=%d failed=%d", adopted, harvested, failed)
}

func defaultTaskOwnerID() string {
	host, err := os.Hostname()
	if err != nil || utils.IsBlank(host) {
		host = "unknown-host"
	}
	return fmt.Sprintf("%s:%d", host, os.Getpid())
}

func (svc *TaskExecutionService) nextLeaseDeadline(now time.Time) time.Time {
	if svc.leaseTTL <= 0 {
		return now.Add(defaultTaskLeaseTTL)
	}
	return now.Add(svc.leaseTTL)
}

func (svc *TaskExecutionService) acquireAdmission(ctx context.Context) (func(), error) {
	if svc.admissionSem == nil {
		return func() {}, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	select {
	case svc.admissionSem <- struct{}{}:
		var once sync.Once
		return func() {
			once.Do(func() {
				<-svc.admissionSem
			})
		}, nil
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
}

func (svc *TaskExecutionService) startTaskLeaseRenewer(ctx context.Context, taskID string) func() {
	if svc.leaseTTL <= 0 || svc.leaseRenewInterval <= 0 || taskID == "" {
		return func() {}
	}
	logger := logging.FromContext(ctx, svc.logger)
	stop := make(chan struct{})
	var stopOnce sync.Once

	async.Go(svc.logger, "server.taskLeaseRenewer", func() {
		ticker := time.NewTicker(svc.leaseRenewInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				renewUntil := svc.nextLeaseDeadline(time.Now())
				ok, err := svc.taskStore.RenewTaskLease(context.Background(), taskID, svc.ownerID, renewUntil)
				if err != nil {
					logger.Warn("Lease renew failed for task %s: %v", taskID, err)
					continue
				}
				if !ok {
					logger.Warn("Lease ownership lost for task %s, cancelling local execution", taskID)
					svc.cancelTaskExecution(taskID, fmt.Errorf("task lease lost"))
					return
				}
			}
		}
	})

	return func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}
}

func (svc *TaskExecutionService) cancelTaskExecution(taskID string, cause error) {
	if taskID == "" || cause == nil {
		return
	}
	svc.cancelMu.RLock()
	cancelFunc := svc.cancelFuncs[taskID]
	svc.cancelMu.RUnlock()
	if cancelFunc != nil {
		cancelFunc(cause)
	}
}

func (svc *TaskExecutionService) releaseTaskLease(taskID string, logger logging.Logger) {
	if taskID == "" {
		return
	}
	if err := svc.taskStore.ReleaseTaskLease(context.Background(), taskID, svc.ownerID); err != nil {
		logger.Warn("Lease release failed for task %s: %v", taskID, err)
	}
}
