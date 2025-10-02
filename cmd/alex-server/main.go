package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	agentApp "alex/internal/agent/app"
	"alex/internal/agent/ports"
	ctxmgr "alex/internal/context"
	"alex/internal/llm"
	"alex/internal/mcp"
	"alex/internal/messaging"
	"alex/internal/parser"
	serverApp "alex/internal/server/app"
	serverHTTP "alex/internal/server/http"
	"alex/internal/session/filestore"
	"alex/internal/storage"
	"alex/internal/tools"
	"alex/internal/utils"
)

// Config holds server configuration
type Config struct {
	LLMProvider   string
	LLMModel      string
	APIKey        string
	BaseURL       string
	MaxTokens     int
	MaxIterations int
	Port          string
}

func main() {
	logger := utils.NewComponentLogger("Main")
	logger.Info("Starting ALEX SSE Server...")

	// Load configuration
	config := loadConfig()

	// Initialize container
	container, err := buildContainer(config)
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}
	defer func() {
		if err := container.Cleanup(); err != nil {
			log.Printf("Failed to cleanup container: %v", err)
		}
	}()

	// Create server coordinator
	broadcaster := serverApp.NewEventBroadcaster()
	serverCoordinator := serverApp.NewServerCoordinator(
		container.AgentCoordinator,
		broadcaster,
		container.SessionStore,
	)

	// Setup HTTP router
	router := serverHTTP.NewRouter(serverCoordinator, broadcaster)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 0, // No timeout for SSE
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("Server listening on :%s", config.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server stopped")
}

// Container holds all dependencies
type Container struct {
	AgentCoordinator *agentApp.AgentCoordinator
	SessionStore     ports.SessionStore
	CostTracker      ports.CostTracker
	MCPRegistry      *mcp.Registry
}

// Cleanup gracefully shuts down all resources
func (c *Container) Cleanup() error {
	if c.MCPRegistry != nil {
		return c.MCPRegistry.Shutdown()
	}
	return nil
}

// buildContainer builds the dependency injection container
func buildContainer(config Config) (*Container, error) {
	logger := utils.NewComponentLogger("Container")

	// Infrastructure Layer
	llmFactory := llm.NewFactory()
	toolRegistry := tools.NewRegistry()
	sessionStore := filestore.New("~/.alex-sessions")
	contextMgr := ctxmgr.NewManager()
	parserImpl := parser.New()
	messageQueue := messaging.NewQueue(100)

	// Register Git tools with LLM client
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
	costTracker := agentApp.NewCostTracker(costStore)

	// MCP Registry - Initialize asynchronously
	mcpRegistry := mcp.NewRegistry()
	go func() {
		if err := mcpRegistry.Initialize(); err != nil {
			logger.Warn("Failed to initialize MCP registry: %v", err)
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
		messageQueue,
		costTracker,
		agentApp.Config{
			LLMProvider:   config.LLMProvider,
			LLMModel:      config.LLMModel,
			APIKey:        config.APIKey,
			BaseURL:       config.BaseURL,
			MaxTokens:     config.MaxTokens,
			MaxIterations: config.MaxIterations,
		},
	)

	// Register subagent tool
	toolRegistry.RegisterSubAgent(coordinator)

	return &Container{
		AgentCoordinator: coordinator,
		SessionStore:     sessionStore,
		CostTracker:      costTracker,
		MCPRegistry:      mcpRegistry,
	}, nil
}

// loadConfig loads configuration from environment variables
func loadConfig() Config {
	config := Config{
		LLMProvider:   getEnv("ALEX_LLM_PROVIDER", "openai"),
		LLMModel:      getEnv("ALEX_LLM_MODEL", "gpt-4o"),
		APIKey:        getEnv("OPENAI_API_KEY", ""),
		BaseURL:       getEnv("ALEX_BASE_URL", ""),
		MaxTokens:     128000,
		MaxIterations: 20,
		Port:          getEnv("PORT", "8080"),
	}

	// Validate required configuration
	if config.APIKey == "" {
		fmt.Fprintf(os.Stderr, "Error: OPENAI_API_KEY environment variable is required\n")
		os.Exit(1)
	}

	return config
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
