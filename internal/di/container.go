package di

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	agentcoordinator "alex/internal/agent/app/coordinator"
	agentstorage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/async"
	runtimeconfig "alex/internal/config"
	"alex/internal/llm"
	"alex/internal/logging"
	"alex/internal/mcp"
	"alex/internal/memory"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/toolregistry"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Container holds all application dependencies
type Container struct {
	AgentCoordinator *agentcoordinator.AgentCoordinator
	SessionStore     agentstorage.SessionStore
	StateStore       sessionstate.Store
	HistoryStore     sessionstate.Store
	HistoryManager   agentstorage.HistoryManager
	CostTracker      agentstorage.CostTracker
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

const (
	defaultSessionPoolMaxConns          = 25
	defaultSessionPoolMinConns          = 5
	defaultSessionPoolMaxConnLifetime   = 1 * time.Hour
	defaultSessionPoolMaxConnIdleTime   = 30 * time.Minute
	defaultSessionPoolHealthCheckPeriod = 1 * time.Minute
	defaultSessionPoolConnectTimeout    = 5 * time.Second
	defaultSessionStatementCache        = 256
)

// Config holds the dependency injection configuration
type Config struct {
	// LLM Configuration
	LLMProvider                string
	LLMModel                   string
	LLMSmallProvider           string
	LLMSmallModel              string
	LLMVisionModel             string
	APIKey                     string
	ArkAPIKey                  string
	BaseURL                    string
	SandboxBaseURL             string
	ACPExecutorAddr            string
	ACPExecutorCWD             string
	ACPExecutorMode            string
	ACPExecutorAutoApprove     bool
	ACPExecutorMaxCLICalls     int
	ACPExecutorMaxDuration     int
	ACPExecutorRequireManifest bool
	TavilyAPIKey               string
	SeedreamTextEndpointID     string
	SeedreamImageEndpointID    string
	SeedreamTextModel          string
	SeedreamImageModel         string
	SeedreamVisionModel        string
	SeedreamVideoModel         string
	MaxTokens                  int
	MaxIterations              int
	ToolMaxConcurrent          int
	LLMCacheSize               int
	LLMCacheTTL                time.Duration
	UserRateLimitRPS           float64
	UserRateLimitBurst         int
	Temperature                float64
	TemperatureProvided        bool
	TopP                       float64
	StopSequences              []string
	AgentPreset                string
	ToolPreset                 string
	ToolMode                   string
	Environment                string
	Verbose                    bool
	DisableTUI                 bool
	FollowTranscript           bool
	FollowStream               bool

	EnvironmentSummary string

	// Storage Configuration
	SessionDir                   string // Directory for session storage (default: ~/.alex/sessions)
	CostDir                      string // Directory for cost tracking (default: ~/.alex/costs)
	MemoryDir                    string // Directory for file-based memory storage (default: ~/.alex/memory)
	SessionStaleAfter            time.Duration
	SessionDatabaseURL           string // Optional database URL for session persistence
	SessionPoolMaxConns          int
	SessionPoolMinConns          int
	SessionPoolMaxConnLifetime   time.Duration
	SessionPoolMaxConnIdleTime   time.Duration
	SessionPoolHealthCheckPeriod time.Duration
	SessionPoolConnectTimeout    time.Duration
	SessionCacheSize             *int

	// RequireSessionDatabase enforces Postgres-backed session persistence when true.
	RequireSessionDatabase bool

	// Feature Flags
	EnableMCP bool // Enable MCP tool registration (requires external dependencies)

	Proactive      runtimeconfig.ProactiveConfig
	ExternalAgents runtimeconfig.ExternalAgentsConfig
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

func startMCPInitialization(ctx context.Context, registry *mcp.Registry, toolRegistry tools.ToolRegistry, logger logging.Logger, tracker *MCPInitializationTracker) {
	const (
		initialBackoff = time.Second
		maxBackoff     = 30 * time.Second
	)

	async.Go(logger, "di.mcpInitialization", func() {
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
	})
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

// HasLLMFactory reports whether the container holds an initialised LLM factory.
func (c *Container) HasLLMFactory() bool {
	return c != nil && c.llmFactory != nil
}

// SessionDir returns the resolved session directory backing file-based stores.
func (c *Container) SessionDir() string {
	if c == nil {
		return ""
	}
	return c.config.SessionDir
}
