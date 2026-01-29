package react

import (
	"context"
	"fmt"
	"sync"
	"time"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/async"
	id "alex/internal/utils/id"
)

// backgroundTask tracks an individual background task.
type backgroundTask struct {
	mu          sync.Mutex
	id          string
	description string
	prompt      string
	agentType   string
	causationID string
	status      agent.BackgroundTaskStatus
	startedAt   time.Time
	completedAt time.Time
	result      *agent.TaskResult
	err         error
}

// BackgroundTaskManager manages background task lifecycle within a single run.
// It implements agent.BackgroundTaskDispatcher.
type BackgroundTaskManager struct {
	mu          sync.RWMutex
	tasks       map[string]*backgroundTask
	completions chan string // task IDs signaled on completion
	logger      agent.Logger
	clock       agent.Clock
	taskCtx     context.Context
	cancelAll   context.CancelFunc
	runCtx      context.Context // for value inheritance (IDs, etc.)

	// executeTask delegates to coordinator.ExecuteTask for internal subagents.
	executeTask func(ctx context.Context, prompt, sessionID string,
		listener agent.EventListener) (*agent.TaskResult, error)

	// externalExecutor handles external code agents (can be nil).
	externalExecutor agent.ExternalAgentExecutor

	sessionID      string
	parentListener agent.EventListener
}

// newBackgroundTaskManager creates a new manager bound to the current run context.
func newBackgroundTaskManager(
	runCtx context.Context,
	logger agent.Logger,
	clock agent.Clock,
	executeTask func(ctx context.Context, prompt, sessionID string,
		listener agent.EventListener) (*agent.TaskResult, error),
	externalExecutor agent.ExternalAgentExecutor,
	sessionID string,
	parentListener agent.EventListener,
) *BackgroundTaskManager {
	taskCtx, cancel := context.WithCancel(context.Background())
	return &BackgroundTaskManager{
		tasks:            make(map[string]*backgroundTask),
		completions:      make(chan string, 64),
		logger:           logger,
		clock:            clock,
		taskCtx:          taskCtx,
		cancelAll:        cancel,
		runCtx:           runCtx,
		executeTask:      executeTask,
		externalExecutor: externalExecutor,
		sessionID:        sessionID,
		parentListener:   parentListener,
	}
}

// Dispatch starts a background task. Returns an error if the task ID is already in use.
func (m *BackgroundTaskManager) Dispatch(
	ctx context.Context,
	taskID, description, prompt, agentType, causationID string,
) error {
	m.mu.Lock()
	if _, exists := m.tasks[taskID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("background task %q already exists", taskID)
	}

	bt := &backgroundTask{
		id:          taskID,
		description: description,
		prompt:      prompt,
		agentType:   agentType,
		causationID: causationID,
		status:      agent.BackgroundTaskStatusPending,
		startedAt:   m.clock.Now(),
	}
	m.tasks[taskID] = bt
	m.mu.Unlock()

	// Build detached context preserving causal chain values from the run context.
	taskCtx := m.taskCtx
	ids := id.IDsFromContext(m.runCtx)
	if ids.SessionID != "" {
		taskCtx = id.WithSessionID(taskCtx, ids.SessionID)
	}
	if ids.RunID != "" {
		taskCtx = id.WithParentRunID(taskCtx, ids.RunID)
	}
	taskCtx = id.WithRunID(taskCtx, id.NewRunID())
	if ids.CorrelationID != "" {
		taskCtx = id.WithCorrelationID(taskCtx, ids.CorrelationID)
	} else if ids.RunID != "" {
		taskCtx = id.WithCorrelationID(taskCtx, ids.RunID)
	}
	if causationID != "" {
		taskCtx = id.WithCausationID(taskCtx, causationID)
	}
	if ids.LogID != "" {
		taskCtx = id.WithLogID(taskCtx, fmt.Sprintf("%s:bg:%s", ids.LogID, id.NewLogID()))
	}

	async.Go(m.logger, "bg-task:"+taskID, func() {
		m.runTask(taskCtx, bt, agentType)
	})

	return nil
}

// runTask executes a background task, routing to internal or external executor.
func (m *BackgroundTaskManager) runTask(ctx context.Context, bt *backgroundTask, agentType string) {
	bt.mu.Lock()
	bt.status = agent.BackgroundTaskStatusRunning
	bt.mu.Unlock()

	var result *agent.TaskResult
	var err error

	switch agentType {
	case "", "internal":
		result, err = m.executeTask(ctx, bt.prompt, m.sessionID, m.parentListener)
	default:
		if m.externalExecutor == nil {
			err = fmt.Errorf("external agent executor not configured for type %q", agentType)
		} else {
			var extResult *agent.ExternalAgentResult
			extResult, err = m.externalExecutor.Execute(ctx, agent.ExternalAgentRequest{
				Prompt:      bt.prompt,
				SessionID:   m.sessionID,
				CausationID: bt.causationID,
			})
			if err == nil && extResult != nil {
				result = &agent.TaskResult{
					Answer:     extResult.Answer,
					Iterations: extResult.Iterations,
					TokensUsed: extResult.TokensUsed,
				}
				if extResult.Error != "" {
					err = fmt.Errorf("%s", extResult.Error)
				}
			}
		}
	}

	bt.mu.Lock()
	bt.completedAt = m.clock.Now()
	bt.result = result
	bt.err = err
	if ctx.Err() != nil {
		bt.status = agent.BackgroundTaskStatusCancelled
	} else if err != nil {
		bt.status = agent.BackgroundTaskStatusFailed
	} else {
		bt.status = agent.BackgroundTaskStatusCompleted
	}
	bt.mu.Unlock()

	// Signal completion (non-blocking).
	select {
	case m.completions <- bt.id:
	default:
		m.logger.Warn("background completions channel full, dropping signal for task %q", bt.id)
	}
}

// Status returns lightweight summaries for the requested task IDs.
// Pass nil or empty slice to query all tasks.
func (m *BackgroundTaskManager) Status(ids []string) []agent.BackgroundTaskSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := m.resolveTargets(ids)
	summaries := make([]agent.BackgroundTaskSummary, 0, len(targets))
	for _, bt := range targets {
		bt.mu.Lock()
		s := agent.BackgroundTaskSummary{
			ID:          bt.id,
			Description: bt.description,
			Status:      bt.status,
			AgentType:   bt.agentType,
			StartedAt:   bt.startedAt,
			CompletedAt: bt.completedAt,
		}
		if bt.err != nil {
			s.Error = bt.err.Error()
		}
		bt.mu.Unlock()
		summaries = append(summaries, s)
	}
	return summaries
}

// Collect returns full results for the requested tasks.
// When wait is true, blocks until tasks complete or timeout elapses.
func (m *BackgroundTaskManager) Collect(ids []string, wait bool, timeout time.Duration) []agent.BackgroundTaskResult {
	if wait {
		m.awaitTasks(ids, timeout)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := m.resolveTargets(ids)
	results := make([]agent.BackgroundTaskResult, 0, len(targets))
	for _, bt := range targets {
		bt.mu.Lock()
		r := agent.BackgroundTaskResult{
			ID:          bt.id,
			Description: bt.description,
			Status:      bt.status,
			AgentType:   bt.agentType,
			Duration:    bt.completedAt.Sub(bt.startedAt),
		}
		if bt.result != nil {
			r.Answer = bt.result.Answer
			r.RunID = bt.result.RunID
			r.Iterations = bt.result.Iterations
			r.TokensUsed = bt.result.TokensUsed
		}
		if bt.err != nil {
			r.Error = bt.err.Error()
		}
		bt.mu.Unlock()
		results = append(results, r)
	}
	return results
}

// DrainCompletions returns all newly completed task IDs without blocking.
func (m *BackgroundTaskManager) DrainCompletions() []string {
	var ids []string
	for {
		select {
		case tid := <-m.completions:
			ids = append(ids, tid)
		default:
			return ids
		}
	}
}

// AwaitAll blocks until every dispatched task has finished or the timeout elapses.
func (m *BackgroundTaskManager) AwaitAll(timeout time.Duration) {
	m.awaitTasks(nil, timeout)
}

// Shutdown cancels all remaining tasks.
func (m *BackgroundTaskManager) Shutdown() {
	m.cancelAll()
}

// resolveTargets returns the tasks matching ids, or all tasks when ids is empty.
// Caller must hold m.mu (read lock or write lock).
func (m *BackgroundTaskManager) resolveTargets(ids []string) []*backgroundTask {
	if len(ids) == 0 {
		targets := make([]*backgroundTask, 0, len(m.tasks))
		for _, bt := range m.tasks {
			targets = append(targets, bt)
		}
		return targets
	}
	targets := make([]*backgroundTask, 0, len(ids))
	for _, tid := range ids {
		if bt, ok := m.tasks[tid]; ok {
			targets = append(targets, bt)
		}
	}
	return targets
}

// awaitTasks blocks until the specified tasks (or all tasks when ids is empty)
// are no longer pending/running, or timeout elapses.
func (m *BackgroundTaskManager) awaitTasks(ids []string, timeout time.Duration) {
	deadline := m.clock.Now().Add(timeout)
	pollInterval := 50 * time.Millisecond

	for {
		if m.clock.Now().After(deadline) {
			return
		}

		allDone := true
		m.mu.RLock()
		targets := m.resolveTargets(ids)
		for _, bt := range targets {
			bt.mu.Lock()
			if bt.status == agent.BackgroundTaskStatusPending || bt.status == agent.BackgroundTaskStatusRunning {
				allDone = false
			}
			bt.mu.Unlock()
			if !allDone {
				break
			}
		}
		m.mu.RUnlock()

		if allDone {
			return
		}

		time.Sleep(pollInterval)
	}
}

// TaskCount returns the number of tracked tasks.
func (m *BackgroundTaskManager) TaskCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tasks)
}
