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

	"alex/internal/agent/domain"
	runtimeconfig "alex/internal/config"
	"alex/internal/di"
	"alex/internal/diagnostics"
	"alex/internal/prompts"
	serverApp "alex/internal/server/app"
	serverHTTP "alex/internal/server/http"
	"alex/internal/tools"
	"alex/internal/utils"
)

// Config holds server configuration
type Config struct {
	Runtime            runtimeconfig.RuntimeConfig
	Port               string
	EnableMCP          bool
	EnvironmentSummary string
}

func main() {
	logger := utils.NewComponentLogger("Main")
	logger.Info("Starting ALEX SSE Server...")

	// Load configuration
	config, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log configuration for debugging
	logger.Info("=== Server Configuration ===")
	runtimeCfg := config.Runtime
	logger.Info("LLM Provider: %s", runtimeCfg.LLMProvider)
	logger.Info("LLM Model: %s", runtimeCfg.LLMModel)
	logger.Info("Base URL: %s", runtimeCfg.BaseURL)
	if runtimeCfg.SandboxBaseURL != "" {
		logger.Info("Sandbox Base URL: %s", runtimeCfg.SandboxBaseURL)
	} else {
		logger.Info("Sandbox Base URL: (not set)")
	}
	if keyLen := len(runtimeCfg.APIKey); keyLen > 10 {
		logger.Info("API Key: %s...%s", runtimeCfg.APIKey[:10], runtimeCfg.APIKey[keyLen-10:])
	} else if keyLen > 0 {
		logger.Info("API Key: %s", runtimeCfg.APIKey)
	} else {
		logger.Info("API Key: (not set)")
	}
	logger.Info("Max Tokens: %d", runtimeCfg.MaxTokens)
	logger.Info("Max Iterations: %d", runtimeCfg.MaxIterations)
	logger.Info("Temperature: %.2f (provided=%t)", runtimeCfg.Temperature, runtimeCfg.TemperatureProvided)
	logger.Info("Environment: %s", runtimeCfg.Environment)
	logger.Info("Port: %s", config.Port)
	logger.Info("===========================")

	// Capture environment diagnostics before bootstrapping the container so the
	// DI layer can inject the summary during coordinator construction.
	hostEnv := runtimeconfig.SnapshotProcessEnv()
	sandboxEnv := map[string]string{}
	envCapturedAt := time.Now().UTC()

	if runtimeCfg.SandboxBaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		manager := tools.NewSandboxManager(runtimeCfg.SandboxBaseURL)
		if env, err := manager.Environment(ctx); err != nil {
			logger.Warn("Failed to capture sandbox environment snapshot: %v", err)
		} else if env != nil {
			sandboxEnv = env
			envCapturedAt = time.Now().UTC()
		}
		cancel()
	}

	config.EnvironmentSummary = prompts.FormatEnvironmentSummary(hostEnv, sandboxEnv)

	// Initialize container (without heavy initialization)
	container, err := buildContainer(config)
	if err != nil {
		log.Fatalf("Failed to initialize container: %v", err)
	}
	defer func() {
		if err := container.Shutdown(); err != nil {
			log.Printf("Failed to shutdown container: %v", err)
		}
	}()

	// Start container lifecycle (heavy initialization)
	if err := container.Start(); err != nil {
		logger.Warn("Container start failed: %v (continuing with limited functionality)", err)
	}

	// Refresh sandbox environment snapshot using the shared manager if available so
	// downstream consumers operate on the same cached state.
	if container.SandboxManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if env, err := container.SandboxManager.Environment(ctx); err != nil {
			logger.Warn("Failed to refresh sandbox environment snapshot: %v", err)
		} else if env != nil {
			sandboxEnv = env
			envCapturedAt = time.Now().UTC()
			config.EnvironmentSummary = prompts.FormatEnvironmentSummary(hostEnv, sandboxEnv)
		}
		cancel()
	}

	if summary := config.EnvironmentSummary; summary != "" {
		container.AgentCoordinator.SetEnvironmentSummary(summary)
	}

	// Create server coordinator
	broadcaster := serverApp.NewEventBroadcaster()
	taskStore := serverApp.NewInMemoryTaskStore()

	// Broadcast environment diagnostics to all connected SSE clients.
	unsubscribeEnv := diagnostics.SubscribeEnvironments(func(payload diagnostics.EnvironmentPayload) {
		event := domain.NewEnvironmentSnapshotEvent(payload.Host, payload.Sandbox, payload.Captured)
		broadcaster.OnEvent(event)
	})
	defer unsubscribeEnv()

	// Set task store on broadcaster for progress tracking
	broadcaster.SetTaskStore(taskStore)

	serverCoordinator := serverApp.NewServerCoordinator(
		container.AgentCoordinator,
		broadcaster,
		container.SessionStore,
		taskStore,
	)

	// Setup health checker
	healthChecker := serverApp.NewHealthChecker()
	healthChecker.RegisterProbe(serverApp.NewMCPProbe(container, config.EnableMCP))
	healthChecker.RegisterProbe(serverApp.NewLLMFactoryProbe(container))
	healthChecker.RegisterProbe(serverApp.NewSandboxProbe(container.SandboxManager))

	// Setup HTTP router
	router := serverHTTP.NewRouter(serverCoordinator, broadcaster, healthChecker, runtimeCfg.Environment)

	// Seed diagnostics so the UI can immediately render environment context.
	diagnostics.PublishEnvironments(diagnostics.EnvironmentPayload{
		Host:     hostEnv,
		Sandbox:  sandboxEnv,
		Captured: envCapturedAt,
	})

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

// buildContainer builds the dependency injection container
func buildContainer(config Config) (*di.Container, error) {
	// Build DI container with configurable storage
	diConfig := di.Config{
		LLMProvider:        config.Runtime.LLMProvider,
		LLMModel:           config.Runtime.LLMModel,
		APIKey:             config.Runtime.APIKey,
		BaseURL:            config.Runtime.BaseURL,
		TavilyAPIKey:       config.Runtime.TavilyAPIKey,
		SandboxBaseURL:     config.Runtime.SandboxBaseURL,
		MaxTokens:          config.Runtime.MaxTokens,
		MaxIterations:      config.Runtime.MaxIterations,
		Temperature:        config.Runtime.Temperature,
		TemperatureSet:     config.Runtime.TemperatureProvided,
		TopP:               config.Runtime.TopP,
		StopSequences:      append([]string(nil), config.Runtime.StopSequences...),
		SessionDir:         config.Runtime.SessionDir,
		CostDir:            config.Runtime.CostDir,
		EnableMCP:          config.EnableMCP,
		EnvironmentSummary: config.EnvironmentSummary,
	}

	return di.BuildContainer(diConfig)
}

// loadConfig loads configuration from environment variables
func loadConfig() (Config, error) {
	envLookup := runtimeconfig.AliasEnvLookup(runtimeconfig.DefaultEnvLookup, map[string][]string{
		"LLM_PROVIDER":       {"ALEX_LLM_PROVIDER"},
		"LLM_MODEL":          {"ALEX_LLM_MODEL"},
		"LLM_BASE_URL":       {"ALEX_BASE_URL"},
		"LLM_MAX_TOKENS":     {"ALEX_LLM_MAX_TOKENS"},
		"LLM_MAX_ITERATIONS": {"ALEX_LLM_MAX_ITERATIONS"},
		"TAVILY_API_KEY":     {"ALEX_TAVILY_API_KEY"},
		"ALEX_ENV":           {"ENVIRONMENT", "NODE_ENV"},
		"ALEX_VERBOSE":       {"VERBOSE"},
		"PORT":               {"ALEX_SERVER_PORT"},
		"ENABLE_MCP":         {"ALEX_ENABLE_MCP"},
		"SANDBOX_BASE_URL":   {"ALEX_SANDBOX_BASE_URL"},
	})

	runtimeCfg, _, err := runtimeconfig.Load(
		runtimeconfig.WithEnv(envLookup),
	)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		Runtime:   runtimeCfg,
		Port:      "8080",
		EnableMCP: true, // Default: enabled
	}

	if port, ok := envLookup("PORT"); ok && port != "" {
		cfg.Port = port
	}

	// Parse feature flags
	if enableMCP, ok := envLookup("ENABLE_MCP"); ok {
		cfg.EnableMCP = enableMCP == "true" || enableMCP == "1"
	}

	if cfg.Runtime.APIKey == "" && cfg.Runtime.LLMProvider != "ollama" && cfg.Runtime.LLMProvider != "mock" {
		return Config{}, fmt.Errorf("API key required for provider '%s'", cfg.Runtime.LLMProvider)
	}

	return cfg, nil
}
