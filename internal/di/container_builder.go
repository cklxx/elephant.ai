package di

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "alex/internal/agent/app/config"
	agentcoordinator "alex/internal/agent/app/coordinator"
	agentcost "alex/internal/agent/app/cost"
	"alex/internal/agent/app/hooks"
	"alex/internal/agent/app/preparation"
	agent "alex/internal/agent/ports/agent"
	agentstorage "alex/internal/agent/ports/storage"
	"alex/internal/agent/presets"
	"alex/internal/analytics/journal"
	runtimeconfig "alex/internal/config"
	ctxmgr "alex/internal/context"
	"alex/internal/external"
	"alex/internal/llm"
	"alex/internal/logging"
	"alex/internal/mcp"
	"alex/internal/memory"
	"alex/internal/parser"
	"alex/internal/rag"
	"alex/internal/session/filestore"
	"alex/internal/session/postgresstore"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/storage"
	toolregistry "alex/internal/toolregistry"
	okrtools "alex/internal/tools/builtin/okr"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

type containerBuilder struct {
	config     Config
	logger     logging.Logger
	sessionDir string
	costDir    string
}

type sessionResources struct {
	sessionStore agentstorage.SessionStore
	stateStore   sessionstate.Store
	historyStore sessionstate.Store
	sessionDB    *pgxpool.Pool
}

type sessionPoolOptions struct {
	maxConns       int
	minConns       int
	maxLifetime    time.Duration
	maxIdle        time.Duration
	healthCheck    time.Duration
	connectTimeout time.Duration
}

type postgresInitError struct {
	step string
	err  error
}

func (e postgresInitError) Error() string {
	return fmt.Sprintf("failed to %s: %v", e.step, e.err)
}

func (e postgresInitError) Unwrap() error {
	return e.err
}

// BuildContainer builds the dependency injection container with the given configuration.
// Heavy initialization (MCP) is deferred until Start() is called.
func BuildContainer(config Config) (*Container, error) {
	builder := newContainerBuilder(config)
	return builder.Build()
}

func newContainerBuilder(config Config) *containerBuilder {
	logger := logging.NewComponentLogger("DI")
	sessionDir := resolveStorageDir(config.SessionDir, "~/.alex/sessions")
	costDir := resolveStorageDir(config.CostDir, "~/.alex/costs")
	if strings.TrimSpace(config.ToolMode) == "" {
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
	resources, err := b.buildSessionResources()
	if err != nil {
		return nil, err
	}

	memoryService, err := b.buildMemoryService(resources)
	if err != nil {
		return nil, err
	}
	journalWriter := b.buildJournalWriter()
	contextMgr := ctxmgr.NewManager(
		ctxmgr.WithStateStore(resources.stateStore),
		ctxmgr.WithJournalWriter(journalWriter),
	)
	historyMgr := ctxmgr.NewHistoryManager(resources.historyStore, b.logger, agent.SystemClock{})
	parserImpl := parser.New()
	llmFactory.EnableToolCallParsing(parserImpl)

	costTracker, err := b.buildCostTracker()
	if err != nil {
		return nil, err
	}

	toolRegistry, err := b.buildToolRegistry(llmFactory, memoryService)
	if err != nil {
		return nil, err
	}

	var externalExecutor agent.ExternalAgentExecutor
	externalRegistry := external.NewRegistry(b.config.ExternalAgents, b.logger)
	if len(externalRegistry.SupportedTypes()) > 0 {
		externalExecutor = externalRegistry
	}

	mcpRegistry := mcp.NewRegistry()
	tracker := newMCPInitializationTracker()

	hookRegistry := b.buildHookRegistry(memoryService)
	iterationHook := b.buildIterationHook(memoryService)
	okrContextProvider := b.buildOKRContextProvider()

	coordinator := agentcoordinator.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		resources.sessionStore,
		contextMgr,
		historyMgr,
		parserImpl,
		costTracker,
		appconfig.Config{
			LLMProvider:         b.config.LLMProvider,
			LLMModel:            b.config.LLMModel,
			LLMSmallProvider:    b.config.LLMSmallProvider,
			LLMSmallModel:       b.config.LLMSmallModel,
			LLMVisionModel:      b.config.LLMVisionModel,
			APIKey:              b.config.APIKey,
			BaseURL:             b.config.BaseURL,
			MaxTokens:           b.config.MaxTokens,
			MaxIterations:       b.config.MaxIterations,
			ToolMaxConcurrent:   b.config.ToolMaxConcurrent,
			Temperature:         b.config.Temperature,
			TemperatureProvided: b.config.TemperatureProvided,
			TopP:                b.config.TopP,
			StopSequences:       append([]string(nil), b.config.StopSequences...),
			AgentPreset:         b.config.AgentPreset,
			ToolPreset:          b.config.ToolPreset,
			ToolMode:            b.config.ToolMode,
			EnvironmentSummary:  b.config.EnvironmentSummary,
			SessionStaleAfter:   b.config.SessionStaleAfter,
			Proactive:           b.config.Proactive,
		},
		agentcoordinator.WithHookRegistry(hookRegistry),
		agentcoordinator.WithIterationHook(iterationHook),
		agentcoordinator.WithMemoryService(memoryService),
		agentcoordinator.WithExternalExecutor(externalExecutor),
		agentcoordinator.WithOKRContextProvider(okrContextProvider),
	)

	// Register subagent tool after coordinator is created.
	toolRegistry.RegisterSubAgent(coordinator)

	b.logger.Info("Container built successfully (heavy initialization deferred to Start())")

	return &Container{
		AgentCoordinator: coordinator,
		SessionStore:     resources.sessionStore,
		StateStore:       resources.stateStore,
		HistoryStore:     resources.historyStore,
		HistoryManager:   historyMgr,
		CostTracker:      costTracker,
		MemoryService:    memoryService,
		MCPRegistry:      mcpRegistry,
		mcpInitTracker:   tracker,
		SessionDB:        resources.sessionDB,
		config:           b.config,
		toolRegistry:     toolRegistry,
		llmFactory:       llmFactory,
		mcpStarted:       false,
	}, nil
}

func (b *containerBuilder) buildLLMFactory() *llm.Factory {
	llmFactory := llm.NewFactory()
	llmFactory.SetCacheOptions(b.config.LLMCacheSize, b.config.LLMCacheTTL)
	if b.config.UserRateLimitRPS > 0 {
		llmFactory.EnableUserRateLimit(rate.Limit(b.config.UserRateLimitRPS), b.config.UserRateLimitBurst)
	}
	return llmFactory
}

func (b *containerBuilder) buildSessionResources() (sessionResources, error) {
	dbURL := strings.TrimSpace(b.config.SessionDatabaseURL)
	if b.config.RequireSessionDatabase && dbURL == "" {
		return sessionResources{}, fmt.Errorf("session database is required but no database URL is configured")
	}

	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resources, err := b.buildPostgresResources(ctx, dbURL)
		if err == nil {
			b.logger.Info("Session persistence backed by Postgres")
			return resources, nil
		}
		if b.config.RequireSessionDatabase {
			return sessionResources{}, err
		}
		b.logPostgresFailure(err)
	}

	return sessionResources{
		sessionStore: filestore.New(b.sessionDir),
		stateStore:   sessionstate.NewFileStore(filepath.Join(b.sessionDir, "snapshots")),
		historyStore: sessionstate.NewFileStore(filepath.Join(b.sessionDir, "turns")),
	}, nil
}

func (b *containerBuilder) buildPostgresResources(ctx context.Context, dbURL string) (sessionResources, error) {
	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return sessionResources{}, postgresInitError{step: "parse session DB config", err: err}
	}

	applySessionPoolOptions(poolConfig, resolveSessionPoolOptions(b.config))

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return sessionResources{}, postgresInitError{step: "create session DB pool", err: err}
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return sessionResources{}, postgresInitError{step: "ping session DB", err: err}
	}

	var dbSessionStore *postgresstore.Store
	if b.config.SessionCacheSize != nil {
		dbSessionStore = postgresstore.New(pool, postgresstore.WithCacheSize(*b.config.SessionCacheSize))
	} else {
		dbSessionStore = postgresstore.New(pool)
	}
	if err := dbSessionStore.EnsureSchema(ctx); err != nil {
		pool.Close()
		return sessionResources{}, postgresInitError{step: "initialize session schema", err: err}
	}

	dbStateStore := sessionstate.NewPostgresStore(pool, sessionstate.SnapshotKindState)
	if err := dbStateStore.EnsureSchema(ctx); err != nil {
		pool.Close()
		return sessionResources{}, postgresInitError{step: "initialize snapshot schema", err: err}
	}

	dbHistoryStore := sessionstate.NewPostgresStore(pool, sessionstate.SnapshotKindTurn)
	if err := dbHistoryStore.EnsureSchema(ctx); err != nil {
		pool.Close()
		return sessionResources{}, postgresInitError{step: "initialize history schema", err: err}
	}

	return sessionResources{
		sessionStore: dbSessionStore,
		stateStore:   dbStateStore,
		historyStore: dbHistoryStore,
		sessionDB:    pool,
	}, nil
}

func (b *containerBuilder) logPostgresFailure(err error) {
	var perr postgresInitError
	if errors.As(err, &perr) {
		b.logger.Warn("Failed to %s: %v", perr.step, perr.err)
		return
	}
	b.logger.Warn("Failed to initialize session DB: %v", err)
}

func resolveSessionPoolOptions(config Config) sessionPoolOptions {
	options := sessionPoolOptions{
		maxConns:       defaultSessionPoolMaxConns,
		minConns:       defaultSessionPoolMinConns,
		maxLifetime:    defaultSessionPoolMaxConnLifetime,
		maxIdle:        defaultSessionPoolMaxConnIdleTime,
		healthCheck:    defaultSessionPoolHealthCheckPeriod,
		connectTimeout: defaultSessionPoolConnectTimeout,
	}
	if config.SessionPoolMaxConns > 0 {
		options.maxConns = config.SessionPoolMaxConns
	}
	if config.SessionPoolMinConns > 0 {
		options.minConns = config.SessionPoolMinConns
	}
	if config.SessionPoolMaxConnLifetime > 0 {
		options.maxLifetime = config.SessionPoolMaxConnLifetime
	}
	if config.SessionPoolMaxConnIdleTime > 0 {
		options.maxIdle = config.SessionPoolMaxConnIdleTime
	}
	if config.SessionPoolHealthCheckPeriod > 0 {
		options.healthCheck = config.SessionPoolHealthCheckPeriod
	}
	if config.SessionPoolConnectTimeout > 0 {
		options.connectTimeout = config.SessionPoolConnectTimeout
	}
	return options
}

func applySessionPoolOptions(poolConfig *pgxpool.Config, options sessionPoolOptions) {
	poolConfig.MaxConns = int32(options.maxConns)
	poolConfig.MinConns = int32(options.minConns)
	poolConfig.MaxConnLifetime = options.maxLifetime
	poolConfig.MaxConnIdleTime = options.maxIdle
	poolConfig.HealthCheckPeriod = options.healthCheck
	poolConfig.ConnConfig.ConnectTimeout = options.connectTimeout
	poolConfig.ConnConfig.StatementCacheCapacity = defaultSessionStatementCache
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
}

func (b *containerBuilder) buildMemoryService(resources sessionResources) (memory.Service, error) {
	store, err := b.buildMemoryStore(resources)
	if err != nil {
		return nil, err
	}
	if err := store.EnsureSchema(context.Background()); err != nil {
		b.logger.Warn("Failed to initialize memory store schema: %v", err)
	}

	retentionPolicy := buildMemoryRetentionPolicy(b.config.Proactive.Memory.Retention)
	if b.config.Proactive.Memory.Retention.PruneOnStart && retentionPolicy.HasRules() {
		if deleted, err := store.Prune(context.Background(), retentionPolicy); err != nil {
			b.logger.Warn("Failed to prune memory store: %v", err)
		} else if len(deleted) > 0 {
			b.logger.Info("Pruned %d expired memories on startup", len(deleted))
		}
	}

	return memory.NewServiceWithRetention(store, retentionPolicy), nil
}

func (b *containerBuilder) buildMemoryStore(resources sessionResources) (memory.Store, error) {
	mode := strings.ToLower(strings.TrimSpace(b.config.Proactive.Memory.Store))
	if mode == "" || mode == "auto" {
		return b.buildAutoMemoryStore(resources)
	}
	switch mode {
	case "file":
		return memory.NewFileStore(resolveStorageDir(b.config.MemoryDir, "~/.alex/memory")), nil
	case "postgres":
		if resources.sessionDB == nil {
			return nil, fmt.Errorf("memory store postgres requires session database")
		}
		return memory.NewPostgresStore(resources.sessionDB), nil
	case "hybrid":
		return b.buildHybridMemoryStore(resources)
	default:
		return nil, fmt.Errorf("unknown memory store %q", mode)
	}
}

func (b *containerBuilder) buildAutoMemoryStore(resources sessionResources) (memory.Store, error) {
	if resources.sessionDB != nil {
		return memory.NewPostgresStore(resources.sessionDB), nil
	}
	return memory.NewFileStore(resolveStorageDir(b.config.MemoryDir, "~/.alex/memory")), nil
}

func (b *containerBuilder) buildHybridMemoryStore(resources sessionResources) (memory.Store, error) {
	keywordStore, err := b.buildAutoMemoryStore(resources)
	if err != nil {
		return nil, err
	}

	hybridCfg := b.config.Proactive.Memory.Hybrid
	persistDir := strings.TrimSpace(hybridCfg.PersistDir)
	if persistDir == "" {
		persistDir = filepath.Join(resolveStorageDir(b.config.MemoryDir, "~/.alex/memory"), "vector")
	}
	if err := os.MkdirAll(persistDir, 0o755); err != nil {
		b.logger.Warn("Failed to create memory vector dir: %v", err)
	}

	baseURL := strings.TrimSpace(hybridCfg.EmbedderBaseURL)
	if baseURL == "" {
		baseURL = strings.TrimSpace(b.config.BaseURL)
	}
	embedder, err := rag.NewEmbedder(rag.EmbedderConfig{
		Provider:  "openai",
		Model:     strings.TrimSpace(hybridCfg.EmbedderModel),
		APIKey:    b.config.APIKey,
		BaseURL:   baseURL,
		CacheSize: 10000,
	})
	if err != nil {
		return nil, fmt.Errorf("init memory embedder: %w", err)
	}

	vectorStore, err := rag.NewVectorStore(rag.StoreConfig{
		PersistPath: persistDir,
		Collection:  strings.TrimSpace(hybridCfg.Collection),
	}, embedder)
	if err != nil {
		return nil, fmt.Errorf("init memory vector store: %w", err)
	}

	return memory.NewHybridStore(keywordStore, vectorStore, embedder, hybridCfg.Alpha, float32(hybridCfg.MinSimilarity), hybridCfg.AllowVectorFailures), nil
}

func buildMemoryRetentionPolicy(cfg runtimeconfig.MemoryRetentionConfig) memory.RetentionPolicy {
	policy := memory.RetentionPolicy{
		PruneOnRecall: cfg.PruneOnRecall,
	}
	if cfg.DefaultDays > 0 {
		policy.DefaultTTL = time.Duration(cfg.DefaultDays) * 24 * time.Hour
	}

	typeTTL := map[string]time.Duration{}
	if cfg.AutoCaptureDays > 0 {
		typeTTL["auto_capture"] = time.Duration(cfg.AutoCaptureDays) * 24 * time.Hour
	}
	if cfg.ChatTurnDays > 0 {
		typeTTL["chat_turn"] = time.Duration(cfg.ChatTurnDays) * 24 * time.Hour
	}
	if cfg.WorkflowTraceDays > 0 {
		typeTTL["workflow_trace"] = time.Duration(cfg.WorkflowTraceDays) * 24 * time.Hour
	}
	if len(typeTTL) > 0 {
		policy.TypeTTL = typeTTL
	}

	return policy
}

func (b *containerBuilder) buildJournalWriter() journal.Writer {
	journalDir := filepath.Join(b.sessionDir, "journals")
	fileWriter, err := journal.NewFileWriter(journalDir)
	if err != nil {
		b.logger.Warn("Failed to initialize journal writer: %v", err)
		return journal.NopWriter()
	}
	return fileWriter
}

func (b *containerBuilder) buildCostTracker() (agentstorage.CostTracker, error) {
	costStore, err := storage.NewFileCostStore(b.costDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost store: %w", err)
	}
	return agentcost.NewCostTracker(costStore), nil
}

func (b *containerBuilder) buildHookRegistry(memoryService memory.Service) *hooks.Registry {
	registry := hooks.NewRegistry(b.logger)
	if !b.config.Proactive.Enabled {
		b.logger.Info("Non-memory proactive hooks disabled by config")
	}

	// Register memory hooks (behavior gated by MemoryPolicy, not config).
	if memoryService != nil {
		recallHook := hooks.NewMemoryRecallHook(memoryService, b.logger, hooks.MemoryRecallConfig{
			MaxRecalls:         b.config.Proactive.Memory.MaxRecalls,
			CaptureGroupMemory: b.config.Proactive.Memory.CaptureGroupMemory,
		})
		registry.Register(recallHook)

		captureHook := hooks.NewMemoryCaptureHook(memoryService, b.logger, hooks.MemoryCaptureConfig{
			DedupeThreshold: b.config.Proactive.Memory.DedupeThreshold,
		})
		registry.Register(captureHook)

		convHook := hooks.NewConversationCaptureHook(memoryService, b.logger, hooks.ConversationCaptureConfig{
			CaptureGroupMemory: b.config.Proactive.Memory.CaptureGroupMemory,
			DedupeThreshold:    b.config.Proactive.Memory.DedupeThreshold,
		})
		registry.Register(convHook)
	}

	// Register OKR context hook (pre-task OKR injection)
	if b.config.Proactive.Enabled && b.config.Proactive.OKR.Enabled {
		okrCfg := okrtools.DefaultOKRConfig()
		if goalsRoot := b.config.Proactive.OKR.GoalsRoot; goalsRoot != "" {
			okrCfg.GoalsRoot = resolveStorageDir(goalsRoot, okrCfg.GoalsRoot)
		}
		okrStore := okrtools.NewGoalStore(okrCfg)
		okrHook := hooks.NewOKRContextHook(okrStore, b.logger, hooks.OKRContextConfig{
			Enabled:    b.config.Proactive.OKR.Enabled,
			AutoInject: b.config.Proactive.OKR.AutoInject,
		})
		registry.Register(okrHook)
	}

	b.logger.Info("Hook registry built with %d hooks", registry.HookCount())
	return registry
}

func (b *containerBuilder) buildIterationHook(memoryService memory.Service) agent.IterationHook {
	if memoryService == nil {
		return nil
	}
	return hooks.NewIterationRefreshHook(memoryService, b.logger, hooks.IterationRefreshConfig{
		DefaultInterval: b.config.Proactive.Memory.RefreshInterval,
		MaxTokens:       b.config.Proactive.Memory.MaxRefreshTokens,
	})
}

func (b *containerBuilder) buildToolRegistry(factory *llm.Factory, memoryService memory.Service) (*toolregistry.Registry, error) {
	toolRegistry, err := toolregistry.NewRegistry(toolregistry.Config{
		TavilyAPIKey:               b.config.TavilyAPIKey,
		MoltbookAPIKey:             b.config.MoltbookAPIKey,
		MoltbookBaseURL:            b.config.MoltbookBaseURL,
		ArkAPIKey:                  b.config.ArkAPIKey,
		LLMFactory:                 factory,
		LLMProvider:                b.config.LLMProvider,
		LLMModel:                   b.config.LLMModel,
		LLMVisionModel:             b.config.LLMVisionModel,
		APIKey:                     b.config.APIKey,
		BaseURL:                    b.config.BaseURL,
		SandboxBaseURL:             b.config.SandboxBaseURL,
		ACPExecutorAddr:            b.config.ACPExecutorAddr,
		ACPExecutorCWD:             b.config.ACPExecutorCWD,
		ACPExecutorMode:            b.config.ACPExecutorMode,
		ACPExecutorAutoApprove:     b.config.ACPExecutorAutoApprove,
		ACPExecutorMaxCLICalls:     b.config.ACPExecutorMaxCLICalls,
		ACPExecutorMaxDuration:     b.config.ACPExecutorMaxDuration,
		ACPExecutorRequireManifest: b.config.ACPExecutorRequireManifest,
		SeedreamTextEndpointID:     b.config.SeedreamTextEndpointID,
		SeedreamImageEndpointID:    b.config.SeedreamImageEndpointID,
		SeedreamTextModel:          b.config.SeedreamTextModel,
		SeedreamImageModel:         b.config.SeedreamImageModel,
		SeedreamVisionModel:        b.config.SeedreamVisionModel,
		SeedreamVideoModel:         b.config.SeedreamVideoModel,
		MemoryService:              memoryService,
		OKRGoalsRoot:               b.resolveOKRGoalsRoot(),
		HTTPLimits:                 b.config.HTTPLimits,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}
	return toolRegistry, nil
}

func (b *containerBuilder) resolveOKRGoalsRoot() string {
	if root := b.config.Proactive.OKR.GoalsRoot; root != "" {
		return resolveStorageDir(root, "")
	}
	return "" // Let OKR tools use their own default
}

func (b *containerBuilder) buildOKRContextProvider() preparation.OKRContextProvider {
	if !b.config.Proactive.Enabled || !b.config.Proactive.OKR.Enabled {
		return nil
	}
	okrCfg := okrtools.DefaultOKRConfig()
	if goalsRoot := b.config.Proactive.OKR.GoalsRoot; goalsRoot != "" {
		okrCfg.GoalsRoot = resolveStorageDir(goalsRoot, okrCfg.GoalsRoot)
	}
	store := okrtools.NewGoalStore(okrCfg)
	b.logger.Info("OKR context provider enabled (goals_root=%s)", okrCfg.GoalsRoot)
	return preparation.NewOKRContextProvider(store)
}
