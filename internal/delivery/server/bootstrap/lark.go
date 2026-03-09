package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"alex/internal/infra/diagnostics"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

// RunLark starts the standalone Lark WebSocket gateway with an embedded debug
// HTTP server and blocks until a shutdown signal is received.  The debug server
// exposes health, SSE event streaming, and runtime config endpoints on
// cfg.DebugPort (default :9090) — no auth, no rate limiting.
func RunLark(observabilityConfigPath string) error {
	logger := logging.NewComponentLogger("Main")
	logger.Info("Starting elephant.ai Lark standalone mode...")

	// ── Phase 1: Required infrastructure ──

	f, err := BootstrapFoundation(observabilityConfigPath, logger)
	if err != nil {
		return err
	}
	defer f.Cleanup()

	config := f.Config
	container := f.Container

	// Validate Lark is enabled and has credentials — fail fast.
	larkCfg := config.Channels.LarkConfig()
	if !larkCfg.Enabled {
		return fmt.Errorf("lark standalone mode requires channels.lark.enabled = true")
	}
	if utils.IsBlank(larkCfg.AppID) || utils.IsBlank(larkCfg.AppSecret) {
		return fmt.Errorf("lark standalone mode requires channels.lark.app_id and app_secret")
	}

	// ── Phase 2: Optional services (attachments only) ──

	optionalStages := []BootstrapStage{
		f.AttachmentStage(),
	}

	if err := RunStages(optionalStages, f.Degraded, logger); err != nil {
		return fmt.Errorf("optional stages: %w", err)
	}

	// ── Phase 2b: In-memory EventBroadcaster (for SSE debug stream) ──

	sessionDir := ""
	if container != nil {
		sessionDir = container.SessionDir()
	}
	broadcaster := buildDebugBroadcaster(f.Obs, sessionDir, logger)
	cleanupDiagnostics := subscribeDiagnostics(broadcaster)
	defer cleanupDiagnostics()

	diagnostics.PublishEnvironments(diagnostics.EnvironmentPayload{
		Host:     f.HostEnv,
		Captured: f.EnvCapturedAt,
	})

	// ── Phase 3: Subsystems (channel gateways, scheduler/timer) ──

	subsystems := NewSubsystemManager(logger)
	defer subsystems.StopAll()

	// Register channel plugins into the registry.
	registerLarkChannel(config, config.Channels.Registry, container, logger, broadcaster)
	registerTelegramChannel(config, config.Channels.Registry, container, logger, broadcaster)

	// Build gateway stages from the channel registry.
	var gatewayStages []BootstrapStage
	for _, plugin := range config.Channels.Registry.Plugins() {
		p := plugin // capture loop variable
		gatewayStages = append(gatewayStages, BootstrapStage{
			Name: p.Name + "-gateway", Required: p.Required,
			Init: func() error {
				return subsystems.Start(context.Background(), &gatewaySubsystem{
					name:    p.Name,
					startFn: p.Build,
				})
			},
		})
	}
	gatewayStages = append(gatewayStages,
		f.SchedulerStage(subsystems),
		f.TimerManagerStage(subsystems),
	)

	if err := RunStages(gatewayStages, f.Degraded, logger); err != nil {
		return fmt.Errorf("gateway stages: %w", err)
	}

	if !f.Degraded.IsEmpty() {
		logger.Warn("[Bootstrap] Lark standalone starting in degraded mode: %v", f.Degraded.Map())
	}

	// ── Phase 3b: Runtime watchdog ──

	watchdogCtx, watchdogCancel := context.WithCancel(context.Background())
	defer watchdogCancel()

	watchdogLogger := logging.NewComponentLogger("Watchdog")
	dumpDir := os.Getenv("ALEX_LOG_DIR")
	if dumpDir == "" {
		dumpDir = "logs"
	}
	watchdog := diagnostics.NewWatchdog(diagnostics.WatchdogConfig{
		DumpDir: dumpDir,
	}, watchdogLogger)
	async.Go(logger, "watchdog", func() {
		watchdog.Run(watchdogCtx)
	})

	// ── Phase 4: Debug HTTP server ──

	debugServer, _, err := BuildDebugHTTPServer(f, broadcaster, container, config)
	if err != nil {
		return fmt.Errorf("debug HTTP server: %w", err)
	}

	debugPort := config.DebugPort
	if debugPort == "" {
		debugPort = "9090"
	}

	debugErrCh := make(chan error, 1)
	debugLn, err := listenDebugPort(debugPort, logger)
	if err != nil {
		return fmt.Errorf("debug HTTP server: %w", err)
	}
	if debugLn != nil {
		debugServer.Addr = debugLn.Addr().String()
		async.Go(logger, "debug.http", func() {
			logger.Info("Debug HTTP server listening on %s", debugServer.Addr)
			debugErrCh <- debugServer.Serve(debugLn)
		})
	} else {
		logger.Warn("Continuing without debug HTTP server (port unavailable)")
	}

	// ── Phase 5: Block until signal ──

	logger.Info("Lark standalone mode running (WS + debug HTTP). Waiting for signal...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case err := <-debugErrCh:
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("debug HTTP server error: %w", err)
		}
	case sig := <-quit:
		logger.Info("Received signal %v, shutting down...", sig)
	}

	// Graceful shutdown of debug HTTP server.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := debugServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("Debug HTTP shutdown error: %v", err)
	}

	logger.Info("Lark standalone mode stopped")
	return nil
}
