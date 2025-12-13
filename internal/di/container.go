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
	"alex/internal/analytics/journal"
	runtimeconfig "alex/internal/config"
	ctxmgr "alex/internal/context"
	"alex/internal/llm"
	"alex/internal/logging"
	"alex/internal/mcp"
	"alex/internal/parser"
	"alex/internal/session/filestore"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/storage"
	toolregistry "alex/internal/toolregistry"
	"alex/internal/tools"
	"golang.org/x/time/rate"
)

// Container holds all application dependencies
type Container struct {
	AgentCoordinator *agentApp.AgentCoordinator
	SessionStore     ports.SessionStore
	StateStore       sessionstate.Store
	HistoryManager   ports.HistoryManager
	CostTracker      ports.CostTracker
	MCPRegistry      *mcp.Registry
	mcpInitTracker   *MCPInitializationTracker
	mcpInitCancel    context.CancelFunc

	SandboxManager *tools.SandboxManager

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
	APIKey                  string
	ArkAPIKey               string
	BaseURL                 string
	TavilyAPIKey            string
	SeedreamTextEndpointID  string
	SeedreamImageEndpointID string
	SeedreamTextModel       string
	SeedreamImageModel      string
	SeedreamVisionModel     string
	SeedreamVideoModel      string
	SandboxBaseURL          string
	MaxTokens               int
	MaxIterations           int
	UserRateLimitRPS        float64
	UserRateLimitBurst      int
	Temperature             float64
	TemperatureSet          bool
	TopP                    float64
	StopSequences           []string
	AgentPreset             string
	ToolPreset              string
	Environment             string
	Verbose                 bool
	DisableTUI              bool
	FollowTranscript        bool
	FollowStream            bool

	EnvironmentSummary string

	// Storage Configuration
	SessionDir string // Directory for session storage (default: ~/.alex-sessions)
	CostDir    string // Directory for cost tracking (default: ~/.alex-costs)

	// Feature Flags
	EnableMCP      bool // Enable MCP tool registration (requires external dependencies)
	DisableSandbox bool // Disable sandbox initialization for faster startup in CLI mode
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

	logger.Info("Container shutdown complete")
	return nil
}

// Cleanup gracefully shuts down all resources (alias for Shutdown for backward compatibility)
func (c *Container) Cleanup() error {
	return c.Shutdown()
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

	logger.Debug("Building container with session_dir=%s, cost_dir=%s", sessionDir, costDir)

	// Infrastructure Layer
	llmFactory := llm.NewFactory()
	if config.UserRateLimitRPS > 0 {
		llmFactory.EnableUserRateLimit(rate.Limit(config.UserRateLimitRPS), config.UserRateLimitBurst)
	}
	executionMode := tools.ExecutionModeLocal
	var sandboxManager *tools.SandboxManager
	sandboxBaseURL := strings.TrimSpace(config.SandboxBaseURL)

	// Skip sandbox initialization if explicitly disabled (e.g., in CLI single-command mode)
	if config.DisableSandbox {
		logger.Debug("Sandbox disabled by configuration (CLI mode optimization)")
		sandboxBaseURL = ""
	} else if sandboxBaseURL != "" {
		executionMode = tools.ExecutionModeSandbox
		sandboxManager = tools.NewSandboxManager(sandboxBaseURL)
		if err := sandboxManager.Initialize(context.Background()); err != nil {
			formatted := tools.FormatSandboxError(err)
			logger.Warn("Sandbox initialization failed for %s: %v (falling back to local execution)", sandboxBaseURL, formatted)
			sandboxManager = nil
			executionMode = tools.ExecutionModeLocal
			sandboxBaseURL = ""
		}
	}

	toolRegistry, err := toolregistry.NewRegistry(toolregistry.Config{
		TavilyAPIKey:            config.TavilyAPIKey,
		SandboxBaseURL:          sandboxBaseURL,
		ArkAPIKey:               config.ArkAPIKey,
		SeedreamTextEndpointID:  config.SeedreamTextEndpointID,
		SeedreamImageEndpointID: config.SeedreamImageEndpointID,
		SeedreamTextModel:       config.SeedreamTextModel,
		SeedreamImageModel:      config.SeedreamImageModel,
		SeedreamVisionModel:     config.SeedreamVisionModel,
		SeedreamVideoModel:      config.SeedreamVideoModel,
		ExecutionMode:           executionMode,
		SandboxManager:          sandboxManager,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create tool registry: %w", err)
	}
	sessionStore := filestore.New(sessionDir)
	stateStore := sessionstate.NewFileStore(filepath.Join(sessionDir, "snapshots"))
	historyStore := sessionstate.NewFileStore(filepath.Join(sessionDir, "turns"))
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

	// Cost tracking storage
	costStore, err := storage.NewFileCostStore(costDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost store: %w", err)
	}
	costTracker := agentApp.NewCostTracker(costStore)

	config.SandboxBaseURL = sandboxBaseURL

	runtimeSnapshot := runtimeconfig.RuntimeConfig{
		LLMProvider:             config.LLMProvider,
		LLMModel:                config.LLMModel,
		APIKey:                  config.APIKey,
		ArkAPIKey:               config.ArkAPIKey,
		BaseURL:                 config.BaseURL,
		TavilyAPIKey:            config.TavilyAPIKey,
		SeedreamTextEndpointID:  config.SeedreamTextEndpointID,
		SeedreamImageEndpointID: config.SeedreamImageEndpointID,
		SeedreamTextModel:       config.SeedreamTextModel,
		SeedreamImageModel:      config.SeedreamImageModel,
		SeedreamVisionModel:     config.SeedreamVisionModel,
		SeedreamVideoModel:      config.SeedreamVideoModel,
		SandboxBaseURL:          sandboxBaseURL,
		Environment:             config.Environment,
		Verbose:                 config.Verbose,
		DisableTUI:              config.DisableTUI,
		FollowTranscript:        config.FollowTranscript,
		FollowStream:            config.FollowStream,
		MaxIterations:           config.MaxIterations,
		MaxTokens:               config.MaxTokens,
		UserRateLimitRPS:        config.UserRateLimitRPS,
		UserRateLimitBurst:      config.UserRateLimitBurst,
		Temperature:             config.Temperature,
		TemperatureProvided:     config.TemperatureSet,
		TopP:                    config.TopP,
		StopSequences:           append([]string(nil), config.StopSequences...),
		SessionDir:              config.SessionDir,
		CostDir:                 config.CostDir,
	}

	// MCP Registry - Create but don't initialize yet
	envLookup := runtimeconfig.RuntimeEnvLookup(runtimeSnapshot, runtimeconfig.DefaultEnvLookup)
	mcpRegistry := mcp.NewRegistry(mcp.WithEnvLookup(envLookup))
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
			APIKey:              config.APIKey,
			BaseURL:             config.BaseURL,
			MaxTokens:           config.MaxTokens,
			MaxIterations:       config.MaxIterations,
			Temperature:         config.Temperature,
			TemperatureProvided: config.TemperatureSet,
			TopP:                config.TopP,
			StopSequences:       append([]string(nil), config.StopSequences...),
			AgentPreset:         config.AgentPreset,
			ToolPreset:          config.ToolPreset,
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
		HistoryManager:   historyMgr,
		CostTracker:      costTracker,
		MCPRegistry:      mcpRegistry,
		mcpInitTracker:   tracker,
		SandboxManager:   sandboxManager,
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

// Deprecated: API key resolution moved to internal/config loader. Retained for backward compatibility.
func GetAPIKey(provider string) string {
	switch provider {
	case "ollama":
		return ""
	default:
		return ""
	}
}

// SessionDir returns the resolved session directory backing file-based stores.
func (c *Container) SessionDir() string {
	if c == nil {
		return ""
	}
	return c.config.SessionDir
}
