package di

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	agentcoordinator "alex/internal/app/agent/coordinator"
	"alex/internal/app/lifecycle"
	"alex/internal/app/toolregistry"
	agentstorage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	react "alex/internal/domain/agent/react"
	taskdomain "alex/internal/domain/task"
	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/infra/llm"
	"alex/internal/infra/mcp"
	"alex/internal/infra/memory"
	sessionstate "alex/internal/infra/session/state_store"
	toolspolicy "alex/internal/infra/tools"
	"alex/internal/shared/async"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LarkGateway is the minimal gateway surface needed outside delivery/channel.
type LarkGateway interface {
	NoticeLoader() func() (string, bool, error)
	SendNotification(ctx context.Context, chatID, text string) error
}

// Container holds all application dependencies
type Container struct {
	AgentCoordinator *agentcoordinator.AgentCoordinator
	SessionStore     agentstorage.SessionStore
	StateStore       sessionstate.Store
	HistoryStore     sessionstate.Store
	HistoryManager   agentstorage.HistoryManager
	CostTracker      agentstorage.CostTracker
	MemoryEngine     memory.Engine
	CheckpointStore  react.CheckpointStore
	MCPRegistry      *mcp.Registry
	mcpInitTracker   *MCPInitializationTracker
	mcpInitCancel    context.CancelFunc
	SessionDB        *pgxpool.Pool
	TaskStore        taskdomain.Store // Unified durable task store (nil if SessionDB is nil)
	LarkGateway      LarkGateway
	LarkOAuth        *larkoauth.Service

	// Drainables holds subsystems that support graceful drain.
	Drainables []lifecycle.Drainable

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
	MoltbookAPIKey             string
	MoltbookBaseURL            string
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
	Toolset                    toolregistry.Toolset
	Profile                    string
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
	ToolPolicy                   toolspolicy.ToolPolicyConfig
	BrowserConfig                toolregistry.BrowserConfig

	// RequireSessionDatabase enforces Postgres-backed session persistence when true.
	RequireSessionDatabase bool

	// Feature Flags
	EnableMCP bool // Enable MCP tool registration (requires external dependencies)

	HTTPLimits     runtimeconfig.HTTPLimitsConfig
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

// drainTimeout is the per-subsystem timeout for graceful drain.
const drainTimeout = 5 * time.Second

// Drain gracefully drains all registered Drainable subsystems (with a
// per-subsystem timeout), then performs the hard shutdown of remaining
// resources. If the parent context expires, drain still attempts Shutdown.
func (c *Container) Drain(ctx context.Context) error {
	logger := logging.NewComponentLogger("DI")

	if len(c.Drainables) > 0 {
		logger.Info("Draining %d subsystem(s)...", len(c.Drainables))
		errs := lifecycle.DrainAll(ctx, drainTimeout, c.Drainables...)
		for _, err := range errs {
			logger.Warn("Drain error: %v", err)
		}
	}

	return c.Shutdown()
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

	if c.AgentCoordinator != nil {
		if err := c.AgentCoordinator.Close(); err != nil {
			logger.Error("Failed to close agent coordinator: %v", err)
			return err
		}
	}

	if c.toolRegistry != nil {
		c.toolRegistry.Close()
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

// AlternateCoordinator holds a secondary AgentCoordinator and ToolRegistry
// that share the parent Container's heavy resources (LLM Factory, Session
// Store, Memory Engine, Cost Tracker, DB Pool). Only the ToolRegistry and
// AgentCoordinator are independently owned and shut down separately.
type AlternateCoordinator struct {
	AgentCoordinator *agentcoordinator.AgentCoordinator
	toolRegistry     *toolregistry.Registry
}

// Shutdown releases only the resources owned by this alternate coordinator
// (tool registry and coordinator). It does NOT close shared resources.
func (a *AlternateCoordinator) Shutdown() error {
	if a == nil {
		return nil
	}
	if a.AgentCoordinator != nil {
		if err := a.AgentCoordinator.Close(); err != nil {
			return err
		}
	}
	if a.toolRegistry != nil {
		a.toolRegistry.Close()
	}
	return nil
}

// BuildAlternateCoordinator creates a lightweight secondary AgentCoordinator
// that shares the container's LLM Factory, Session Store, Memory Engine,
// Cost Tracker, and other heavy resources, but uses a fresh ToolRegistry
// configured with the given toolMode, toolset, and browser config.
//
// This avoids the cost of duplicating an entire DI Container when only
// the tool configuration differs (e.g. Lark gateway needing CLI-mode tools).
func (c *Container) BuildAlternateCoordinator(
	toolMode string,
	toolset toolregistry.Toolset,
	browserCfg toolregistry.BrowserConfig,
) (*AlternateCoordinator, error) {
	if c == nil {
		return nil, fmt.Errorf("cannot build alternate coordinator from nil container")
	}

	// Override only the tool-related fields in a copy of the config.
	altConfig := c.config
	altConfig.ToolMode = toolMode
	altConfig.Toolset = toolset
	altConfig.BrowserConfig = browserCfg

	builder := newContainerBuilder(altConfig)
	return builder.buildAlternateFrom(c)
}
