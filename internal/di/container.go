package di

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agentApp "alex/internal/agent/app"
	"alex/internal/agent/ports"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/time/rate"
)

// Container holds all application dependencies
type Container struct {
	AgentCoordinator *agentApp.AgentCoordinator
	SessionStore     ports.SessionStore
	StateStore       sessionstate.Store
	HistoryStore     sessionstate.Store
	HistoryManager   ports.HistoryManager
	CostTracker      ports.CostTracker
	MemoryService    memory.Service
	MCPRegistry      *mcp.Registry
	mcpInitTracker   *MCPInitializationTracker
	mcpInitCancel    context.CancelFunc
	SessionDB        *pgxpool.Pool

	// Lazy initialization state
	config       Config
	toolRegistry *toolregistry.Registry
	llmFactory   *llm.Factory
	mcpStarted   bool
	mcpMu        sync.Mutex
}

// Config holds the dependency injection configuration
type Config struct {
	// LLM Configuration
	LLMProvider             string
	LLMModel                string
	LLMSmallProvider        string
	LLMSmallModel           string
	LLMVisionModel          string
	APIKey                  string
	ArkAPIKey               string
	BaseURL                 string
	SandboxBaseURL          string
	ACPExecutorAddr         string
	ACPExecutorCWD          string
	ACPExecutorAutoApprove  bool
	ACPExecutorMaxCLICalls  int
	ACPExecutorMaxDuration  int
	ACPExecutorRequireManifest bool
	TavilyAPIKey            string
	SeedreamTextEndpointID  string
	SeedreamImageEndpointID string
	SeedreamTextModel       string
	SeedreamImageModel      string
	SeedreamVisionModel     string
	SeedreamVideoModel      string
	MaxTokens               int
	MaxIterations           int
	UserRateLimitRPS        float64
	UserRateLimitBurst      int
	Temperature             float64
	TemperatureProvided     bool
	TopP                    float64
	StopSequences           []string
	AgentPreset             string
	ToolPreset              string
	ToolMode                string
	Environment             string
	Verbose                 bool
	DisableTUI              bool
	FollowTranscript        bool
	FollowStream            bool

	EnvironmentSummary string

	// Storage Configuration
	SessionDir         string // Directory for session storage (default: ~/.alex-sessions)
	CostDir            string // Directory for cost tracking (default: ~/.alex-costs)
	SessionDatabaseURL string // Optional database URL for session persistence

	// RequireSessionDatabase enforces Postgres-backed session persistence when true.
	RequireSessionDatabase bool

	// Feature Flags
	EnableMCP bool // Enable MCP tool registration (requires external dependencies)
}

// Start initializes heavy dependencies (MCP) based on feature flags
func (c *Container) Start() error {
	logger := logging.NewComponentLogger("DI")
	logger.Info("Starting container lifecycle...")

	// Initialize MCP if enabled
	if c.config.EnableMCP {
		if err := c.startMCP(); err != nil {
			logger.Warn("Failed to start MCP: %v (continuing without MCP)", err)
		} else {
			logger.Info("MCP initialization started (asynchronous)")
		}
	} else {
		logger.Info("MCP disabled by configuration")
	}

	logger.Info("Container lifecycle started")
	return nil
}

// Shutdown gracefully shuts down all resources
func (c *Container) Shutdown() error {
	logger := logging.NewComponentLogger("DI")
	logger.Info("Shutting down container...")

	c.mcpMu.Lock()
	defer c.mcpMu.Unlock()

	if c.mcpInitCancel != nil {
		c.mcpInitCancel()
		c.mcpInitCancel = nil
	}

	if c.mcpStarted && c.MCPRegistry != nil {
		if err := c.MCPRegistry.Shutdown(); err != nil {
			logger.Error("Failed to shutdown MCP: %v", err)
			return err
		}
		c.mcpStarted = false
		logger.Info("MCP shutdown successfully")
	}

	if c.SessionDB != nil {
		c.SessionDB.Close()
		c.SessionDB = nil
	}

	logger.Info("Container shutdown complete")
	return nil
}

// startMCP starts MCP registry initialization
func (c *Container) startMCP() error {
	c.mcpMu.Lock()
	defer c.mcpMu.Unlock()

	if c.mcpStarted {
		return nil // Already started
	}

	logger := logging.NewComponentLogger("DI")
	initCtx, cancel := context.WithCancel(context.Background())
	c.mcpInitCancel = cancel
	startMCPInitialization(initCtx, c.MCPRegistry, c.toolRegistry, logger, c.mcpInitTracker)
	c.mcpStarted = true

	return nil
}

// BuildContainer builds the dependency injection container with the given configuration
// Heavy initialization (MCP) is deferred until Start() is called
func BuildContainer(config Config) (*Container, error) {
	logger := logging.NewComponentLogger("DI")

	// Resolve storage directories with defaults
	sessionDir := resolveStorageDir(config.SessionDir, "~/.alex-sessions")
	costDir := resolveStorageDir(config.CostDir, "~/.alex-costs")

	if strings.TrimSpace(config.ToolMode) == "" {
		config.ToolMode = string(presets.ToolModeCLI)
	}

	logger.Debug("Building container with session_dir=%s, cost_dir=%s", sessionDir, costDir)

	// Infrastructure Layer
	llmFactory := llm.NewFactory()
	if config.UserRateLimitRPS > 0 {
		llmFactory.EnableUserRateLimit(rate.Limit(config.UserRateLimitRPS), config.UserRateLimitBurst)
	}

	var (
		sessionStore ports.SessionStore
		stateStore   sessionstate.Store
		historyStore sessionstate.Store
		sessionDB    *pgxpool.Pool
		memoryStore  memory.Store
	)

	dbURL := strings.TrimSpace(config.SessionDatabaseURL)
	if config.RequireSessionDatabase && dbURL == "" {
		return nil, fmt.Errorf("session database is required but no database URL is configured")
	}

	if dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			if config.RequireSessionDatabase {
				return nil, fmt.Errorf("failed to create session DB pool: %w", err)
			}
			logger.Warn("Failed to create session DB pool: %v", err)
		} else if err := pool.Ping(ctx); err != nil {
			pool.Close()
			if config.RequireSessionDatabase {
				return nil, fmt.Errorf("failed to ping session DB: %w", err)
			}
			logger.Warn("Failed to ping session DB: %v", err)
		} else {
			dbSessionStore := postgresstore.New(pool)
			if err := dbSessionStore.EnsureSchema(ctx); err != nil {
				pool.Close()
				if config.RequireSessionDatabase {
					return nil, fmt.Errorf("failed to initialize session schema: %w", err)
				}
				logger.Warn("Failed to initialize session schema: %v", err)
			} else {
				dbStateStore := sessionstate.NewPostgresStore(pool, sessionstate.SnapshotKindState)
				dbHistoryStore := sessionstate.NewPostgresStore(pool, sessionstate.SnapshotKindTurn)
				if err := dbStateStore.EnsureSchema(ctx); err != nil {
					pool.Close()
					if config.RequireSessionDatabase {
						return nil, fmt.Errorf("failed to initialize snapshot schema: %w", err)
					}
					logger.Warn("Failed to initialize snapshot schema: %v", err)
				} else if err := dbHistoryStore.EnsureSchema(ctx); err != nil {
					pool.Close()
					if config.RequireSessionDatabase {
						return nil, fmt.Errorf("failed to initialize history schema: %w", err)
					}
					logger.Warn("Failed to initialize history schema: %v", err)
				} else {
					sessionDB = pool
					sessionStore = dbSessionStore
					stateStore = dbStateStore
					historyStore = dbHistoryStore
					memoryStore = memory.NewPostgresStore(pool)
					logger.Info("Session persistence backed by Postgres")
				}
			}
		}
	}
	if sessionStore == nil {
		sessionStore = filestore.New(sessionDir)
	}
	if stateStore == nil {
		stateStore = sessionstate.NewFileStore(filepath.Join(sessionDir, "snapshots"))
	}
	if historyStore == nil {
		historyStore = sessionstate.NewFileStore(filepath.Join(sessionDir, "turns"))
	}
	if memoryStore == nil {
		memoryStore = memory.NewInMemoryStore()
	}

	memoryService := memory.NewService(memoryStore)
	if err := memoryStore.EnsureSchema(context.Background()); err != nil {
		logger.Warn("Failed to initialize memory store schema: %v", err)
	}
	journalDir := filepath.Join(sessionDir, "journals")
	var journalWriter journal.Writer
	if fileWriter, err := journal.NewFileWriter(journalDir); err != nil {
		logger.Warn("Failed to initialize journal writer: %v", err)
		journalWriter = journal.NopWriter()
	} else {
		journalWriter = fileWriter
	}
	contextMgr := ctxmgr.NewManager(
		ctxmgr.WithStateStore(stateStore),
		ctxmgr.WithJournalWriter(journalWriter),
	)
	historyMgr := ctxmgr.NewHistoryManager(historyStore, logger, ports.SystemClock{})
	parserImpl := parser.New()
	llmFactory.EnableToolCallParsing(parserImpl)

	// Cost tracking storage
	costStore, err := storage.NewFileCostStore(costDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost store: %w", err)
	}
	costTracker := agentApp.NewCostTracker(costStore)

	toolRegistry, err := toolregistry.NewRegistry(toolregistry.Config{
		TavilyAPIKey:            config.TavilyAPIKey,
		ArkAPIKey:               config.ArkAPIKey,
		LLMFactory:              llmFactory,
		LLMProvider:             config.LLMProvider,
		LLMModel:                config.LLMModel,
		LLMVisionModel:          config.LLMVisionModel,
		APIKey:                  config.APIKey,
		BaseURL:                 config.BaseURL,
		SandboxBaseURL:          config.SandboxBaseURL,
		ACPExecutorAddr:         config.ACPExecutorAddr,
		ACPExecutorCWD:          config.ACPExecutorCWD,
		ACPExecutorAutoApprove:  config.ACPExecutorAutoApprove,
		ACPExecutorMaxCLICalls:  config.ACPExecutorMaxCLICalls,
		ACPExecutorMaxDuration:  config.ACPExecutorMaxDuration,
		ACPExecutorRequireManifest: config.ACPExecutorRequireManifest,
		SeedreamTextEndpointID:  config.SeedreamTextEndpointID,
		SeedreamImageEndpointID: config.SeedreamImageEndpointID,
		SeedreamTextModel:       config.SeedreamTextModel,
		SeedreamImageModel:      config.SeedreamImageModel,
		SeedreamVisionModel:     config.SeedreamVisionModel,
		SeedreamVideoModel:      config.SeedreamVideoModel,
		MemoryService:           memoryService,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}

	// MCP Registry - Create but don't initialize yet
	mcpRegistry := mcp.NewRegistry()
	tracker := newMCPInitializationTracker()

	coordinator := agentApp.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		sessionStore,
		contextMgr,
		historyMgr,
		parserImpl,
		costTracker,
		agentApp.Config{
			LLMProvider:         config.LLMProvider,
			LLMModel:            config.LLMModel,
			LLMSmallProvider:    config.LLMSmallProvider,
			LLMSmallModel:       config.LLMSmallModel,
			LLMVisionModel:      config.LLMVisionModel,
			APIKey:              config.APIKey,
			BaseURL:             config.BaseURL,
			MaxTokens:           config.MaxTokens,
			MaxIterations:       config.MaxIterations,
			Temperature:         config.Temperature,
			TemperatureProvided: config.TemperatureProvided,
			TopP:                config.TopP,
			StopSequences:       append([]string(nil), config.StopSequences...),
			AgentPreset:         config.AgentPreset,
			ToolPreset:          config.ToolPreset,
			ToolMode:            config.ToolMode,
			EnvironmentSummary:  config.EnvironmentSummary,
		},
	)

	// Register subagent tool after coordinator is created
	toolRegistry.RegisterSubAgent(coordinator)

	logger.Info("Container built successfully (heavy initialization deferred to Start())")

	config.SessionDir = sessionDir
	config.CostDir = costDir

	return &Container{
		AgentCoordinator: coordinator,
		SessionStore:     sessionStore,
		StateStore:       stateStore,
		HistoryStore:     historyStore,
		HistoryManager:   historyMgr,
		CostTracker:      costTracker,
		MemoryService:    memoryService,
		MCPRegistry:      mcpRegistry,
		mcpInitTracker:   tracker,
		SessionDB:        sessionDB,
		config:           config,
		toolRegistry:     toolRegistry,
		llmFactory:       llmFactory,
		mcpStarted:       false,
	}, nil
}

// MCPInitializationStatus captures the asynchronous MCP bootstrap status.
type MCPInitializationStatus struct {
	Ready       bool
	Attempts    int
	LastError   error
	LastAttempt time.Time
	LastSuccess time.Time
}

// MCPInitializationTracker tracks registry initialization state over time.
type MCPInitializationTracker struct {
	mu     sync.RWMutex
	status MCPInitializationStatus
}

func newMCPInitializationTracker() *MCPInitializationTracker {
	return &MCPInitializationTracker{}
}

func (t *MCPInitializationTracker) recordAttempt() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Attempts++
	t.status.LastAttempt = time.Now()
}

func (t *MCPInitializationTracker) recordFailure(err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.LastError = err
	t.status.Ready = false
}

func (t *MCPInitializationTracker) recordSuccess() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.status.Ready = true
	t.status.LastError = nil
	t.status.LastSuccess = time.Now()
}

// Snapshot returns a copy of the current initialization status.
func (t *MCPInitializationTracker) Snapshot() MCPInitializationStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.status
}

// MCPInitializationStatus returns the current asynchronous MCP bootstrap state.
func (c *Container) MCPInitializationStatus() MCPInitializationStatus {
	if c == nil || c.mcpInitTracker == nil {
		return MCPInitializationStatus{}
	}
	return c.mcpInitTracker.Snapshot()
}

func startMCPInitialization(ctx context.Context, registry *mcp.Registry, toolRegistry ports.ToolRegistry, logger logging.Logger, tracker *MCPInitializationTracker) {
	const (
		initialBackoff = time.Second
		maxBackoff     = 30 * time.Second
	)

	go func() {
		logger = logging.OrNop(logger)
		backoff := initialBackoff
		for {
			if ctx.Err() != nil {
				logger.Info("MCP initialization cancelled")
				return
			}
			tracker.recordAttempt()
			snapshot := tracker.Snapshot()
			logger.Info("Initializing MCP registry (attempt %d)", snapshot.Attempts)

			if err := registry.Initialize(); err != nil {
				logger.Warn("MCP initialization failed: %v", err)
				tracker.recordFailure(err)
				backoff = nextBackoff(backoff, maxBackoff)
				if !sleepContext(ctx, backoff) {
					return
				}
				continue
			}

			backoff = initialBackoff

			for {
				if ctx.Err() != nil {
					logger.Info("MCP initialization cancelled")
					return
				}
				if err := registry.RegisterWithToolRegistry(toolRegistry); err != nil {
					logger.Warn("MCP tool registration failed: %v", err)
					tracker.recordFailure(err)
					backoff = nextBackoff(backoff, maxBackoff)
					if !sleepContext(ctx, backoff) {
						return
					}
					continue
				}
				tracker.recordSuccess()
				logger.Info("MCP registry ready")
				return
			}
		}
	}()
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	if current >= max {
		return max
	}
	next := current * 2
	if next > max {
		return max
	}
	if next < time.Second {
		return time.Second
	}
	return next
}

// resolveStorageDir resolves a storage directory path, handling ~ expansion and environment variables
func resolveStorageDir(configured, defaultPath string) string {
	// Use configured path if provided, otherwise use default
	path := configured
	if path == "" {
		path = defaultPath
	}

	// Handle empty path edge case
	if path == "" {
		return path
	}

	// Expand ~ to home directory
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			// Handle ~/ and ~ cases
			if len(path) > 1 && path[1] == '/' {
				// ~/path -> /home/user/path
				path = filepath.Join(home, path[2:])
			} else if len(path) == 1 {
				// ~ -> /home/user
				path = home
			} else {
				// ~path (no slash) -> /home/user/path (treat as relative)
				path = filepath.Join(home, path[1:])
			}
		}
		// If UserHomeDir fails, leave path as-is (will fail later with clear error)
	}

	// Expand environment variables like $HOME
	path = os.ExpandEnv(path)

	return path
}

// SessionDir returns the resolved session directory backing file-based stores.
func (c *Container) SessionDir() string {
	if c == nil {
		return ""
	}
	return c.config.SessionDir
}
