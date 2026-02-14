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
	serverApp "alex/internal/delivery/server/app"
	serverHTTP "alex/internal/delivery/server/http"
	"alex/internal/delivery/server/ports"
	agentdomain "alex/internal/domain/agent"
	"alex/internal/domain/materials"
	"alex/internal/infra/analytics"
	"alex/internal/infra/attachments"
	"alex/internal/infra/diagnostics"
	"alex/internal/infra/httpclient"
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

	var historyStore serverApp.EventHistoryStore
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
				eventsDir := filepath.Join(container.SessionDir(), "_server")
				fileHistory := serverApp.NewFileEventHistoryStore(eventsDir)
				if err := fileHistory.EnsureSchema(context.Background()); err != nil {
					return err
				}
				historyStore = fileHistory
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

	if analyticsCleanup != nil {
		defer analyticsCleanup()
	}
	broadcasterOpts := []serverApp.EventBroadcasterOption{
		serverApp.WithEventHistoryStore(historyStore),
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

	// Task store: in-memory with file persistence.
	var taskStore ports.TaskStore
	taskStoreOpts := []serverApp.TaskStoreOption{}
	if sessionDir := strings.TrimSpace(container.SessionDir()); sessionDir != "" {
		taskStoreOpts = append(taskStoreOpts, serverApp.WithTaskPersistenceFile(filepath.Join(sessionDir, "_server", "tasks.json")))
	}
	memStore := serverApp.NewInMemoryTaskStore(taskStoreOpts...)
	taskStore = memStore
	defer memStore.Close()
	progressTracker := serverApp.NewTaskProgressTracker(taskStore)

	cleanupDiagnostics := subscribeDiagnostics(broadcaster)
	defer cleanupDiagnostics()

	journalReader := BuildJournalReader(container.SessionDir(), logger)

	// ── Build the 3 standalone services ──

	taskOpts := []serverApp.TaskExecutionServiceOption{
		serverApp.WithTaskAnalytics(analyticsClient),
		serverApp.WithTaskObservability(f.Obs),
		serverApp.WithTaskProgressTracker(progressTracker),
		serverApp.WithTaskStateStore(container.StateStore),
	}
	if ownerID := strings.TrimSpace(config.TaskExecution.OwnerID); ownerID != "" {
		taskOpts = append(taskOpts, serverApp.WithTaskOwnerID(ownerID))
	}
	if config.TaskExecution.LeaseTTL > 0 || config.TaskExecution.LeaseRenewInterval > 0 {
		taskOpts = append(taskOpts, serverApp.WithTaskLeaseConfig(config.TaskExecution.LeaseTTL, config.TaskExecution.LeaseRenewInterval))
	}
	if config.TaskExecution.MaxInFlight > 0 {
		taskOpts = append(taskOpts, serverApp.WithTaskAdmissionLimit(config.TaskExecution.MaxInFlight))
	} else {
		// MaxInFlight == 0 explicitly disables admission limiter.
		taskOpts = append(taskOpts, serverApp.WithTaskAdmissionLimit(config.TaskExecution.MaxInFlight))
	}
	if config.TaskExecution.ResumeClaimBatchSize > 0 {
		taskOpts = append(taskOpts, serverApp.WithResumeClaimBatchSize(config.TaskExecution.ResumeClaimBatchSize))
	}

	tasksSvc := serverApp.NewTaskExecutionService(
		container.AgentCoordinator,
		broadcaster,
		taskStore,
		taskOpts...,
	)

	sessionsSvc := serverApp.NewSessionService(
		container.AgentCoordinator,
		container.SessionStore,
		broadcaster,
		serverApp.WithSessionStateStore(container.StateStore),
		serverApp.WithSessionHistoryStore(container.HistoryStore),
	)

	snapshotsSvc := serverApp.NewSnapshotService(
		container.AgentCoordinator,
		broadcaster,
		serverApp.WithSnapshotStateStore(container.StateStore),
		serverApp.WithSnapshotJournalReader(journalReader),
	)

	if resumed, err := tasksSvc.ResumePendingTasks(context.Background()); err != nil {
		logger.Warn("[Bootstrap] Failed to resume pending/running tasks: %v", err)
	} else if resumed > 0 {
		logger.Info("[Bootstrap] Resumed %d pending/running tasks", resumed)
	}

	// ── Phase 3: Subsystems (scheduler/timer only — NO Lark gateway) ──
	// IMPORTANT: Lark gateway is NOT started in web server mode to prevent
	// duplicate message processing when both `alex-server` and `alex-server lark`
	// are running. Lark gateway should ONLY run in standalone mode via `alex-server lark`.

	subsystems := NewSubsystemManager(logger)
	defer subsystems.StopAll()

	gatewayStages := []BootstrapStage{
		// Lark gateway removed - use `alex-server lark` for Lark integration
		// Kernel removed - only runs in Lark standalone mode (`alex-server lark`)
		f.SchedulerStage(subsystems),
		f.TimerManagerStage(subsystems),
	}

	if err := RunStages(gatewayStages, f.Degraded, logger); err != nil {
		return fmt.Errorf("gateway stages: %w", err)
	}

	// ── Phase 4: HTTP layer ──

	healthChecker := serverApp.NewHealthChecker()
	healthChecker.RegisterProbe(serverApp.NewMCPProbe(container, config.EnableMCP))
	healthChecker.RegisterProbe(serverApp.NewLLMFactoryProbe(container))
	healthChecker.RegisterProbe(serverApp.NewDegradedProbe(f.Degraded))

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
	// Hooks bridge: forward Claude Code hook events to Lark.
	var hooksBridge http.Handler
	if container.LarkGateway != nil {
		hooksBridge = buildHooksBridge(config, container, logger)
	}

	router := serverHTTP.NewRouter(
		serverHTTP.RouterDeps{
			Tasks:                  tasksSvc,
			Sessions:               sessionsSvc,
			Snapshots:              snapshotsSvc,
			Broadcaster:            broadcaster,
			RunTracker:             progressTracker,
			HealthChecker:          healthChecker,
			ConfigHandler:          configHandler,
			OnboardingStateHandler: onboardingStateHandler,
			Evaluation:             evaluationService,
			Obs:                    f.Obs,
			AttachmentCfg:          config.Attachment,
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
		event := agentdomain.NewDiagnosticEnvironmentSnapshotEvent(payload.Host, payload.Captured)
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
