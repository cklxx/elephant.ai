package main

import (
	"context"

	"alex/internal/agent/app"
	"alex/internal/agent/ports"
	ctxmgr "alex/internal/context"
	"alex/internal/llm"
	"alex/internal/mcp"
	"alex/internal/messaging"
	"alex/internal/parser"
	"alex/internal/session/filestore"
	"alex/internal/storage"
	"alex/internal/tools"
	"alex/internal/utils"
)

// ListSessions is a helper method on Coordinator
func (c *Container) ListSessions(ctx context.Context) ([]string, error) {
	return []string{}, nil // TODO: Implement via session store
}

// Cleanup gracefully shuts down all resources
func (c *Container) Cleanup() error {
	if c.MCPRegistry != nil {
		return c.MCPRegistry.Shutdown()
	}
	return nil
}

type Container struct {
	Coordinator  *app.AgentCoordinator
	SessionStore ports.SessionStore
	CostTracker  ports.CostTracker
	MCPRegistry  *mcp.Registry
}

func buildContainer() (*Container, error) {
	// Load configuration
	config := loadConfig()

	// Infrastructure Layer
	llmFactory := llm.NewFactory()
	toolRegistry := tools.NewRegistry()
	sessionStore := filestore.New("~/.alex-sessions")
	contextMgr := ctxmgr.NewManager()
	parserImpl := parser.New()
	messageQueue := messaging.NewQueue(100)

	// Register Git tools with LLM client for commit message and PR generation
	llmClient, err := llmFactory.GetClient(config.LLMProvider, config.LLMModel, llm.Config{
		APIKey:  config.APIKey,
		BaseURL: config.BaseURL,
	})
	if err == nil {
		toolRegistry.RegisterGitTools(llmClient)
	}

	// Cost tracking storage
	costStore, err := storage.NewFileCostStore("~/.alex-costs")
	if err != nil {
		return nil, err
	}
	costTracker := app.NewCostTracker(costStore)

	// MCP Registry - Initialize asynchronously to avoid blocking UI startup
	mcpRegistry := mcp.NewRegistry()
	logger := utils.NewComponentLogger("Container")

	// Initialize MCP in background
	go func() {
		if err := mcpRegistry.Initialize(); err != nil {
			logger.Warn("Failed to initialize MCP registry: %v", err)
			// Not fatal - continue without MCP tools
		} else {
			// Register MCP tools with the tool registry
			if err := mcpRegistry.RegisterWithToolRegistry(toolRegistry); err != nil {
				logger.Warn("Failed to register MCP tools: %v", err)
			} else {
				logger.Info("MCP tools registered successfully")
			}
		}
	}()

	// Application Layer
	coordinator := app.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		sessionStore,
		contextMgr,
		parserImpl,
		messageQueue,
		costTracker,
		app.Config{
			LLMProvider:   config.LLMProvider,
			LLMModel:      config.LLMModel,
			APIKey:        config.APIKey,
			BaseURL:       config.BaseURL,
			MaxTokens:     config.MaxTokens,
			MaxIterations: config.MaxIterations,
		},
	)

	// Register subagent tool after coordinator is created
	toolRegistry.RegisterSubAgent(coordinator)

	return &Container{
		Coordinator:  coordinator,
		SessionStore: sessionStore,
		CostTracker:  costTracker,
		MCPRegistry:  mcpRegistry,
	}, nil
}
