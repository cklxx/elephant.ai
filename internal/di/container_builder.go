package di

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	appconfig "alex/internal/agent/app/config"
	appcontext "alex/internal/agent/app/context"
	agentcoordinator "alex/internal/agent/app/coordinator"
	agentcost "alex/internal/agent/app/cost"
	"alex/internal/agent/app/hooks"
	"alex/internal/agent/app/preparation"
	react "alex/internal/agent/domain/react"
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
	"alex/internal/session/filestore"
	"alex/internal/session/postgresstore"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/storage"
	toolregistry "alex/internal/toolregistry"
	toolspolicy "alex/internal/tools"
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

	memoryEngine, err := b.buildMemoryEngine()
	if err != nil {
		return nil, err
	}
	journalWriter := b.buildJournalWriter()
	contextOptions := []ctxmgr.Option{
		ctxmgr.WithStateStore(resources.stateStore),
		ctxmgr.WithJournalWriter(journalWriter),
	}
	if memoryEngine != nil {
		contextOptions = append(contextOptions, ctxmgr.WithMemoryEngine(memoryEngine))
		contextOptions = append(contextOptions, ctxmgr.WithMemoryGate(memoryGateFunc(b.config.Proactive.Memory.Enabled)))
	}
	contextMgr := ctxmgr.NewManager(contextOptions...)
	historyMgr := ctxmgr.NewHistoryManager(resources.historyStore, b.logger, agent.SystemClock{})
	parserImpl := parser.New()
	llmFactory.EnableToolCallParsing(parserImpl)

	costTracker, err := b.buildCostTracker()
	if err != nil {
		return nil, err
	}

	toolRegistry, err := b.buildToolRegistry(llmFactory, memoryEngine)
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

	hookRegistry := b.buildHookRegistry()
	okrContextProvider := b.buildOKRContextProvider()
	checkpointStore := react.NewFileCheckpointStore(filepath.Join(b.sessionDir, "checkpoints"))
	credentialRefresher := buildCredentialRefresher()

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
			ToolPolicy:          b.config.ToolPolicy,
		},
		agentcoordinator.WithHookRegistry(hookRegistry),
		agentcoordinator.WithExternalExecutor(externalExecutor),
		agentcoordinator.WithOKRContextProvider(okrContextProvider),
		agentcoordinator.WithCheckpointStore(checkpointStore),
		agentcoordinator.WithCredentialRefresher(credentialRefresher),
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
		MemoryEngine:     memoryEngine,
		CheckpointStore:  checkpointStore,
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

func (b *containerBuilder) buildMemoryEngine() (memory.Engine, error) {
	root := resolveStorageDir(b.config.MemoryDir, "~/.alex/memory")
	engine := memory.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		b.logger.Warn("Failed to initialize memory root: %v", err)
	}
	return engine, nil
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

func (b *containerBuilder) buildHookRegistry() *hooks.Registry {
	registry := hooks.NewRegistry(b.logger)
	if !b.config.Proactive.Enabled {
		b.logger.Info("Non-memory proactive hooks disabled by config")
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

func (b *containerBuilder) buildToolRegistry(factory *llm.Factory, memoryEngine memory.Engine) (*toolregistry.Registry, error) {
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
		MemoryEngine:               memoryEngine,
		OKRGoalsRoot:               b.resolveOKRGoalsRoot(),
		HTTPLimits:                 b.config.HTTPLimits,
		ToolPolicy:                 toolspolicy.NewToolPolicy(b.config.ToolPolicy),
		Toolset:                    b.config.Toolset,
		BrowserConfig:              b.config.BrowserConfig,
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

func memoryGateFunc(enabled bool) func(context.Context) bool {
	return func(ctx context.Context) bool {
		if !enabled {
			return false
		}
		policy := appcontext.ResolveMemoryPolicy(ctx)
		return policy.Enabled
	}
}

// buildCredentialRefresher creates a function that re-resolves CLI credentials
// at task execution time. This ensures long-running servers (e.g. Lark) use
// fresh tokens even after the startup token expires (Codex, Antigravity).
func buildCredentialRefresher() preparation.CredentialRefresher {
	return func(provider string) (string, string, bool) {
		provider = strings.ToLower(strings.TrimSpace(provider))
		creds := runtimeconfig.LoadCLICredentials()
		switch provider {
		case "codex", "openai-responses", "responses":
			if creds.Codex.APIKey != "" {
				return creds.Codex.APIKey, creds.Codex.BaseURL, true
			}
		case "antigravity":
			if creds.Antigravity.APIKey != "" {
				return creds.Antigravity.APIKey, creds.Antigravity.BaseURL, true
			}
		case "anthropic", "claude":
			if creds.Claude.APIKey != "" {
				return creds.Claude.APIKey, creds.Claude.BaseURL, true
			}
		}
		return "", "", false
	}
}
