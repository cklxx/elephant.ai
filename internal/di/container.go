package di

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	agentApp "alex/internal/agent/app"
	"alex/internal/agent/ports"
	runtimeconfig "alex/internal/config"
	ctxmgr "alex/internal/context"
	"alex/internal/llm"
	"alex/internal/mcp"
	"alex/internal/parser"
	"alex/internal/session/filestore"
	"alex/internal/storage"
	"alex/internal/tools"
	"alex/internal/utils"
)

// Container holds all application dependencies
type Container struct {
	AgentCoordinator *agentApp.AgentCoordinator
	SessionStore     ports.SessionStore
	CostTracker      ports.CostTracker
	MCPRegistry      *mcp.Registry
	mcpInitTracker   *MCPInitializationTracker
}

// Config holds the dependency injection configuration
type Config struct {
	// LLM Configuration
	LLMProvider      string
	LLMModel         string
	APIKey           string
	BaseURL          string
	TavilyAPIKey     string
	MaxTokens        int
	MaxIterations    int
	Temperature      float64
	TemperatureSet   bool
	TopP             float64
	StopSequences    []string
	Environment      string
	Verbose          bool
	DisableTUI       bool
	FollowTranscript bool
	FollowStream     bool

	// Storage Configuration
	SessionDir string // Directory for session storage (default: ~/.alex-sessions)
	CostDir    string // Directory for cost tracking (default: ~/.alex-costs)
}

// Cleanup gracefully shuts down all resources
func (c *Container) Cleanup() error {
	if c.MCPRegistry != nil {
		return c.MCPRegistry.Shutdown()
	}
	return nil
}

// BuildContainer builds the dependency injection container with the given configuration
func BuildContainer(config Config) (*Container, error) {
	logger := utils.NewComponentLogger("DI")

	// Resolve storage directories with defaults
	sessionDir := resolveStorageDir(config.SessionDir, "~/.alex-sessions")
	costDir := resolveStorageDir(config.CostDir, "~/.alex-costs")

	logger.Debug("Building container with session_dir=%s, cost_dir=%s", sessionDir, costDir)

	// Infrastructure Layer
	llmFactory := llm.NewFactory()
	toolRegistry := tools.NewRegistry(tools.Config{TavilyAPIKey: config.TavilyAPIKey})
	sessionStore := filestore.New(sessionDir)
	contextMgr := ctxmgr.NewManager()
	parserImpl := parser.New()

	// Note: MessageQueue removed - not used in current architecture
	// Tasks are processed directly through ExecuteTask, not queued

	// Cost tracking storage
	costStore, err := storage.NewFileCostStore(costDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost store: %w", err)
	}
	costTracker := agentApp.NewCostTracker(costStore)

	runtimeSnapshot := runtimeconfig.RuntimeConfig{
		LLMProvider:         config.LLMProvider,
		LLMModel:            config.LLMModel,
		APIKey:              config.APIKey,
		BaseURL:             config.BaseURL,
		TavilyAPIKey:        config.TavilyAPIKey,
		Environment:         config.Environment,
		Verbose:             config.Verbose,
		DisableTUI:          config.DisableTUI,
		FollowTranscript:    config.FollowTranscript,
		FollowStream:        config.FollowStream,
		MaxIterations:       config.MaxIterations,
		MaxTokens:           config.MaxTokens,
		Temperature:         config.Temperature,
		TemperatureProvided: config.TemperatureSet,
		TopP:                config.TopP,
		StopSequences:       append([]string(nil), config.StopSequences...),
		SessionDir:          config.SessionDir,
		CostDir:             config.CostDir,
	}

	// MCP Registry - Initialize asynchronously with retry/backoff
	envLookup := runtimeconfig.RuntimeEnvLookup(runtimeSnapshot, runtimeconfig.DefaultEnvLookup)
	mcpRegistry := mcp.NewRegistry(mcp.WithEnvLookup(envLookup))
	tracker := newMCPInitializationTracker()
	startMCPInitialization(mcpRegistry, toolRegistry, logger, tracker)

	// Application Layer
	coordinator := agentApp.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		sessionStore,
		contextMgr,
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
		},
	)

	// Register subagent tool after coordinator is created
	toolRegistry.RegisterSubAgent(coordinator)

	logger.Info("Container built successfully")

	return &Container{
		AgentCoordinator: coordinator,
		SessionStore:     sessionStore,
		CostTracker:      costTracker,
		MCPRegistry:      mcpRegistry,
		mcpInitTracker:   tracker,
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

func startMCPInitialization(registry *mcp.Registry, toolRegistry ports.ToolRegistry, logger *utils.Logger, tracker *MCPInitializationTracker) {
	const (
		initialBackoff = time.Second
		maxBackoff     = 30 * time.Second
	)

	go func() {
		backoff := initialBackoff
		for {
			tracker.recordAttempt()
			snapshot := tracker.Snapshot()
			logger.Info("Initializing MCP registry (attempt %d)", snapshot.Attempts)

			if err := registry.Initialize(); err != nil {
				logger.Warn("MCP initialization failed: %v", err)
				tracker.recordFailure(err)
				backoff = nextBackoff(backoff, maxBackoff)
				time.Sleep(backoff)
				continue
			}

			backoff = initialBackoff

			for {
				if err := registry.RegisterWithToolRegistry(toolRegistry); err != nil {
					logger.Warn("MCP tool registration failed: %v", err)
					tracker.recordFailure(err)
					backoff = nextBackoff(backoff, maxBackoff)
					time.Sleep(backoff)
					continue
				}
				tracker.recordSuccess()
				logger.Info("MCP registry ready")
				return
			}
		}
	}()
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
