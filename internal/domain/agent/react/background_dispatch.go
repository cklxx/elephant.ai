package react

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/executioncontrol"
)

// Dispatch starts a background task. Returns an error if the task ID is already in use.
func (m *BackgroundTaskManager) Dispatch(
	ctx context.Context,
	req agent.BackgroundDispatchRequest,
) error {
	taskID := strings.TrimSpace(req.TaskID)
	if taskID == "" {
		return fmt.Errorf("task_id is required")
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		return fmt.Errorf("description is required")
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return fmt.Errorf("prompt is required")
	}
	agentType := strings.TrimSpace(req.AgentType)
	if agentType == "" {
		agentType = "internal"
	}
	executionMode := executioncontrol.NormalizeExecutionMode(req.ExecutionMode)
	autonomyLevel := executioncontrol.NormalizeAutonomyLevel(req.AutonomyLevel)
	workspaceMode := req.WorkspaceMode
	if workspaceMode == "" {
		workspaceMode = agent.WorkspaceModeShared
	}
	if workspaceMode != agent.WorkspaceModeShared &&
		workspaceMode != agent.WorkspaceModeBranch &&
		workspaceMode != agent.WorkspaceModeWorktree {
		return fmt.Errorf("invalid workspace_mode: %s", workspaceMode)
	}

	sink := resolveBackgroundEventSink(ctx, backgroundEventSink{
		emitEvent:      m.emitEvent,
		baseEvent:      m.baseEvent,
		parentListener: m.parentListener,
	})

	m.mu.Lock()
	if _, exists := m.tasks[taskID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("background task %q already exists", taskID)
	}
	if m.maxConcurrentTasks > 0 {
		active := m.reconcileActiveTaskCountLocked()
		if active >= m.maxConcurrentTasks {
			m.mu.Unlock()
			return fmt.Errorf("background task limit reached: %d active (max=%d)", active, m.maxConcurrentTasks)
		}
	}

	bt := &backgroundTask{
		id:             taskID,
		description:    description,
		prompt:         prompt,
		agentType:      agentType,
		executionMode:  executionMode,
		autonomyLevel:  autonomyLevel,
		causationID:    req.CausationID,
		status:         agent.BackgroundTaskStatusPending,
		mergeStatus:    agent.MergeStatusNotMerged,
		startedAt:      m.clock.Now(),
		emitEvent:      sink.emitEvent,
		baseEvent:      sink.baseEvent,
		parentListener: sink.parentListener,
		notifyParent:   sink.notifyParent,
		dependsOn:      append([]string(nil), req.DependsOn...),
		inheritContext: req.InheritContext,
		fileScope:      append([]string(nil), req.FileScope...),
		config:         core.CloneStringMap(req.Config),
	}
	if len(req.DependsOn) > 0 {
		bt.status = agent.BackgroundTaskStatusBlocked
	}
	if err := m.validateDependencies(taskID, req.DependsOn); err != nil {
		m.mu.Unlock()
		return err
	}

	if workspaceMode != agent.WorkspaceModeShared {
		if m.workspaceMgr == nil {
			m.mu.Unlock()
			return fmt.Errorf("workspace manager not available for mode %s", workspaceMode)
		}
		alloc, err := m.workspaceMgr.Allocate(ctx, taskID, workspaceMode, req.FileScope)
		if err != nil {
			m.mu.Unlock()
			return err
		}
		bt.workspace = alloc
	}
	m.tasks[taskID] = bt
	m.activeTasks.Add(1)
	m.mu.Unlock()

	// Build detached context preserving causal chain values from the run context.
	taskCtx := m.taskCtx
	ids := m.idContext.IDsFromContext(ctx)
	if ids.SessionID == "" && ids.RunID == "" && ids.ParentRunID == "" &&
		ids.LogID == "" && ids.CorrelationID == "" && ids.CausationID == "" {
		ids = m.idContext.IDsFromContext(m.runCtx)
	}
	if ids.SessionID != "" {
		taskCtx = m.idContext.WithSessionID(taskCtx, ids.SessionID)
	}
	if ids.RunID != "" {
		taskCtx = m.idContext.WithParentRunID(taskCtx, ids.RunID)
	}
	taskCtx = m.idContext.WithRunID(taskCtx, m.idGenerator.NewRunID())
	if ids.CorrelationID != "" {
		taskCtx = m.idContext.WithCorrelationID(taskCtx, ids.CorrelationID)
	} else if ids.RunID != "" {
		taskCtx = m.idContext.WithCorrelationID(taskCtx, ids.RunID)
	}
	if bt.causationID != "" {
		taskCtx = m.idContext.WithCausationID(taskCtx, bt.causationID)
	}
	if ids.LogID != "" {
		taskCtx = m.idContext.WithLogID(taskCtx, fmt.Sprintf("%s:bg:%s", ids.LogID, m.idGenerator.NewLogID()))
	}

	// Propagate CompletionNotifier from dispatch context (or run context as fallback).
	if notifier := agent.GetCompletionNotifier(ctx); notifier != nil {
		taskCtx = agent.WithCompletionNotifier(taskCtx, notifier)
	} else if notifier := agent.GetCompletionNotifier(m.runCtx); notifier != nil {
		taskCtx = agent.WithCompletionNotifier(taskCtx, notifier)
	}

	// Propagate app-layer context values (e.g. LLM selection) via registered propagators.
	for _, propagate := range m.contextPropagators {
		taskCtx = propagate(ctx, taskCtx)
	}

	taskCtx, taskCancel := context.WithCancel(taskCtx)
	bt.taskCancel = taskCancel

	m.goRunner(m.logger, "bg-task:"+taskID, func() {
		m.runTask(taskCtx, bt, agentType)
	})

	return nil
}

func (m *BackgroundTaskManager) activeTaskCountLocked() int {
	count := 0
	for _, bt := range m.tasks {
		bt.mu.Lock()
		status := bt.status
		completed := !bt.completedAt.IsZero()
		signaled := bt.completionSignaled
		bt.mu.Unlock()
		if completed || signaled {
			continue
		}
		switch status {
		case agent.BackgroundTaskStatusPending, agent.BackgroundTaskStatusBlocked, agent.BackgroundTaskStatusRunning:
			count++
		}
	}
	return count
}

func (m *BackgroundTaskManager) reconcileActiveTaskCountLocked() int {
	actual := m.activeTaskCountLocked()
	m.activeTasks.Store(int64(actual))
	return actual
}

// runTask executes a background task, routing to internal or external executor.
func (m *BackgroundTaskManager) runTask(ctx context.Context, bt *backgroundTask, agentType string) {
	defer func() {
		if r := recover(); r != nil {
			bt.mu.Lock()
			alreadyDone := !bt.completedAt.IsZero()
			if !alreadyDone {
				bt.completedAt = m.clock.Now()
				bt.err = fmt.Errorf("task panicked: %v", r)
				bt.status = agent.BackgroundTaskStatusFailed
			}
			bt.mu.Unlock()
			if !alreadyDone {
				m.emitCompletionEvent(ctx, bt)
				if notifier := agent.GetCompletionNotifier(ctx); notifier != nil {
					notifier.NotifyCompletion(ctx, bt.id, string(agent.BackgroundTaskStatusFailed), "", bt.err.Error(), agent.MergeStatusNotMerged, 0)
				}
				m.signalCompletion(bt.id)
			}
		}
	}()

	now := m.clock.Now()
	bt.mu.Lock()
	if bt.status != agent.BackgroundTaskStatusBlocked {
		bt.status = agent.BackgroundTaskStatusRunning
		bt.lastActivityAt = now
	}
	bt.mu.Unlock()

	if len(bt.dependsOn) > 0 {
		if err := m.awaitDependencies(ctx, bt); err != nil {
			bt.mu.Lock()
			bt.completedAt = m.clock.Now()
			bt.err = err
			bt.status = agent.BackgroundTaskStatusFailed
			bt.mu.Unlock()
			m.signalCompletion(bt.id)
			return
		}
		bt.mu.Lock()
		bt.status = agent.BackgroundTaskStatusRunning
		bt.lastActivityAt = m.clock.Now()
		bt.mu.Unlock()
	}

	prompt := bt.prompt
	if bt.inheritContext {
		prompt = m.buildContextEnrichedPrompt(bt)
	}

	var result *agent.TaskResult
	var err error

	switch agentType {
	case "", "internal":
		listener := bt.parentListener
		if listener == nil {
			listener = m.parentListener
		}
		result, err = alexerrors.RetryWithResultAndLog(ctx, alexerrors.RetryConfig{
			MaxAttempts:  2,
			BaseDelay:    10 * time.Second,
			MaxDelay:     30 * time.Second,
			JitterFactor: 0.25,
		}, func(ctx context.Context) (*agent.TaskResult, error) {
			return m.executeTask(ctx, prompt, m.sessionID, listener)
		}, m.logger)
	default:
		if m.externalExecutor == nil {
			err = fmt.Errorf("external agent executor not configured for type %q", agentType)
		} else {
			workingDir := m.workingDir
			if bt.workspace != nil && bt.workspace.WorkingDir != "" {
				workingDir = bt.workspace.WorkingDir
			}

			// Heartbeat goroutine: emit lightweight progress events every 5 minutes
			// to keep the SerializingEventListener queue alive during long idle periods.
			heartbeatCtx, heartbeatCancel := context.WithCancel(ctx)
			if bt.emitEvent != nil && bt.baseEvent != nil {
				m.goRunner(m.logger, "bg-heartbeat:"+bt.id, func() {
					m.runHeartbeat(heartbeatCtx, bt)
				})
			}

			extResult, execErr := m.externalExecutor.Execute(ctx, agent.ExternalAgentRequest{
				TaskID:        bt.id,
				Prompt:        prompt,
				AgentType:     agentType,
				WorkingDir:    workingDir,
				Config:        core.CloneStringMap(bt.config),
				SessionID:     m.sessionID,
				CausationID:   bt.causationID,
				ExecutionMode: bt.executionMode,
				AutonomyLevel: bt.autonomyLevel,
				OnProgress: func(p agent.ExternalAgentProgress) {
					m.captureProgress(ctx, bt, p)
				},
				OnBridgeStarted: func(info any) {
					// Propagate bridge started info to completion notifier for persistence.
					if notifier := agent.GetCompletionNotifier(ctx); notifier != nil {
						if persister, ok := notifier.(agent.BridgeMetaPersister); ok {
							persister.PersistBridgeMeta(ctx, bt.id, info)
						}
					}
				},
			})
			heartbeatCancel()

			if extResult != nil {
				result = &agent.TaskResult{
					Answer:     extResult.Answer,
					Iterations: extResult.Iterations,
					TokensUsed: extResult.TokensUsed,
				}
			}
			if execErr != nil {
				err = execErr
			} else if extResult != nil && extResult.Error != "" {
				err = fmt.Errorf("%s", extResult.Error)
			}
		}
	}

	if err == nil {
		if mergeErr := m.tryAutoMerge(ctx, bt, result); mergeErr != nil {
			err = mergeErr
		}
	}

	bt.mu.Lock()
	bt.completedAt = m.clock.Now()
	bt.result = result
	bt.err = err
	switch {
	case ctx.Err() != nil:
		bt.status = agent.BackgroundTaskStatusCancelled
	case err != nil:
		bt.status = agent.BackgroundTaskStatusFailed
	default:
		bt.status = agent.BackgroundTaskStatusCompleted
	}
	bt.mu.Unlock()

	m.emitCompletionEvent(ctx, bt)

	// Direct TaskStore write via CompletionNotifier — ensures persistence
	// even if the event listener chain is broken.
	if notifier := agent.GetCompletionNotifier(ctx); notifier != nil {
		bt.mu.Lock()
		nStatus := string(bt.status)
		nAnswer := ""
		nTokens := 0
		nMerge := bt.mergeStatus
		if bt.result != nil {
			nAnswer = bt.result.Answer
			nTokens = bt.result.TokensUsed
		}
		nErr := ""
		if bt.err != nil {
			nErr = bt.err.Error()
		}
		bt.mu.Unlock()
		notifier.NotifyCompletion(ctx, bt.id, nStatus, nAnswer, nErr, nMerge, nTokens)
	}

	m.signalCompletion(bt.id)
}
