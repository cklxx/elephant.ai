package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	agentdomain "alex/internal/agent/domain"
	"alex/internal/analytics"
	"alex/internal/analytics/journal"
	"alex/internal/di"
	"alex/internal/diagnostics"
	"alex/internal/environment"
	"alex/internal/observability"
	serverApp "alex/internal/server/app"
	serverBootstrap "alex/internal/server/bootstrap"
	serverHTTP "alex/internal/server/http"
	"alex/internal/tools"
	"alex/internal/utils"
)

func main() {
	logger := utils.NewComponentLogger("Main")
	logger.Info("Starting Spinner SSE Server...")

	obs, err := observability.New(os.Getenv("ALEX_OBSERVABILITY_CONFIG"))
	if err != nil {
		logger.Warn("Observability disabled: %v", err)
		obs = nil
	}
	if obs != nil {
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := obs.Shutdown(ctx); err != nil {
				logger.Warn("Observability shutdown error: %v", err)
			}
		}()
	}

	// Load configuration
	config, configManager, resolver, err := serverBootstrap.LoadConfig()
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

	hostSummary := environment.CollectLocalSummary(20)
	hostEnv := environment.SummaryMap(hostSummary)
	config.EnvironmentSummary = environment.FormatSummary(hostSummary)

	sandboxEnv := map[string]string{}
	envCapturedAt := time.Now().UTC()

	if runtimeCfg.SandboxBaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		manager := tools.NewSandboxManager(runtimeCfg.SandboxBaseURL)
		summary, err := environment.CollectSandboxSummary(ctx, manager, 20)
		if err != nil {
			logger.Warn("Failed to capture sandbox environment summary: %v", err)
		} else {
			sandboxEnv = environment.SummaryMap(summary)
			config.EnvironmentSummary = environment.FormatSummary(summary)
			envCapturedAt = time.Now().UTC()
		}
		cancel()
	}

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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		summary, err := environment.CollectSandboxSummary(ctx, container.SandboxManager, 20)
		if err != nil {
			logger.Warn("Failed to refresh sandbox environment summary: %v", err)
		} else {
			sandboxEnv = environment.SummaryMap(summary)
			envCapturedAt = time.Now().UTC()
			config.EnvironmentSummary = environment.FormatSummary(summary)
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
		event := agentdomain.NewWorkflowDiagnosticEnvironmentSnapshotEvent(payload.Host, payload.Sandbox, payload.Captured)
		broadcaster.OnEvent(event)
	})
	defer unsubscribeEnv()

	// Broadcast sandbox initialization progress so the UI can surface long-running steps.
	unsubscribeSandboxProgress := diagnostics.SubscribeSandboxProgress(func(payload diagnostics.SandboxProgressPayload) {
		event := agentdomain.NewWorkflowDiagnosticSandboxProgressEvent(
			string(payload.Status),
			payload.Stage,
			payload.Message,
			payload.Step,
			payload.TotalSteps,
			payload.Error,
			payload.Updated,
		)
		broadcaster.OnEvent(event)
	})
	defer unsubscribeSandboxProgress()

	// Set task store on broadcaster for progress tracking
	broadcaster.SetTaskStore(taskStore)
	if archiver := serverApp.NewSandboxAttachmentArchiver(container.SandboxManager, ""); archiver != nil {
		broadcaster.SetAttachmentArchiver(archiver)
	}

	analyticsClient := analytics.NewNoopClient()
	if apiKey := strings.TrimSpace(config.Analytics.PostHogAPIKey); apiKey != "" {
		client, err := analytics.NewPostHogClient(apiKey, strings.TrimSpace(config.Analytics.PostHogHost))
		if err != nil {
			logger.Warn("Analytics disabled: %v", err)
		} else {
			analyticsClient = client
			logger.Info("Analytics client initialized (PostHog)")
		}
	} else {
		logger.Info("Analytics client disabled: POSTHOG_API_KEY not provided")
	}
	defer func() {
		if err := analyticsClient.Close(); err != nil {
			logger.Warn("Failed to close analytics client: %v", err)
		}
	}()

	var journalReader journal.Reader
	if sessionDir := strings.TrimSpace(container.SessionDir()); sessionDir != "" {
		reader, err := journal.NewFileReader(filepath.Join(sessionDir, "journals"))
		if err != nil {
			logger.Warn("Failed to initialize journal reader: %v", err)
		} else {
			journalReader = reader
		}
	} else {
		logger.Warn("Session directory missing; turn replay disabled")
	}

	serverCoordinator := serverApp.NewServerCoordinator(
		container.AgentCoordinator,
		broadcaster,
		container.SessionStore,
		taskStore,
		container.StateStore,
		serverApp.WithAnalyticsClient(analyticsClient),
		serverApp.WithJournalReader(journalReader),
		serverApp.WithObservability(obs),
	)

	// Setup health checker
	healthChecker := serverApp.NewHealthChecker()
	healthChecker.RegisterProbe(serverApp.NewMCPProbe(container, config.EnableMCP))
	healthChecker.RegisterProbe(serverApp.NewLLMFactoryProbe(container))
	healthChecker.RegisterProbe(serverApp.NewSandboxProbe(container.SandboxManager))

	authService, authCleanup, err := serverBootstrap.BuildAuthService(config, logger)
	if err != nil {
		logger.Warn("Authentication disabled: %v", err)
	}
	if authCleanup != nil {
		defer authCleanup()
	}
	var authHandler *serverHTTP.AuthHandler
	if authService != nil {
		secureCookies := strings.EqualFold(runtimeCfg.Environment, "production")
		authHandler = serverHTTP.NewAuthHandler(authService, secureCookies)
		logger.Info("Authentication module initialized")
	}

	// Setup HTTP router
	configHandler := serverHTTP.NewConfigHandler(configManager, resolver)
	router := serverHTTP.NewRouter(
		serverCoordinator,
		broadcaster,
		healthChecker,
		authHandler,
		authService,
		runtimeCfg.Environment,
		config.AllowedOrigins,
		configHandler,
		obs,
	)

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
		ReadTimeout:  5 * time.Minute, // Allow long-running commands (npm install, vite create, etc.)
		WriteTimeout: 0,               // No timeout for SSE
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
func buildContainer(config serverBootstrap.Config) (*di.Container, error) {
	// Build DI container with configurable storage
	diConfig := di.ConfigFromRuntimeConfig(config.Runtime)
	diConfig.EnableMCP = config.EnableMCP
	diConfig.EnvironmentSummary = config.EnvironmentSummary

	return di.BuildContainer(diConfig)
}
