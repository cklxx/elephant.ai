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

	"alex/internal/app/subscription"
	"alex/internal/app/workdir"
	serverApp "alex/internal/delivery/server/app"
	"alex/internal/delivery/server/ports"
	serverHTTP "alex/internal/delivery/server/http"
	agentdomain "alex/internal/domain/agent"
	"alex/internal/domain/materials"
	"alex/internal/infra/analytics"
	"alex/internal/infra/attachments"
	"alex/internal/infra/diagnostics"
	"alex/internal/infra/external/bridge"
	"alex/internal/infra/httpclient"
	"alex/internal/infra/sandbox"
	taskinfra "alex/internal/infra/task"
	"alex/internal/shared/async"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// RunServer starts the HTTP API server and blocks until a shutdown signal is received.
func RunServer(observabilityConfigPath string) error {
	logger := logging.NewComponentLogger("Main")
	logger.Info("Starting elephant.ai SSE Server...")

	// ── Phase 1: Required infrastructure (failure aborts startup) ──

	f, err := BootstrapFoundation(observabilityConfigPath, logger)
	if err != nil {
		return err
	}
	defer f.Cleanup()

	config := f.Config
	container := f.Container

	// ── Phase 2: Optional services (failure records degraded, continues) ──

	var attachmentStore *attachments.Store
	var historyStore serverApp.EventHistoryStore
	var asyncHistoryStore *serverApp.AsyncEventHistoryStore
	var analyticsClient analytics.Client
	var analyticsCleanup func()

	config.Attachment = attachments.NormalizeConfig(config.Attachment)
	optionalStages := []BootstrapStage{
		{
			Name: "attachments", Required: false,
			Init: func() error {
				store, err := attachments.NewStore(config.Attachment)
				if err != nil {
					return err
				}
				attachmentStore = store
				client := httpclient.NewWithCircuitBreaker(45*time.Second, logger, "attachment_migrator")
				migrator := materials.NewAttachmentStoreMigrator(store, client, config.Attachment.CloudflarePublicBaseURL, logger)
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
				if config.EventHistory.Retention > 0 {
					historyOpts = append(historyOpts, serverApp.WithHistoryRetention(config.EventHistory.Retention))
				}
				pgHistory := serverApp.NewPostgresEventHistoryStore(container.SessionDB, historyOpts...)
				if err := pgHistory.EnsureSchema(ctx); err != nil {
					return err
				}
				historyStore = pgHistory
				asyncHistoryOpts := []serverApp.AsyncEventHistoryStoreOption{}
				if config.EventHistory.AsyncBatchSize > 0 {
					asyncHistoryOpts = append(asyncHistoryOpts, serverApp.WithAsyncHistoryBatchSize(config.EventHistory.AsyncBatchSize))
				}
				if config.EventHistory.AsyncFlushInterval > 0 {
					asyncHistoryOpts = append(asyncHistoryOpts, serverApp.WithAsyncHistoryFlushInterval(config.EventHistory.AsyncFlushInterval))
				}
				if config.EventHistory.AsyncAppendTimeout > 0 {
					asyncHistoryOpts = append(asyncHistoryOpts, serverApp.WithAsyncHistoryAppendTimeout(config.EventHistory.AsyncAppendTimeout))
				}
				if config.EventHistory.AsyncQueueCapacity > 0 {
					asyncHistoryOpts = append(asyncHistoryOpts, serverApp.WithAsyncHistoryQueueCapacity(config.EventHistory.AsyncQueueCapacity))
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

	if err := RunStages(optionalStages, f.Degraded, logger); err != nil {
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
	if config.EventHistory.MaxEvents > 0 {
		broadcasterOpts = append(broadcasterOpts, serverApp.WithMaxHistory(config.EventHistory.MaxEvents))
	}
	if config.EventHistory.MaxSessions > 0 {
		broadcasterOpts = append(broadcasterOpts, serverApp.WithMaxSessions(config.EventHistory.MaxSessions))
	}
	if config.EventHistory.SessionTTL > 0 {
		broadcasterOpts = append(broadcasterOpts, serverApp.WithSessionTTL(config.EventHistory.SessionTTL))
	}
	broadcaster := serverApp.NewEventBroadcaster(broadcasterOpts...)

	// Task store: prefer unified Postgres store when SessionDB is available,
	// falling back to the in-memory store with file persistence.
	var taskStore ports.TaskStore
	var taskStoreCleanup func()
	if container.TaskStore != nil {
		adapter := taskinfra.NewServerAdapter(container.TaskStore)
		taskStore = adapter
		taskStoreCleanup = func() {}
		logger.Info("[Bootstrap] Task store backed by unified Postgres store")
	} else {
		taskStoreOpts := []serverApp.TaskStoreOption{}
		if sessionDir := strings.TrimSpace(container.SessionDir()); sessionDir != "" {
			taskStoreOpts = append(taskStoreOpts, serverApp.WithTaskPersistenceFile(filepath.Join(sessionDir, "_server", "tasks.json")))
		}
		memStore := serverApp.NewInMemoryTaskStore(taskStoreOpts...)
		taskStore = memStore
		taskStoreCleanup = func() { memStore.Close() }
		logger.Info("[Bootstrap] Task store backed by in-memory store (Postgres unavailable)")
	}
	defer taskStoreCleanup()
	progressTracker := serverApp.NewTaskProgressTracker(taskStore)

	cleanupDiagnostics := subscribeDiagnostics(broadcaster)
	defer cleanupDiagnostics()

	journalReader := BuildJournalReader(container.SessionDir(), logger)

	coordinatorOpts := []serverApp.ServerCoordinatorOption{
		serverApp.WithAnalyticsClient(analyticsClient),
		serverApp.WithJournalReader(journalReader),
		serverApp.WithObservability(f.Obs),
		serverApp.WithHistoryStore(container.HistoryStore),
		serverApp.WithProgressTracker(progressTracker),
	}

	// Wire bridge orphan resumer when unified task store is available.
	if container.TaskStore != nil {
		bridgeWorkDir := workdir.DefaultWorkingDir()
		resumer := bridge.NewResumer(container.TaskStore, bridge.New(bridge.BridgeConfig{}), nil)
		coordinatorOpts = append(coordinatorOpts,
			serverApp.WithCoordinatorBridgeResumer(
				&bridgeResumerAdapter{resumer: resumer},
				bridgeWorkDir,
			),
		)
	}

	serverCoordinator := serverApp.NewServerCoordinator(
		container.AgentCoordinator,
		broadcaster,
		container.SessionStore,
		taskStore,
		container.StateStore,
		coordinatorOpts...,
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
		f.SchedulerStage(subsystems),
		f.TimerManagerStage(subsystems),
	}

	if err := RunStages(gatewayStages, f.Degraded, logger); err != nil {
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
	healthChecker.RegisterProbe(serverApp.NewDegradedProbe(f.Degraded))

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

	runtimeUpdates, runtimeReloader := f.RuntimeCacheUpdates()
	configHandler := serverHTTP.NewConfigHandler(f.ConfigManager(), f.Resolver(), runtimeUpdates, runtimeReloader)
	onboardingStore := subscription.NewOnboardingStateStore(
		subscription.ResolveOnboardingStatePath(runtimeconfig.DefaultEnvLookup, nil),
	)
	onboardingStateHandler := serverHTTP.NewOnboardingStateHandler(onboardingStore)
	evaluationService, err := serverApp.NewEvaluationService("./evaluation_results")
	if err != nil {
		logger.Warn("Evaluation service disabled: %v", err)
	}
	var larkOAuthHandler *serverHTTP.LarkOAuthHandler
	if container.LarkOAuth != nil {
		larkOAuthHandler = serverHTTP.NewLarkOAuthHandler(container.LarkOAuth, logger)
	}
	sandboxClient := sandbox.NewClient(sandbox.Config{
		BaseURL:          config.Runtime.SandboxBaseURL,
		MaxResponseBytes: config.Runtime.HTTPLimits.SandboxMaxResponseBytes,
	})

	// Hooks bridge: forward Claude Code hook events to Lark.
	var hooksBridge http.Handler
	if container.LarkGateway != nil {
		hooksBridge = buildHooksBridge(config, container, logger)
	}

	router := serverHTTP.NewRouter(
		serverHTTP.RouterDeps{
			Coordinator:            serverCoordinator,
			Broadcaster:            broadcaster,
			RunTracker:             progressTracker,
			HealthChecker:          healthChecker,
			AuthHandler:            authHandler,
			AuthService:            authService,
			ConfigHandler:          configHandler,
			OnboardingStateHandler: onboardingStateHandler,
			Evaluation:             evaluationService,
			Obs:                    f.Obs,
			AttachmentCfg:          config.Attachment,
			SandboxClient:          sandboxClient,
			LarkOAuthHandler:       larkOAuthHandler,
			MemoryEngine:           container.MemoryEngine,
			HooksBridge:            hooksBridge,
		},
		serverHTTP.RouterConfig{
			Environment:      config.Runtime.Environment,
			AllowedOrigins:   config.AllowedOrigins,
			MaxTaskBodyBytes: config.MaxTaskBodyBytes,
			StreamGuard: serverHTTP.StreamGuardConfig{
				MaxDuration:   config.StreamGuard.MaxDuration,
				MaxBytes:      config.StreamGuard.MaxBytes,
				MaxConcurrent: config.StreamGuard.MaxConcurrent,
			},
			RateLimit: serverHTTP.RateLimitConfig{
				RequestsPerMinute: config.RateLimit.RequestsPerMinute,
				Burst:             config.RateLimit.Burst,
			},
			NonStreamTimeout: config.NonStreamTimeout,
		},
	)

	if !f.Degraded.IsEmpty() {
		logger.Warn("[Bootstrap] Server starting in degraded mode: %v", f.Degraded.Map())
	}

	diagnostics.PublishEnvironments(diagnostics.EnvironmentPayload{
		Host:     f.HostEnv,
		Captured: f.EnvCapturedAt,
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

// bridgeResumerAdapter wraps bridge.Resumer to implement serverApp.BridgeOrphanResumer.
type bridgeResumerAdapter struct {
	resumer *bridge.Resumer
}

func (a *bridgeResumerAdapter) ResumeOrphans(ctx context.Context, workDir string) []serverApp.OrphanResumeResult {
	results := a.resumer.ResumeOrphans(ctx, workDir)
	out := make([]serverApp.OrphanResumeResult, len(results))
	for i, r := range results {
		out[i] = serverApp.OrphanResumeResult{
			TaskID: r.TaskID,
			Action: string(r.Action),
			Error:  r.Error,
		}
	}
	return out
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
