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
	"alex/internal/analytics"
	"alex/internal/async"
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
	degraded := NewDegradedComponents()

	// ── Phase 1: Required infrastructure (failure aborts startup) ──

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

	// ── Phase 2: Optional services (failure records degraded, continues) ──

	config.Attachment = attachments.NormalizeConfig(config.Attachment)
	var attachmentStore *attachments.Store
	var historyStore serverApp.EventHistoryStore
	var asyncHistoryStore *serverApp.AsyncEventHistoryStore
	var analyticsClient analytics.Client
	var analyticsCleanup func()

	optionalStages := []BootstrapStage{
		{
			Name: "attachments", Required: false,
			Init: func() error {
				store, err := attachments.NewStore(config.Attachment)
				if err != nil {
					return err
				}
				attachmentStore = store
				migrator := materials.NewAttachmentStoreMigrator(store, nil, config.Attachment.CloudflarePublicBaseURL, logger)
				container.AgentCoordinator.SetAttachmentMigrator(migrator)
				container.AgentCoordinator.SetAttachmentPersister(
					attachments.NewStorePersister(store),
				)
				return nil
			},
		},
		{
			Name: "event-history", Required: false,
			Init: func() error {
				if container.SessionDB == nil {
					return nil // not an error, just not configured
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				historyOpts := []serverApp.PostgresEventHistoryStoreOption{}
				if attachmentStore != nil {
					historyOpts = append(historyOpts, serverApp.WithHistoryAttachmentStore(attachmentStore))
				}
				if config.EventHistoryRetention > 0 {
					historyOpts = append(historyOpts, serverApp.WithHistoryRetention(config.EventHistoryRetention))
				}
				pgHistory := serverApp.NewPostgresEventHistoryStore(container.SessionDB, historyOpts...)
				if err := pgHistory.EnsureSchema(ctx); err != nil {
					return err
				}
				historyStore = pgHistory
				asyncHistoryStore = serverApp.NewAsyncEventHistoryStore(pgHistory)
				return nil
			},
		},
		{
			Name: "analytics", Required: false,
			Init: func() error {
				analyticsClient, analyticsCleanup = BuildAnalyticsClient(config.Analytics, logger)
				return nil
			},
		},
	}

	if err := RunStages(optionalStages, degraded, logger); err != nil {
		return fmt.Errorf("optional stages: %w", err)
	}

	if asyncHistoryStore != nil {
		defer func() { _ = asyncHistoryStore.Close() }()
	}
	if analyticsCleanup != nil {
		defer analyticsCleanup()
	}

	broadcasterHistoryStore := historyStore
	if asyncHistoryStore != nil {
		broadcasterHistoryStore = asyncHistoryStore
	}
	broadcasterOpts := []serverApp.EventBroadcasterOption{
		serverApp.WithEventHistoryStore(broadcasterHistoryStore),
	}
	if config.EventHistoryMaxEvents > 0 {
		broadcasterOpts = append(broadcasterOpts, serverApp.WithMaxHistory(config.EventHistoryMaxEvents))
	}
	if config.EventHistoryMaxSessions > 0 {
		broadcasterOpts = append(broadcasterOpts, serverApp.WithMaxSessions(config.EventHistoryMaxSessions))
	}
	if config.EventHistorySessionTTL > 0 {
		broadcasterOpts = append(broadcasterOpts, serverApp.WithSessionTTL(config.EventHistorySessionTTL))
	}
	broadcaster := serverApp.NewEventBroadcaster(broadcasterOpts...)
	taskStore := serverApp.NewInMemoryTaskStore()
	defer taskStore.Close()
	progressTracker := serverApp.NewTaskProgressTracker(taskStore)

	cleanupDiagnostics := subscribeDiagnostics(broadcaster)
	defer cleanupDiagnostics()

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
		serverApp.WithProgressTracker(progressTracker),
	)

	// ── Phase 3: Subsystems (gateways, scheduler — managed lifecycle) ──

	subsystems := NewSubsystemManager(logger)
	defer subsystems.StopAll()

	gatewayStages := []BootstrapStage{
		{
			Name: "lark-gateway", Required: false,
			Init: func() error {
				return subsystems.Start(context.Background(), &gatewaySubsystem{
					name: "lark",
					startFn: func(ctx context.Context) (func(), error) {
						return startLarkGateway(ctx, config, container, logger, broadcaster)
					},
				})
			},
		},
		{
			Name: "scheduler", Required: false,
			Init: func() error {
				if !config.Runtime.Proactive.Scheduler.Enabled {
					return nil
				}
				return subsystems.Start(context.Background(), &gatewaySubsystem{
					name: "scheduler",
					startFn: func(ctx context.Context) (func(), error) {
						sched := startScheduler(ctx, config, container, logger)
						if sched == nil {
							return nil, fmt.Errorf("scheduler init returned nil")
						}
						return sched.Stop, nil
					},
				})
			},
		},
	}

	if err := RunStages(gatewayStages, degraded, logger); err != nil {
		return fmt.Errorf("gateway stages: %w", err)
	}

	// ── Phase 4: Session migration (best-effort) ──

	if historyStore != nil {
		migrationCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := MigrateSessionsToDatabase(
			migrationCtx,
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

	// ── Phase 5: HTTP layer ──

	healthChecker := serverApp.NewHealthChecker()
	healthChecker.RegisterProbe(serverApp.NewMCPProbe(container, config.EnableMCP))
	healthChecker.RegisterProbe(serverApp.NewLLMFactoryProbe(container))
	healthChecker.RegisterProbe(serverApp.NewDegradedProbe(degraded))

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
		serverHTTP.RouterDeps{
			Coordinator:             serverCoordinator,
			Broadcaster:             broadcaster,
			RunTracker:              progressTracker,
			HealthChecker:           healthChecker,
			AuthHandler:             authHandler,
			AuthService:             authService,
			ConfigHandler:           configHandler,
			Evaluation:              evaluationService,
			Obs:                     obs,
			MemoryService:           container.MemoryService,
			AttachmentCfg:           config.Attachment,
			SandboxBaseURL:          config.Runtime.SandboxBaseURL,
			SandboxMaxResponseBytes: config.Runtime.HTTPLimits.SandboxMaxResponseBytes,
		},
		serverHTTP.RouterConfig{
			Environment:      config.Runtime.Environment,
			AllowedOrigins:   config.AllowedOrigins,
			MaxTaskBodyBytes: config.MaxTaskBodyBytes,
			StreamGuard: serverHTTP.StreamGuardConfig{
				MaxDuration:   config.StreamMaxDuration,
				MaxBytes:      config.StreamMaxBytes,
				MaxConcurrent: config.StreamMaxConcurrent,
			},
			RateLimit: serverHTTP.RateLimitConfig{
				RequestsPerMinute: config.RateLimitRequestsPerMinute,
				Burst:             config.RateLimitBurst,
			},
			NonStreamTimeout: config.NonStreamTimeout,
		},
	)

	if !degraded.IsEmpty() {
		logger.Warn("[Bootstrap] Server starting in degraded mode: %v", degraded.Map())
	}

	diagnostics.PublishEnvironments(diagnostics.EnvironmentPayload{
		Host:     hostEnv,
		Captured: envCapturedAt,
	})

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      router,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return serveUntilSignal(server, logger)
}

// gatewaySubsystem adapts the start/cleanup gateway pattern to the Subsystem interface.
type gatewaySubsystem struct {
	name    string
	startFn func(ctx context.Context) (func(), error)
	cleanup func()
}

func (g *gatewaySubsystem) Name() string { return g.name }

func (g *gatewaySubsystem) Start(ctx context.Context) error {
	cleanup, err := g.startFn(ctx)
	if err != nil {
		return err
	}
	g.cleanup = cleanup
	return nil
}

func (g *gatewaySubsystem) Stop() {
	if g.cleanup != nil {
		g.cleanup()
	}
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
	async.Go(logger, "server.listen", func() {
		logger.Info("Server listening on %s", server.Addr)
		errCh <- server.ListenAndServe()
	})

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
