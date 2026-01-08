package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agentdomain "alex/internal/agent/domain"
	"alex/internal/attachments"
	"alex/internal/diagnostics"
	"alex/internal/logging"
	"alex/internal/materials"
	serverApp "alex/internal/server/app"
	serverHTTP "alex/internal/server/http"
)

// RunServer starts the HTTP API server and blocks until a shutdown signal is received.
func RunServer(observabilityConfigPath string) error {
	logger := logging.NewComponentLogger("Main")
	logger.Info("Starting elephant.ai SSE Server...")

	obs, cleanupObs := InitObservability(observabilityConfigPath, logger)
	if cleanupObs != nil {
		defer cleanupObs()
	}

	config, configManager, resolver, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	LogServerConfiguration(logger, config)

	hostEnv, hostSummary := CaptureHostEnvironment(20)
	config.EnvironmentSummary = hostSummary
	envCapturedAt := time.Now().UTC()

	container, err := BuildContainer(config)
	if err != nil {
		return fmt.Errorf("build container: %w", err)
	}
	defer func() {
		if err := container.Shutdown(); err != nil {
			logger.Warn("Failed to shutdown container: %v", err)
		}
	}()

	if err := container.Start(); err != nil {
		logger.Warn("Container start failed: %v (continuing with limited functionality)", err)
	}

	if summary := config.EnvironmentSummary; summary != "" {
		container.AgentCoordinator.SetEnvironmentSummary(summary)
	}

	config.Attachment = attachments.NormalizeConfig(config.Attachment)
	if store, err := attachments.NewStore(config.Attachment); err != nil {
		logger.Warn("Attachment migrator disabled: %v", err)
	} else {
		migrator := materials.NewAttachmentStoreMigrator(store, nil, config.Attachment.CloudflarePublicBaseURL, logger)
		container.AgentCoordinator.SetAttachmentMigrator(migrator)
	}

	var historyStore serverApp.EventHistoryStore
	var asyncHistoryStore *serverApp.AsyncEventHistoryStore
	if container.SessionDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		pgHistory := serverApp.NewPostgresEventHistoryStore(container.SessionDB)
		if err := pgHistory.EnsureSchema(ctx); err != nil {
			logger.Warn("Failed to initialize event history schema: %v", err)
		} else {
			historyStore = pgHistory
			asyncHistoryStore = serverApp.NewAsyncEventHistoryStore(pgHistory)
			defer func() {
				_ = asyncHistoryStore.Close()
			}()
		}
	}

	broadcasterHistoryStore := historyStore
	if asyncHistoryStore != nil {
		broadcasterHistoryStore = asyncHistoryStore
	}
	broadcaster := serverApp.NewEventBroadcaster(serverApp.WithEventHistoryStore(broadcasterHistoryStore))
	taskStore := serverApp.NewInMemoryTaskStore()

	cleanupDiagnostics := subscribeDiagnostics(broadcaster)
	defer cleanupDiagnostics()

	broadcaster.SetTaskStore(taskStore)

	analyticsClient, analyticsCleanup := BuildAnalyticsClient(config.Analytics, logger)
	if analyticsCleanup != nil {
		defer analyticsCleanup()
	}

	journalReader := BuildJournalReader(container.SessionDir(), logger)

	serverCoordinator := serverApp.NewServerCoordinator(
		container.AgentCoordinator,
		broadcaster,
		container.SessionStore,
		taskStore,
		container.StateStore,
		serverApp.WithAnalyticsClient(analyticsClient),
		serverApp.WithJournalReader(journalReader),
		serverApp.WithObservability(obs),
		serverApp.WithHistoryStore(container.HistoryStore),
	)

	if historyStore != nil {
		if err := MigrateSessionsToDatabase(
			context.Background(),
			container.SessionDir(),
			container.SessionStore,
			container.StateStore,
			container.HistoryStore,
			historyStore,
			logger,
		); err != nil {
			logger.Warn("Session migration failed: %v", err)
		}
	}

	healthChecker := serverApp.NewHealthChecker()
	healthChecker.RegisterProbe(serverApp.NewMCPProbe(container, config.EnableMCP))
	healthChecker.RegisterProbe(serverApp.NewLLMFactoryProbe(container))

	authService, authCleanup, err := BuildAuthService(config, logger)
	if err != nil {
		logger.Warn("Authentication disabled: %v", err)
	}
	if authCleanup != nil {
		defer authCleanup()
	}

	var authHandler *serverHTTP.AuthHandler
	if authService != nil {
		secureCookies := strings.EqualFold(config.Runtime.Environment, "production")
		authHandler = serverHTTP.NewAuthHandler(authService, secureCookies)
		logger.Info("Authentication module initialized")
	}

	configHandler := serverHTTP.NewConfigHandler(configManager, resolver)
	evaluationService, err := serverApp.NewEvaluationService("./evaluation_results")
	if err != nil {
		logger.Warn("Evaluation service disabled: %v", err)
	}
	router := serverHTTP.NewRouter(
		serverCoordinator,
		broadcaster,
		healthChecker,
		authHandler,
		authService,
		config.Runtime.Environment,
		config.AllowedOrigins,
		config.Runtime.SandboxBaseURL,
		configHandler,
		evaluationService,
		obs,
		config.Attachment,
	)

	diagnostics.PublishEnvironments(diagnostics.EnvironmentPayload{
		Host:     hostEnv,
		Captured: envCapturedAt,
	})

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	return serveUntilSignal(server, logger)
}

func subscribeDiagnostics(broadcaster *serverApp.EventBroadcaster) func() {
	unsubscribeEnv := diagnostics.SubscribeEnvironments(func(payload diagnostics.EnvironmentPayload) {
		event := agentdomain.NewWorkflowDiagnosticEnvironmentSnapshotEvent(payload.Host, payload.Captured)
		broadcaster.OnEvent(event)
	})

	return func() {
		if unsubscribeEnv != nil {
			unsubscribeEnv()
		}
	}
}

func serveUntilSignal(server *http.Server, logger logging.Logger) error {
	logger = logging.OrNop(logger)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("Server listening on %s", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return fmt.Errorf("server error: %w", err)
	case <-quit:
		logger.Info("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		shutdownErr := server.Shutdown(ctx)

		serveErr := <-errCh
		if serveErr == http.ErrServerClosed {
			serveErr = nil
		}

		if shutdownErr != nil {
			return fmt.Errorf("shutdown: %w", shutdownErr)
		}
		if serveErr != nil {
			return fmt.Errorf("server error: %w", serveErr)
		}

		logger.Info("Server stopped")
		return nil
	}
}
