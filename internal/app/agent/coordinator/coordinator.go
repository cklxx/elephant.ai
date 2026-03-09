package coordinator

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	appconfig "alex/internal/app/agent/config"
	"alex/internal/app/agent/cost"
	"alex/internal/app/agent/hooks"
	"alex/internal/app/agent/preparation"
	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	react "alex/internal/domain/agent/react"
	"alex/internal/domain/agent/types"
	materialports "alex/internal/domain/materialregistry/ports"
	toolspolicy "alex/internal/infra/tools"
	"alex/internal/infra/tools/builtin/shared"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	utils "alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

type RuntimeConfigResolver func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error)

// AgentCoordinator manages session lifecycle and delegates to domain
type AgentCoordinator struct {
	llmFactory       llm.LLMClientFactory
	toolRegistry     tools.ToolRegistry
	sessionStore     storage.SessionStore
	contextMgr       agent.ContextManager
	historyMgr       storage.HistoryManager
	parser           agent.FunctionCallParser
	costTracker      storage.CostTracker
	config           appconfig.Config
	runtimeResolver  RuntimeConfigResolver
	logger           agent.Logger
	clock            agent.Clock
	externalExecutor agent.ExternalAgentExecutor
	bgRegistry       *backgroundTaskRegistry
	iterationHook    agent.IterationHook
	teamDefinitions  []agent.TeamDefinition
	teamRunRecorder  agent.TeamRunRecorder
	checkpointStore  react.CheckpointStore
	atomicWriter     agent.AtomicFileWriter

	prepService         preparationService
	costDecorator       *cost.CostTrackingDecorator
	attachmentMigrator  materialports.Migrator
	attachmentPersister ports.AttachmentPersister
	hookRegistry        *hooks.Registry
	okrContextProvider  preparation.OKRContextProvider
	credentialRefresher preparation.CredentialRefresher
	channelHints        map[string]string
	timerManager        shared.TimerManagerService // injected at bootstrap; tools retrieve via shared.TimerManagerFromContext
	schedulerService    any                        // injected at bootstrap; tools retrieve via shared.SchedulerFromContext
	toolSLACollector    *toolspolicy.SLACollector

	sessionSaveMu      sync.Mutex                      // Protects concurrent session saves
	pendingSessionSave atomic.Pointer[storage.Session] // latest snapshot awaiting save
	sessionSaveOnce    sync.Once                       // Ensures single save loop goroutine
}

type preparationService interface {
	Prepare(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error)
	SetEnvironmentSummary(summary string)
	ResolveAgentPreset(ctx context.Context, preset string) string
	ResolveToolPreset(ctx context.Context, preset string) string
}

func NewAgentCoordinator(
	llmFactory llm.LLMClientFactory,
	toolRegistry tools.ToolRegistry,
	sessionStore storage.SessionStore,
	contextMgr agent.ContextManager,
	historyManager storage.HistoryManager,
	parser agent.FunctionCallParser,
	costTracker storage.CostTracker,
	config appconfig.Config,
	opts ...CoordinatorOption,
) *AgentCoordinator {
	if len(config.StopSequences) > 0 {
		config.StopSequences = append([]string(nil), config.StopSequences...)
	}
	config.LLMProfile = config.DefaultLLMProfile()

	coordinator := &AgentCoordinator{
		llmFactory:   llmFactory,
		toolRegistry: toolRegistry,
		sessionStore: sessionStore,
		contextMgr:   contextMgr,
		historyMgr:   historyManager,
		parser:       parser,
		costTracker:  costTracker,
		config:       config,
		logger:       logging.NewComponentLogger("Coordinator"),
		clock:        agent.SystemClock{},
		bgRegistry:   newBackgroundTaskRegistry(),
	}

	for _, opt := range opts {
		opt(coordinator)
	}

	// Create services only if not provided via options
	if coordinator.costDecorator == nil {
		coordinator.costDecorator = cost.NewCostTrackingDecorator(costTracker, coordinator.logger, coordinator.clock)
	}

	coordinator.prepService = preparation.NewExecutionPreparationService(preparation.ExecutionPreparationDeps{
		LLMFactory:          llmFactory,
		ToolRegistry:        toolRegistry,
		SessionStore:        sessionStore,
		ContextMgr:          contextMgr,
		HistoryMgr:          historyManager,
		Parser:              parser,
		Config:              config,
		Logger:              coordinator.logger,
		Clock:               coordinator.clock,
		CostDecorator:       coordinator.costDecorator,
		CostTracker:         coordinator.costTracker,
		OKRContextProvider:  coordinator.okrContextProvider,
		CredentialRefresher: coordinator.credentialRefresher,
		ChannelHints:        coordinator.channelHints,
	})

	if coordinator.contextMgr != nil {
		if err := coordinator.contextMgr.Preload(context.Background()); err != nil {
			coordinator.logger.Warn("Context preload failed: %v", err)
		}
	}

	return coordinator
}

type planSessionTitleRecorder struct {
	mu      sync.RWMutex
	title   string
	sink    agent.EventListener
	onTitle func(string)
}

func (r *planSessionTitleRecorder) OnEvent(event agent.AgentEvent) {
	if event == nil {
		return
	}

	if e, ok := event.(*domain.Event); ok && e.Kind == types.EventToolCompleted {
		if e.Data.Error == nil && strings.EqualFold(strings.TrimSpace(e.Data.ToolName), "plan") {
			if title := extractPlanSessionTitle(e.Data.Metadata); title != "" {
				shouldNotify := false
				r.mu.Lock()
				if r.title == "" {
					r.title = title
					shouldNotify = true
				}
				r.mu.Unlock()
				if shouldNotify && r.onTitle != nil {
					r.onTitle(title)
				}
			}
		}
	}

	if r.sink != nil {
		r.sink.OnEvent(event)
	}
}

func (r *planSessionTitleRecorder) Title() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.title
}

func extractPlanSessionTitle(metadata map[string]any) string {
	if metadata == nil {
		return ""
	}

	if raw, ok := metadata["session_title"].(string); ok {
		if title := utils.NormalizeSessionTitle(raw); title != "" {
			return title
		}
	}

	if raw, ok := metadata["overall_goal_ui"].(string); ok {
		return utils.NormalizeSessionTitle(raw)
	}

	return ""
}

func (c *AgentCoordinator) loggerFor(ctx context.Context) agent.Logger {
	return logging.FromContext(ctx, c.logger)
}

// resolveUserID extracts a user identifier from the session metadata.
func (c *AgentCoordinator) resolveUserID(ctx context.Context, session *storage.Session) string {
	if ctx != nil {
		if uid := id.UserIDFromContext(ctx); uid != "" {
			return uid
		}
	}
	if session == nil || session.Metadata == nil {
		return ""
	}
	if uid := strings.TrimSpace(session.Metadata["user_id"]); uid != "" {
		return uid
	}
	// Fallback: use session ID prefix for Lark sessions
	if strings.HasPrefix(session.ID, "lark-") {
		return session.ID
	}
	return ""
}

// PrepareExecution prepares the execution environment without running the task
func (c *AgentCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	return c.prepareExecutionWithListener(ctx, task, sessionID, nil, c.effectiveConfig(ctx))
}

// prepareExecutionWithListener prepares execution with event emission support
func (c *AgentCoordinator) prepareExecutionWithListener(ctx context.Context, task string, sessionID string, listener agent.EventListener, cfg appconfig.Config) (*agent.ExecutionEnvironment, error) {
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	if listener == nil {
		if _, ok := c.prepService.(*preparation.ExecutionPreparationService); !ok && c.prepService != nil {
			return c.prepService.Prepare(ctx, task, sessionID)
		}
	}
	logger := c.loggerFor(ctx)
	prepService := preparation.NewExecutionPreparationService(preparation.ExecutionPreparationDeps{
		LLMFactory:          c.llmFactory,
		ToolRegistry:        c.toolRegistry,
		SessionStore:        c.sessionStore,
		ContextMgr:          c.contextMgr,
		HistoryMgr:          c.historyMgr,
		Parser:              c.parser,
		Config:              cfg,
		Logger:              logger,
		Clock:               c.clock,
		CostDecorator:       c.costDecorator,
		EventEmitter:        listener,
		CostTracker:         c.costTracker,
		OKRContextProvider:  c.okrContextProvider,
		CredentialRefresher: c.credentialRefresher,
		ChannelHints:        c.channelHints,
	})
	return prepService.Prepare(ctx, task, sessionID)
}
