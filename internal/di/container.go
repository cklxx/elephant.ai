package di

import (
	"fmt"
	"os"
	"path/filepath"

	agentApp "alex/internal/agent/app"
	"alex/internal/agent/ports"
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
}

// Config holds the dependency injection configuration
type Config struct {
	// LLM Configuration
	LLMProvider   string
	LLMModel      string
	APIKey        string
	BaseURL       string
	MaxTokens     int
	MaxIterations int
	Temperature   float64
	TopP          float64
	StopSequences []string

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
	toolRegistry := tools.NewRegistry()
	sessionStore := filestore.New(sessionDir)
	contextMgr := ctxmgr.NewManager()
	parserImpl := parser.New()

	// Note: MessageQueue removed - not used in current architecture
	// Tasks are processed directly through ExecuteTask, not queued

	// Register Git tools with LLM client
	llmClient, err := llmFactory.GetClient(config.LLMProvider, config.LLMModel, llm.Config{
		APIKey:  config.APIKey,
		BaseURL: config.BaseURL,
	})
	if err == nil {
		toolRegistry.RegisterGitTools(llmClient)
	}

	// Cost tracking storage
	costStore, err := storage.NewFileCostStore(costDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost store: %w", err)
	}
	costTracker := agentApp.NewCostTracker(costStore)

	// MCP Registry - Initialize asynchronously to avoid blocking startup
	mcpRegistry := mcp.NewRegistry()
	go func() {
		if err := mcpRegistry.Initialize(); err != nil {
			logger.Warn("Failed to initialize MCP registry: %v", err)
			// Not fatal - continue without MCP tools
		} else {
			if err := mcpRegistry.RegisterWithToolRegistry(toolRegistry); err != nil {
				logger.Warn("Failed to register MCP tools: %v", err)
			} else {
				logger.Info("MCP tools registered successfully")
			}
		}
	}()

	// Application Layer
	coordinator := agentApp.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		sessionStore,
		contextMgr,
		parserImpl,
		costTracker,
		agentApp.Config{
			LLMProvider:   config.LLMProvider,
			LLMModel:      config.LLMModel,
			APIKey:        config.APIKey,
			BaseURL:       config.BaseURL,
			MaxTokens:     config.MaxTokens,
			MaxIterations: config.MaxIterations,
			Temperature:   config.Temperature,
			TopP:          config.TopP,
			StopSequences: append([]string(nil), config.StopSequences...),
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
	}, nil
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

// GetAPIKey attempts to retrieve API key from multiple sources based on provider
func GetAPIKey(provider string) string {
	// Try provider-specific key first
	switch provider {
	case "openrouter":
		if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
			return key
		}
	case "deepseek":
		if key := os.Getenv("DEEPSEEK_API_KEY"); key != "" {
			return key
		}
	case "ollama":
		// Ollama doesn't require API key
		return ""
	}

	// Fallback to OPENAI_API_KEY for OpenAI and as generic fallback
	return os.Getenv("OPENAI_API_KEY")
}

// GetStorageDir retrieves storage directory from environment or returns default
func GetStorageDir(envVar, defaultPath string) string {
	if dir := os.Getenv(envVar); dir != "" {
		return dir
	}
	return defaultPath
}
