package react

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
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

	emitEvent      func(agent.AgentEvent)
	baseEvent      func(context.Context) domain.BaseEvent
	parentListener agent.EventListener

	progress         *agent.ExternalAgentProgress
	pendingInput     *agent.InputRequestSummary
	lastProgressEmit time.Time
	dependsOn        []string
	inheritContext   bool
	workspace        *agent.WorkspaceAllocation
	fileScope        []string
	config           map[string]string
}

// BackgroundTaskManager manages background task lifecycle within a single run.
// It implements agent.BackgroundTaskDispatcher.
type BackgroundTaskManager struct {
	mu           sync.RWMutex
	tasks        map[string]*backgroundTask
	completions  chan string // task IDs signaled on completion
	logger       agent.Logger
	clock        agent.Clock
	taskCtx      context.Context
	cancelAll    context.CancelFunc
	runCtx       context.Context // for value inheritance (IDs, etc.)
	workingDir   string
	workspaceMgr agent.WorkspaceManager
	idGenerator  agent.IDGenerator
	idContext    agent.IDContextReader
	goRunner     agent.GoRunner

	// executeTask delegates to coordinator.ExecuteTask for internal subagents.
	executeTask func(ctx context.Context, prompt, sessionID string,
		listener agent.EventListener) (*agent.TaskResult, error)

	// externalExecutor handles external code agents (can be nil).
	externalExecutor agent.ExternalAgentExecutor
	inputExecutor    agent.InteractiveExternalExecutor
	externalInputCh  chan agent.InputRequest
	closeInputOnce   sync.Once
	emitEvent        func(event agent.AgentEvent)
	baseEvent        func(ctx context.Context) domain.BaseEvent

	sessionID      string
	parentListener agent.EventListener
}

// BackgroundManagerConfig configures a shared background task manager.
type BackgroundManagerConfig struct {
	RunContext          context.Context
	Logger              agent.Logger
	Clock               agent.Clock
	IDGenerator         agent.IDGenerator
	IDContextReader     agent.IDContextReader
	GoRunner            agent.GoRunner
	WorkingDirResolver  agent.WorkingDirResolver
	WorkspaceMgrFactory agent.WorkspaceManagerFactory
	ExecuteTask         func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
	ExternalExecutor    agent.ExternalAgentExecutor
	SessionID           string
}

// newBackgroundTaskManager creates a new manager bound to the current run context.
func newBackgroundTaskManager(
	runCtx context.Context,
	logger agent.Logger,
	clock agent.Clock,
	executeTask func(ctx context.Context, prompt, sessionID string,
		listener agent.EventListener) (*agent.TaskResult, error),
	externalExecutor agent.ExternalAgentExecutor,
	emitEvent func(event agent.AgentEvent),
	baseEvent func(ctx context.Context) domain.BaseEvent,
	sessionID string,
	parentListener agent.EventListener,
) *BackgroundTaskManager {
	return newBackgroundTaskManagerWithDeps(
		runCtx,
		logger,
		clock,
		nil,
		nil,
		nil,
		nil,
		nil,
		executeTask,
		externalExecutor,
		emitEvent,
		baseEvent,
		sessionID,
		parentListener,
	)
}

func newBackgroundTaskManagerWithDeps(
	runCtx context.Context,
	logger agent.Logger,
	clock agent.Clock,
	idGenerator agent.IDGenerator,
	idContextReader agent.IDContextReader,
	goRunner agent.GoRunner,
	workingDirResolver agent.WorkingDirResolver,
	workspaceMgrFactory agent.WorkspaceManagerFactory,
	executeTask func(ctx context.Context, prompt, sessionID string,
		listener agent.EventListener) (*agent.TaskResult, error),
	externalExecutor agent.ExternalAgentExecutor,
	emitEvent func(event agent.AgentEvent),
	baseEvent func(ctx context.Context) domain.BaseEvent,
	sessionID string,
	parentListener agent.EventListener,
) *BackgroundTaskManager {
	if idGenerator == nil {
		idGenerator = defaultIDGenerator{}
	}
	if idContextReader == nil {
		idContextReader = defaultIDContextReader{}
	}
	if goRunner == nil {
		goRunner = defaultGoRunner{}
	}
	if workingDirResolver == nil {
		workingDirResolver = defaultWorkingDirResolver{}
	}
	if workspaceMgrFactory == nil {
		workspaceMgrFactory = defaultWorkspaceManagerFactory{}
	}

	taskCtx, cancel := context.WithCancel(context.Background())
	workingDir := ""
	if workingDirResolver != nil {
		workingDir = strings.TrimSpace(workingDirResolver.ResolveWorkingDir(runCtx))
	}
	var workspaceMgr agent.WorkspaceManager
	if workspaceMgrFactory != nil && workingDir != "" {
		workspaceMgr = workspaceMgrFactory.NewWorkspaceManager(workingDir, logger)
	}

	var inputExecutor agent.InteractiveExternalExecutor
	var externalInputCh chan agent.InputRequest
	if externalExecutor != nil {
		if interactive, ok := externalExecutor.(agent.InteractiveExternalExecutor); ok {
			if interactive.InputRequests() != nil {
				inputExecutor = interactive
				externalInputCh = make(chan agent.InputRequest, 32)
			}
		}
	}

	manager := &BackgroundTaskManager{
		tasks:            make(map[string]*backgroundTask),
		completions:      make(chan string, 64),
		logger:           logger,
		clock:            clock,
		taskCtx:          taskCtx,
		cancelAll:        cancel,
		runCtx:           runCtx,
		workingDir:       workingDir,
		workspaceMgr:     workspaceMgr,
		idGenerator:      idGenerator,
		idContext:        idContextReader,
		goRunner:         goRunner,
		executeTask:      executeTask,
		externalExecutor: externalExecutor,
		inputExecutor:    inputExecutor,
		externalInputCh:  externalInputCh,
		emitEvent:        emitEvent,
		baseEvent:        baseEvent,
		sessionID:        sessionID,
		parentListener:   parentListener,
	}

	if inputExecutor != nil && externalInputCh != nil {
		goRunner.Go(logger, "bg.externalInput", func() {
			manager.forwardExternalInputRequests()
		})
	}

	return manager
}

// NewBackgroundTaskManager creates a background task manager intended for reuse (e.g., per session).
func NewBackgroundTaskManager(cfg BackgroundManagerConfig) *BackgroundTaskManager {
	return newBackgroundTaskManagerWithDeps(
		cfg.RunContext,
		cfg.Logger,
		cfg.Clock,
		cfg.IDGenerator,
		cfg.IDContextReader,
		cfg.GoRunner,
		cfg.WorkingDirResolver,
		cfg.WorkspaceMgrFactory,
		cfg.ExecuteTask,
		cfg.ExternalExecutor,
		nil,
		nil,
		cfg.SessionID,
		nil,
	)
}

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

	bt := &backgroundTask{
		id:             taskID,
		description:    description,
		prompt:         prompt,
		agentType:      agentType,
		causationID:    req.CausationID,
		status:         agent.BackgroundTaskStatusPending,
		startedAt:      m.clock.Now(),
		emitEvent:      sink.emitEvent,
		baseEvent:      sink.baseEvent,
		parentListener: sink.parentListener,
		dependsOn:      append([]string(nil), req.DependsOn...),
		inheritContext: req.InheritContext,
		fileScope:      append([]string(nil), req.FileScope...),
		config:         cloneStringMap(req.Config),
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

	m.goRunner.Go(m.logger, "bg-task:"+taskID, func() {
		m.runTask(taskCtx, bt, agentType)
	})

	return nil
}

// runTask executes a background task, routing to internal or external executor.
func (m *BackgroundTaskManager) runTask(ctx context.Context, bt *backgroundTask, agentType string) {
	bt.mu.Lock()
	if bt.status != agent.BackgroundTaskStatusBlocked {
		bt.status = agent.BackgroundTaskStatusRunning
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
		result, err = m.executeTask(ctx, prompt, m.sessionID, listener)
	default:
		if m.externalExecutor == nil {
			err = fmt.Errorf("external agent executor not configured for type %q", agentType)
		} else {
			workingDir := m.workingDir
			if bt.workspace != nil && bt.workspace.WorkingDir != "" {
				workingDir = bt.workspace.WorkingDir
			}

			extResult, execErr := m.externalExecutor.Execute(ctx, agent.ExternalAgentRequest{
				TaskID:      bt.id,
				Prompt:      prompt,
				AgentType:   agentType,
				WorkingDir:  workingDir,
				Config:      cloneStringMap(bt.config),
				SessionID:   m.sessionID,
				CausationID: bt.causationID,
				OnProgress: func(p agent.ExternalAgentProgress) {
					m.captureProgress(ctx, bt, p)
				},
			})
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

	m.emitCompletionEvent(ctx, bt)
	m.signalCompletion(bt.id)
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
		now := m.clock.Now()
		elapsed := time.Duration(0)
		if !bt.startedAt.IsZero() {
			end := bt.completedAt
			if end.IsZero() {
				end = now
			}
			elapsed = end.Sub(bt.startedAt)
			if elapsed < 0 {
				elapsed = 0
			}
		}
		s := agent.BackgroundTaskSummary{
			ID:          bt.id,
			Description: bt.description,
			Status:      bt.status,
			AgentType:   bt.agentType,
			StartedAt:   bt.startedAt,
			CompletedAt: bt.completedAt,
			Elapsed:     elapsed,
		}
		if bt.err != nil {
			s.Error = bt.err.Error()
		}
		if bt.progress != nil {
			progress := *bt.progress
			s.Progress = &progress
		}
		if bt.pendingInput != nil {
			pending := *bt.pendingInput
			s.PendingInput = &pending
		}
		if bt.workspace != nil {
			workspaceCopy := *bt.workspace
			s.Workspace = &workspaceCopy
		}
		if len(bt.fileScope) > 0 {
			s.FileScope = append([]string(nil), bt.fileScope...)
		}
		if len(bt.dependsOn) > 0 {
			s.DependsOn = append([]string(nil), bt.dependsOn...)
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
		duration := time.Duration(0)
		if !bt.completedAt.IsZero() {
			duration = bt.completedAt.Sub(bt.startedAt)
			if duration < 0 {
				duration = 0
			}
		}
		r := agent.BackgroundTaskResult{
			ID:          bt.id,
			Description: bt.description,
			Status:      bt.status,
			AgentType:   bt.agentType,
			Duration:    duration,
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

// InputRequests exposes external input requests when available.
func (m *BackgroundTaskManager) InputRequests() <-chan agent.InputRequest {
	return m.externalInputCh
}

// ReplyExternalInput forwards an external input response to the executor.
func (m *BackgroundTaskManager) ReplyExternalInput(ctx context.Context, resp agent.InputResponse) error {
	if m.inputExecutor == nil {
		return fmt.Errorf("external input responder not configured")
	}
	if err := m.inputExecutor.Reply(ctx, resp); err != nil {
		return err
	}
	if strings.TrimSpace(resp.TaskID) != "" {
		m.mu.RLock()
		bt := m.tasks[resp.TaskID]
		m.mu.RUnlock()
		if bt != nil {
			bt.mu.Lock()
			if bt.pendingInput != nil && bt.pendingInput.RequestID == resp.RequestID {
				bt.pendingInput = nil
			}
			bt.mu.Unlock()
		}
	}
	if strings.TrimSpace(resp.TaskID) != "" {
		m.mu.RLock()
		bt := m.tasks[resp.TaskID]
		m.mu.RUnlock()
		if bt != nil && bt.emitEvent != nil && bt.baseEvent != nil {
			bt.emitEvent(&domain.ExternalInputResponseEvent{
				BaseEvent: bt.baseEvent(ctx),
				TaskID:    resp.TaskID,
				RequestID: resp.RequestID,
				Approved:  resp.Approved,
				OptionID:  resp.OptionID,
				Message:   resp.Text,
			})
		}
	}
	return nil
}

// MergeExternalWorkspace merges an external agent's workspace back into the base branch.
func (m *BackgroundTaskManager) MergeExternalWorkspace(ctx context.Context, taskID string, strategy agent.MergeStrategy) (*agent.MergeResult, error) {
	if m.workspaceMgr == nil {
		return nil, fmt.Errorf("workspace manager not available")
	}
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("task_id is required")
	}
	if strategy == "" {
		strategy = agent.MergeStrategyAuto
	}

	m.mu.RLock()
	bt := m.tasks[taskID]
	m.mu.RUnlock()
	if bt == nil {
		return nil, fmt.Errorf("background task %q not found", taskID)
	}
	bt.mu.Lock()
	alloc := bt.workspace
	bt.mu.Unlock()
	if alloc == nil {
		return nil, fmt.Errorf("task %q has no workspace to merge", taskID)
	}
	return m.workspaceMgr.Merge(ctx, alloc, strategy)
}

func (m *BackgroundTaskManager) forwardExternalInputRequests() {
	if m.inputExecutor == nil || m.externalInputCh == nil {
		return
	}
	inputs := m.inputExecutor.InputRequests()
	for {
		select {
		case <-m.taskCtx.Done():
			m.closeInputOnce.Do(func() { close(m.externalInputCh) })
			return
		case req, ok := <-inputs:
			if !ok {
				m.closeInputOnce.Do(func() { close(m.externalInputCh) })
				return
			}
			if strings.TrimSpace(req.TaskID) != "" {
				m.mu.RLock()
				bt := m.tasks[req.TaskID]
				m.mu.RUnlock()
				if bt != nil {
					bt.mu.Lock()
					bt.pendingInput = &agent.InputRequestSummary{
						RequestID: req.RequestID,
						Type:      req.Type,
						Summary:   req.Summary,
						Since:     m.clock.Now(),
					}
					bt.mu.Unlock()
				}
			}
			select {
			case m.externalInputCh <- req:
			default:
				m.logger.Warn("external input channel full, dropping request %q", req.RequestID)
			}
		}
	}
}

func (m *BackgroundTaskManager) captureProgress(ctx context.Context, bt *backgroundTask, p agent.ExternalAgentProgress) {
	now := m.clock.Now()
	shouldEmit := false
	bt.mu.Lock()
	bt.progress = &p
	if bt.emitEvent != nil && bt.baseEvent != nil {
		if bt.lastProgressEmit.IsZero() || now.Sub(bt.lastProgressEmit) >= 2*time.Second {
			bt.lastProgressEmit = now
			shouldEmit = true
		}
	}
	bt.mu.Unlock()

	if shouldEmit {
		elapsed := time.Duration(0)
		if !bt.startedAt.IsZero() {
			elapsed = now.Sub(bt.startedAt)
			if elapsed < 0 {
				elapsed = 0
			}
		}
		bt.emitEvent(&domain.ExternalAgentProgressEvent{
			BaseEvent:    bt.baseEvent(ctx),
			TaskID:       bt.id,
			AgentType:    bt.agentType,
			Iteration:    p.Iteration,
			MaxIter:      p.MaxIter,
			TokensUsed:   p.TokensUsed,
			CostUSD:      p.CostUSD,
			CurrentTool:  p.CurrentTool,
			CurrentArgs:  p.CurrentArgs,
			FilesTouched: append([]string(nil), p.FilesTouched...),
			LastActivity: p.LastActivity,
			Elapsed:      elapsed,
		})
	}
}

func (m *BackgroundTaskManager) emitCompletionEvent(ctx context.Context, bt *backgroundTask) {
	if bt.emitEvent == nil || bt.baseEvent == nil {
		return
	}

	bt.mu.Lock()
	description := bt.description
	status := bt.status
	startedAt := bt.startedAt
	completedAt := bt.completedAt
	answer := ""
	iterations := 0
	tokensUsed := 0
	if bt.result != nil {
		answer = bt.result.Answer
		iterations = bt.result.Iterations
		tokensUsed = bt.result.TokensUsed
	}
	errMsg := ""
	if bt.err != nil {
		errMsg = bt.err.Error()
	}
	bt.mu.Unlock()

	duration := time.Duration(0)
	if !startedAt.IsZero() && !completedAt.IsZero() {
		duration = completedAt.Sub(startedAt)
		if duration < 0 {
			duration = 0
		}
	}

	bt.emitEvent(&domain.BackgroundTaskCompletedEvent{
		BaseEvent:   bt.baseEvent(ctx),
		TaskID:      bt.id,
		Description: description,
		Status:      string(status),
		Answer:      answer,
		Error:       errMsg,
		Duration:    duration,
		Iterations:  iterations,
		TokensUsed:  tokensUsed,
	})
}

func (m *BackgroundTaskManager) buildContextEnrichedPrompt(bt *backgroundTask) string {
	if len(bt.dependsOn) == 0 {
		return bt.prompt
	}
	var sb strings.Builder
	sb.WriteString("[Collaboration Context]\n")
	sb.WriteString("This task depends on completed tasks whose results are provided below.\n\n")
	for _, depID := range bt.dependsOn {
		m.mu.RLock()
		dep := m.tasks[depID]
		m.mu.RUnlock()
		if dep == nil {
			continue
		}
		dep.mu.Lock()
		status := dep.status
		answer := ""
		if dep.result != nil {
			answer = dep.result.Answer
		}
		errMsg := ""
		if dep.err != nil {
			errMsg = dep.err.Error()
		}
		dep.mu.Unlock()

		sb.WriteString(fmt.Sprintf("--- Task %q (%s) — %s ---\n", depID, dep.agentType, strings.ToUpper(string(status))))
		if answer != "" {
			sb.WriteString("Result summary: ")
			sb.WriteString(answer)
			sb.WriteString("\n")
		}
		if errMsg != "" {
			sb.WriteString("Error: ")
			sb.WriteString(errMsg)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("[Your Task]\n")
	sb.WriteString(bt.prompt)
	return sb.String()
}

func (m *BackgroundTaskManager) awaitDependencies(ctx context.Context, bt *backgroundTask) error {
	pollInterval := 200 * time.Millisecond
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		allDone := true
		for _, depID := range bt.dependsOn {
			m.mu.RLock()
			dep := m.tasks[depID]
			m.mu.RUnlock()
			if dep == nil {
				return fmt.Errorf("dependency %q not found", depID)
			}
			dep.mu.Lock()
			status := dep.status
			errMsg := ""
			if dep.err != nil {
				errMsg = dep.err.Error()
			}
			dep.mu.Unlock()

			switch status {
			case agent.BackgroundTaskStatusCompleted:
				// ok
			case agent.BackgroundTaskStatusFailed, agent.BackgroundTaskStatusCancelled:
				if errMsg == "" {
					errMsg = "dependency failed"
				}
				return fmt.Errorf("dependency %q failed: %s", depID, errMsg)
			default:
				allDone = false
			}
			if !allDone {
				break
			}
		}
		if allDone {
			return nil
		}
		time.Sleep(pollInterval)
	}
}

func (m *BackgroundTaskManager) validateDependencies(taskID string, deps []string) error {
	if len(deps) == 0 {
		return nil
	}
	for _, dep := range deps {
		if strings.TrimSpace(dep) == "" {
			return fmt.Errorf("dependency task id must not be empty")
		}
		if dep == taskID {
			return fmt.Errorf("task %q cannot depend on itself", taskID)
		}
		if _, ok := m.tasks[dep]; !ok {
			return fmt.Errorf("dependency %q not found", dep)
		}
	}

	graph := make(map[string][]string, len(m.tasks)+1)
	for id, task := range m.tasks {
		graph[id] = append([]string(nil), task.dependsOn...)
	}
	graph[taskID] = append([]string(nil), deps...)

	const (
		unvisited = iota
		visiting
		done
	)
	state := make(map[string]int, len(graph))
	var visit func(string) error
	visit = func(node string) error {
		switch state[node] {
		case visiting:
			return fmt.Errorf("dependency cycle detected involving %q", node)
		case done:
			return nil
		}
		state[node] = visiting
		for _, next := range graph[node] {
			if err := visit(next); err != nil {
				return err
			}
		}
		state[node] = done
		return nil
	}
	return visit(taskID)
}

func (m *BackgroundTaskManager) signalCompletion(taskID string) {
	select {
	case m.completions <- taskID:
	default:
		m.logger.Warn("background completions channel full, dropping signal for task %q", taskID)
	}
}

func resolveBackgroundEventSink(ctx context.Context, fallback backgroundEventSink) backgroundEventSink {
	sink, ok := getBackgroundEventSink(ctx)
	if !ok {
		return fallback
	}
	if sink.emitEvent == nil {
		sink.emitEvent = fallback.emitEvent
	}
	if sink.baseEvent == nil {
		sink.baseEvent = fallback.baseEvent
	}
	if sink.parentListener == nil {
		sink.parentListener = fallback.parentListener
	}
	return sink
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
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
			if bt.status == agent.BackgroundTaskStatusPending ||
				bt.status == agent.BackgroundTaskStatusRunning ||
				bt.status == agent.BackgroundTaskStatusBlocked {
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
