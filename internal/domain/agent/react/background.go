package react

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
)

// backgroundTask tracks an individual background task.
type backgroundTask struct {
	mu            sync.Mutex
	id            string
	description   string
	prompt        string
	agentType     string
	executionMode string
	autonomyLevel string
	causationID   string
	status        agent.BackgroundTaskStatus
	startedAt     time.Time
	completedAt   time.Time
	result        *agent.TaskResult
	err           error
	taskCancel    context.CancelFunc // per-task cancel; nil until Dispatch runs the goroutine
	// completionSignaled flips once signalCompletion is invoked. AwaitAll uses
	// this to avoid returning before completion events are emitted.
	completionSignaled bool

	emitEvent      func(agent.AgentEvent)
	baseEvent      func(context.Context) domain.BaseEvent
	parentListener agent.EventListener
	notifyParent   func(event agent.AgentEvent) // direct bypass for completion events

	progress         *agent.ExternalAgentProgress
	pendingInput     *agent.InputRequestSummary
	lastProgressEmit time.Time
	lastActivityAt   time.Time
	dependsOn        []string
	inheritContext   bool
	workspace        *agent.WorkspaceAllocation
	fileScope        []string
	config           map[string]string
	mergeStatus      string
}

// BackgroundTaskManager manages background task lifecycle within a single run.
// It implements agent.BackgroundTaskDispatcher.
type BackgroundTaskManager struct {
	mu           sync.RWMutex
	tasks        map[string]*backgroundTask
	completedMu  sync.Mutex
	completedIDs []string
	activeTasks  atomic.Int64
	depNotify    chan struct{}
	depMu        sync.Mutex
	logger       agent.Logger
	clock        agent.Clock
	taskCtx      context.Context
	cancelAll    context.CancelFunc
	runCtx       context.Context // for value inheritance (IDs, etc.)
	workingDir   string
	workspaceMgr agent.WorkspaceManager
	idGenerator  agent.IDGenerator
	idContext    agent.IDContextReader
	goRunner     agent.GoRunnerFunc

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

	sessionID          string
	parentListener     agent.EventListener
	maxConcurrentTasks int
	staleThreshold     time.Duration

	// contextPropagators copy app-layer context values into detached background task contexts.
	contextPropagators []agent.ContextPropagatorFunc

	tmuxSender    agent.TmuxSender
	eventAppender agent.EventAppender
}

const defaultStaleThreshold = 15 * time.Minute

var ErrBackgroundTaskNotFound = errors.New("background task not found")

// BackgroundManagerConfig configures a shared background task manager.
type BackgroundManagerConfig struct {
	RunContext          context.Context
	Logger              agent.Logger
	Clock               agent.Clock
	IDGenerator         agent.IDGenerator
	IDContextReader     agent.IDContextReader
	GoRunner            agent.GoRunnerFunc
	WorkingDirResolver  agent.WorkingDirResolverFunc
	WorkspaceMgrFactory agent.WorkspaceManagerFactoryFunc
	ExecuteTask         func(ctx context.Context, prompt, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
	ExternalExecutor    agent.ExternalAgentExecutor
	SessionID           string
	MaxConcurrentTasks  int
	StaleThreshold      time.Duration

	// ContextPropagators are called during Dispatch to copy app-layer context values
	// (e.g. LLM selection) from the dispatch context into the detached task context.
	ContextPropagators []agent.ContextPropagatorFunc

	TmuxSender    agent.TmuxSender
	EventAppender agent.EventAppender
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
		0,
	)
}

func newBackgroundTaskManagerWithDeps(
	runCtx context.Context,
	logger agent.Logger,
	clock agent.Clock,
	idGenerator agent.IDGenerator,
	idContextReader agent.IDContextReader,
	goRunner agent.GoRunnerFunc,
	workingDirResolver agent.WorkingDirResolverFunc,
	workspaceMgrFactory agent.WorkspaceManagerFactoryFunc,
	executeTask func(ctx context.Context, prompt, sessionID string,
		listener agent.EventListener) (*agent.TaskResult, error),
	externalExecutor agent.ExternalAgentExecutor,
	emitEvent func(event agent.AgentEvent),
	baseEvent func(ctx context.Context) domain.BaseEvent,
	sessionID string,
	parentListener agent.EventListener,
	maxConcurrentTasks int,
) *BackgroundTaskManager {
	if idGenerator == nil {
		idGenerator = defaultIDGenerator{}
	}
	if idContextReader == nil {
		idContextReader = defaultIDContextReader{}
	}
	if goRunner == nil {
		goRunner = func(_ agent.Logger, _ string, fn func()) { go fn() }
	}
	if workingDirResolver == nil {
		workingDirResolver = func(context.Context) string { return "." }
	}
	if workspaceMgrFactory == nil {
		workspaceMgrFactory = func(string, agent.Logger) agent.WorkspaceManager { return nil }
	}

	taskCtx, cancel := context.WithCancel(context.Background())
	workingDir := ""
	if workingDirResolver != nil {
		workingDir = strings.TrimSpace(workingDirResolver(runCtx))
	}
	if workingDir == "" {
		// Keep workspace features available even when cwd probing fails in long-running test/process setups.
		workingDir = "."
	}
	var workspaceMgr agent.WorkspaceManager
	if workspaceMgrFactory != nil && workingDir != "" {
		workspaceMgr = workspaceMgrFactory(workingDir, logger)
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
		tasks:              make(map[string]*backgroundTask),
		depNotify:          make(chan struct{}),
		logger:             logger,
		clock:              clock,
		taskCtx:            taskCtx,
		cancelAll:          cancel,
		runCtx:             runCtx,
		workingDir:         workingDir,
		workspaceMgr:       workspaceMgr,
		idGenerator:        idGenerator,
		idContext:          idContextReader,
		goRunner:           goRunner,
		executeTask:        executeTask,
		externalExecutor:   externalExecutor,
		inputExecutor:      inputExecutor,
		externalInputCh:    externalInputCh,
		emitEvent:          emitEvent,
		baseEvent:          baseEvent,
		sessionID:          sessionID,
		parentListener:     parentListener,
		maxConcurrentTasks: maxConcurrentTasks,
		staleThreshold:     defaultStaleThreshold,
	}

	if inputExecutor != nil && externalInputCh != nil {
		goRunner(logger, "bg.externalInput", func() {
			manager.forwardExternalInputRequests()
		})
	}

	return manager
}

// NewBackgroundTaskManager creates a background task manager intended for reuse (e.g., per session).
func NewBackgroundTaskManager(cfg BackgroundManagerConfig) *BackgroundTaskManager {
	mgr := newBackgroundTaskManagerWithDeps(
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
		cfg.MaxConcurrentTasks,
	)
	if cfg.StaleThreshold > 0 {
		mgr.staleThreshold = cfg.StaleThreshold
	}
	mgr.contextPropagators = cfg.ContextPropagators
	mgr.tmuxSender = cfg.TmuxSender
	mgr.eventAppender = cfg.EventAppender
	return mgr
}

// isStale reports whether a running task has not shown activity within the threshold.
func isStale(now, lastActivity time.Time, threshold time.Duration) bool {
	if lastActivity.IsZero() || threshold <= 0 {
		return false
	}
	return now.Sub(lastActivity) > threshold
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
	if sink.notifyParent == nil {
		sink.notifyParent = fallback.notifyParent
	}
	return sink
}
