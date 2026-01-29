package di

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	appconfig "alex/internal/agent/app/config"
	agentcoordinator "alex/internal/agent/app/coordinator"
	agentcost "alex/internal/agent/app/cost"
	"alex/internal/agent/app/hooks"
	agent "alex/internal/agent/ports/agent"
	agentstorage "alex/internal/agent/ports/storage"
	"alex/internal/agent/presets"
	"alex/internal/analytics/journal"
	ctxmgr "alex/internal/context"
	"alex/internal/llm"
	"alex/internal/logging"
	"alex/internal/mcp"
	"alex/internal/memory"
	"alex/internal/parser"
	"alex/internal/session/filestore"
	"alex/internal/session/postgresstore"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/storage"
	toolregistry "alex/internal/toolregistry"
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
	memoryStore  memory.Store
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

	memoryService := b.buildMemoryService(resources.memoryStore)
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

	mcpRegistry := mcp.NewRegistry()
	tracker := newMCPInitializationTracker()

	hookRegistry := b.buildHookRegistry(memoryService)

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
		},
		agentcoordinator.WithHookRegistry(hookRegistry),
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

	memoryDir := resolveStorageDir(b.config.MemoryDir, "~/.alex/memory")
	return sessionResources{
		sessionStore: filestore.New(b.sessionDir),
		stateStore:   sessionstate.NewFileStore(filepath.Join(b.sessionDir, "snapshots")),
		historyStore: sessionstate.NewFileStore(filepath.Join(b.sessionDir, "turns")),
		memoryStore:  memory.NewFileStore(memoryDir),
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
		memoryStore:  memory.NewPostgresStore(pool),
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

func (b *containerBuilder) buildMemoryService(store memory.Store) memory.Service {
	memoryService := memory.NewService(store)
	if err := store.EnsureSchema(context.Background()); err != nil {
		b.logger.Warn("Failed to initialize memory store schema: %v", err)
	}
	return memoryService
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

	// Register memory recall hook (pre-task auto-recall)
	if memoryService != nil {
		recallHook := hooks.NewMemoryRecallHook(memoryService, b.logger, hooks.MemoryRecallConfig{
			MaxRecalls: 5,
		})
		registry.Register(recallHook)

		// Register memory capture hook (post-task auto-capture)
		captureHook := hooks.NewMemoryCaptureHook(memoryService, b.logger)
		registry.Register(captureHook)
	}

	b.logger.Info("Hook registry built with %d hooks", registry.HookCount())
	return registry
}

func (b *containerBuilder) buildToolRegistry(factory *llm.Factory, memoryService memory.Service) (*toolregistry.Registry, error) {
	toolRegistry, err := toolregistry.NewRegistry(toolregistry.Config{
		TavilyAPIKey:               b.config.TavilyAPIKey,
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
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}
	return toolRegistry, nil
}
