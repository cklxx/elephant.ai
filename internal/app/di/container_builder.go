package di

import (
	"alex/internal/shared/utils"
	"context"
	"fmt"

	agentcoordinator "alex/internal/app/agent/coordinator"
	ctxmgr "alex/internal/app/context"
	"alex/internal/app/lifecycle"
	coretape "alex/internal/core/tape"
	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	agentstorage "alex/internal/domain/agent/ports/storage"
	"alex/internal/domain/agent/presets"
	"alex/internal/infra/adapters"
	"alex/internal/infra/memory"
	sessionstate "alex/internal/infra/session/state_store"
	toolspolicy "alex/internal/infra/tools"
	"alex/internal/shared/logging"
	"alex/internal/shared/parser"
)

type containerBuilder struct {
	config        Config
	logger        logging.Logger
	sessionDir    string
	costDir       string
	cachedTapeStore coretape.TapeStore
}

type sessionResources struct {
	sessionStore agentstorage.SessionStore
	stateStore   sessionstate.Store
	historyStore sessionstate.Store
}

// BuildContainer builds the dependency injection container with the given configuration.
func BuildContainer(config Config) (*Container, error) {
	builder := newContainerBuilder(config)
	return builder.Build()
}

func newContainerBuilder(config Config) *containerBuilder {
	logger := logging.NewComponentLogger("DI")
	sessionDir := resolveStorageDir(config.SessionDir, "~/.alex/sessions")
	costDir := resolveStorageDir(config.CostDir, "~/.alex/costs")
	if utils.IsBlank(config.ToolMode) {
		config.ToolMode = string(presets.ToolModeCLI)
	}
	config.SessionDir = sessionDir
	config.CostDir = costDir

	return &containerBuilder{
		config:     config,
		logger:     logger,
		sessionDir: sessionDir,
		costDir:    costDir,
	}
}

func (b *containerBuilder) Build() (*Container, error) {
	b.logger.Debug("Building container with session_dir=%s, cost_dir=%s", b.sessionDir, b.costDir)

	llmFactory := b.buildLLMFactory()
	resources := b.buildSessionResources()
	taskStore := b.buildTaskStore()
	decisionStore, err := b.buildDecisionStore()
	if err != nil {
		return nil, fmt.Errorf("build decision store: %w", err)
	}

	bgCtx, bgCancel := context.WithCancel(context.Background())
	buildOK := false
	defer func() {
		if !buildOK {
			bgCancel()
		}
	}()
	memoryEngine := b.buildMemoryEngine(bgCtx)
	contextOptions := []ctxmgr.Option{
		ctxmgr.WithStateStore(resources.stateStore),
	}
	contextOptions = append(contextOptions, ctxmgr.WithMemoryEngine(memoryEngine))
	contextOptions = append(contextOptions, ctxmgr.WithMemoryGate(memoryGateFunc(b.config.Proactive.Memory.Enabled)))
	predCfg := b.config.Proactive.Memory.Prediction
	contextOptions = append(contextOptions, ctxmgr.WithPredictionConfig(predCfg))
	var queryTracker *memory.QueryTracker
	if predCfg.Enabled {
		memRoot := resolveStorageDir(b.config.MemoryDir, "~/.alex/memory")
		if memRoot != "" {
			queryTracker = memory.NewQueryTracker(memRoot)
			contextOptions = append(contextOptions, ctxmgr.WithQueryTracker(queryTracker))
		}
	}
	if tapeReader := b.buildTapeMessageReader(); tapeReader != nil {
		contextOptions = append(contextOptions, ctxmgr.WithTapeMessageReader(tapeReader))
	}
	contextMgr := ctxmgr.NewManager(contextOptions...)
	historyMgr := ctxmgr.NewHistoryManager(resources.historyStore, b.logger, agent.SystemClock{})
	parserImpl := parser.New()
	llmFactory.EnableToolCallParsing(parserImpl)

	costTracker, err := b.buildCostTracker()
	if err != nil {
		return nil, err
	}

	toolSLACollector, err := toolspolicy.NewSLACollector(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create SLA collector: %w", err)
	}

	toolRegistry, err := b.buildToolRegistry(llmFactory, memoryEngine, toolSLACollector)
	if err != nil {
		return nil, err
	}

	okrStore := b.buildOKRGoalStore()
	hookRuntime := b.buildHookRuntime(memoryEngine, llmFactory, okrStore, queryTracker)
	okrContextProvider := b.buildOKRContextProvider(okrStore)
	checkpointStore := b.buildCheckpointStore()
	credentialRefresher := buildCredentialRefresher()
	tapeMgr := b.buildTapeManager()

	coordinator := agentcoordinator.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		resources.sessionStore,
		contextMgr,
		historyMgr,
		parserImpl,
		costTracker,
		b.buildAgentAppConfig(),
		agentcoordinator.WithHookRuntime(hookRuntime),
		agentcoordinator.WithOKRContextProvider(okrContextProvider),
		agentcoordinator.WithCheckpointStore(checkpointStore),
		agentcoordinator.WithCredentialRefresher(credentialRefresher),
		agentcoordinator.WithToolSLACollector(toolSLACollector),
		agentcoordinator.WithChannelHints(channels.DefaultHints()),
		agentcoordinator.WithAtomicWriter(adapters.NewOSAtomicWriter()),
		agentcoordinator.WithTurnRecorder(b.buildTurnRecorder(tapeMgr)),
		agentcoordinator.WithTapeManager(tapeMgr),
	)

	b.logger.Info("Container built successfully (heavy initialization deferred to Start())")

	container := &Container{
		AgentCoordinator: coordinator,
		StorageResources: StorageResources{
			SessionStore:    resources.sessionStore,
			StateStore:      resources.stateStore,
			HistoryStore:    resources.historyStore,
			HistoryManager:  historyMgr,
			CostTracker:     costTracker,
			MemoryEngine:    memoryEngine,
			CheckpointStore: checkpointStore,
			TaskStore:       taskStore,
			DecisionStore:   decisionStore,
		},
		TapeManager:  tapeMgr,
		config:       b.config,
		toolRegistry: toolRegistry,
		llmFactory:   llmFactory,
		bgCancel:     bgCancel,
	}
	if drainable, ok := memoryEngine.(lifecycle.Drainable); ok {
		container.Drainables = append(container.Drainables, drainable)
	}
	if taskStoreCloser, ok := taskStore.(interface{ Close() }); ok {
		container.Drainables = append(container.Drainables, lifecycle.DrainFunc{
			DrainName: "task-store",
			Fn:        func(context.Context) { taskStoreCloser.Close() },
		})
	}

	buildOK = true
	return container, nil
}

