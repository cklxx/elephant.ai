package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"alex/internal/delivery/channels/lark"
	serverApp "alex/internal/delivery/server/app"
	serverHTTP "alex/internal/delivery/server/http"
	agentdomain "alex/internal/domain/agent"
	"alex/internal/domain/materials"
	"alex/internal/infra/analytics"
	"alex/internal/infra/attachments"
	"alex/internal/infra/diagnostics"
	"alex/internal/shared/async"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
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

	config, configManager, resolver, runtimeCache, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	LogServerConfiguration(logger, config)

	if runtimeCache != nil {
		for _, configPath := range runtimeconfig.DefaultRuntimeConfigWatchPaths(runtimeconfig.DefaultEnvLookup, nil) {
			configWatcher, err := runtimeconfig.NewRuntimeConfigWatcher(
				configPath,
				runtimeCache,
				runtimeconfig.WithConfigWatchLogger(logger),
				runtimeconfig.WithConfigWatchBeforeReload(func(ctx context.Context) error {
					_, err := configManager.RefreshOverrides(ctx)
					return err
				}),
			)
			if err != nil {
				logger.Warn("Config watcher disabled for %s: %v", configPath, err)
				continue
			}
			if err := configWatcher.Start(context.Background()); err != nil {
				logger.Warn("Config watcher failed to start for %s: %v", configPath, err)
				continue
			}
			defer configWatcher.Stop()
		}
	}

	hostEnv, hostSummary := CaptureHostEnvironment(20)
	config.EnvironmentSummary = hostSummary
	envCapturedAt := time.Now().UTC()

	container, err := BuildContainer(config)
	if err != nil {
		return fmt.Errorf("build container: %w", err)
	}
	if resolver != nil && container != nil && container.AgentCoordinator != nil {
		container.AgentCoordinator.SetRuntimeConfigResolver(resolver)
	}
	defer func() {
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer drainCancel()
		if err := container.Drain(drainCtx); err != nil {
			logger.Warn("Failed to drain/shutdown container: %v", err)
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
				asyncHistoryOpts := []serverApp.AsyncEventHistoryStoreOption{}
				if config.EventHistoryAsyncBatchSize > 0 {
					asyncHistoryOpts = append(asyncHistoryOpts, serverApp.WithAsyncHistoryBatchSize(config.EventHistoryAsyncBatchSize))
				}
				if config.EventHistoryAsyncFlushInterval > 0 {
					asyncHistoryOpts = append(asyncHistoryOpts, serverApp.WithAsyncHistoryFlushInterval(config.EventHistoryAsyncFlushInterval))
				}
				if config.EventHistoryAsyncAppendTimeout > 0 {
					asyncHistoryOpts = append(asyncHistoryOpts, serverApp.WithAsyncHistoryAppendTimeout(config.EventHistoryAsyncAppendTimeout))
				}
				if config.EventHistoryAsyncQueueCapacity > 0 {
					asyncHistoryOpts = append(asyncHistoryOpts, serverApp.WithAsyncHistoryQueueCapacity(config.EventHistoryAsyncQueueCapacity))
				}
				asyncHistoryStore = serverApp.NewAsyncEventHistoryStore(pgHistory, asyncHistoryOpts...)
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
	taskStoreOpts := []serverApp.TaskStoreOption{}
	if sessionDir := strings.TrimSpace(container.SessionDir()); sessionDir != "" {
		taskStoreOpts = append(taskStoreOpts, serverApp.WithTaskPersistenceFile(filepath.Join(sessionDir, "_server", "tasks.json")))
	}
	taskStore := serverApp.NewInMemoryTaskStore(taskStoreOpts...)
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
	if resumed, err := serverCoordinator.ResumePendingTasks(context.Background()); err != nil {
		logger.Warn("[Bootstrap] Failed to resume pending/running tasks: %v", err)
	} else if resumed > 0 {
		logger.Info("[Bootstrap] Resumed %d pending/running tasks", resumed)
	}

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
						container.Drainables = append(container.Drainables, sched)
						return sched.Stop, nil
					},
				})
			},
		},
		{
			Name: "timer-manager", Required: false,
			Init: func() error {
				if !config.Runtime.Proactive.Timer.Enabled {
					return nil
				}
				return subsystems.Start(context.Background(), &gatewaySubsystem{
					name: "timer-manager",
					startFn: func(ctx context.Context) (func(), error) {
						mgr := startTimerManager(ctx, config, container, logger)
						if mgr == nil {
							return nil, fmt.Errorf("timer-manager init returned nil")
						}
						return mgr.Stop, nil
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

	var runtimeUpdates <-chan struct{}
	var runtimeReloader func(context.Context) error
	if runtimeCache != nil {
		runtimeUpdates = runtimeCache.Updates()
		runtimeReloader = runtimeCache.Reload
	}
	configHandler := serverHTTP.NewConfigHandler(configManager, resolver, runtimeUpdates, runtimeReloader)
	evaluationService, err := serverApp.NewEvaluationService("./evaluation_results")
	if err != nil {
		logger.Warn("Evaluation service disabled: %v", err)
	}
	var larkCardHandler http.Handler
	if container.LarkGateway != nil {
		larkCardHandler = lark.NewCardCallbackHandler(container.LarkGateway, logger)
	}
	var larkOAuthHandler *serverHTTP.LarkOAuthHandler
	if container.LarkOAuth != nil {
		larkOAuthHandler = serverHTTP.NewLarkOAuthHandler(container.LarkOAuth, logger)
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
			AttachmentCfg:           config.Attachment,
			SandboxBaseURL:          config.Runtime.SandboxBaseURL,
			SandboxMaxResponseBytes: config.Runtime.HTTPLimits.SandboxMaxResponseBytes,
			LarkCardHandler:         larkCardHandler,
			LarkOAuthHandler:        larkOAuthHandler,
			MemoryEngine:            container.MemoryEngine,
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
