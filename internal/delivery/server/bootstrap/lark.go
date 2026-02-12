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

	"alex/internal/infra/diagnostics"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
)

// RunLark starts the standalone Lark WebSocket gateway with an embedded debug
// HTTP server and blocks until a shutdown signal is received.  The debug server
// exposes health, SSE event streaming, and runtime config endpoints on
// cfg.DebugPort (default :9090) — no auth, no CORS, no rate limiting.
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
	larkCfg := config.Channels.Lark
	if !larkCfg.Enabled {
		return fmt.Errorf("lark standalone mode requires channels.lark.enabled = true")
	}
	if strings.TrimSpace(larkCfg.AppID) == "" || strings.TrimSpace(larkCfg.AppSecret) == "" {
		return fmt.Errorf("lark standalone mode requires channels.lark.app_id and app_secret")
	}

	// No MCP needed in standalone mode.
	config.EnableMCP = false

	// ── Phase 2: Optional services (attachments only) ──

	optionalStages := []BootstrapStage{
		f.AttachmentStage(),
	}

	if err := RunStages(optionalStages, f.Degraded, logger); err != nil {
		return fmt.Errorf("optional stages: %w", err)
	}

	// ── Phase 2b: In-memory EventBroadcaster (for SSE debug stream) ──

	broadcaster := buildDebugBroadcaster(f.Obs)
	cleanupDiagnostics := subscribeDiagnostics(broadcaster)
	defer cleanupDiagnostics()

	diagnostics.PublishEnvironments(diagnostics.EnvironmentPayload{
		Host:     f.HostEnv,
		Captured: f.EnvCapturedAt,
	})

	// ── Phase 3: Subsystems (Lark gateway required, scheduler/timer optional) ──

	subsystems := NewSubsystemManager(logger)
	defer subsystems.StopAll()

	gatewayStages := []BootstrapStage{
		{
			Name: "lark-gateway", Required: true,
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
		f.KernelStage(subsystems),
	}

	if err := RunStages(gatewayStages, f.Degraded, logger); err != nil {
		return fmt.Errorf("gateway stages: %w", err)
	}

	if !f.Degraded.IsEmpty() {
		logger.Warn("[Bootstrap] Lark standalone starting in degraded mode: %v", f.Degraded.Map())
	}

	// ── Phase 4: Debug HTTP server ──

	debugServer, err := BuildDebugHTTPServer(f, broadcaster, container, config)
	if err != nil {
		return fmt.Errorf("debug HTTP server: %w", err)
	}

	debugErrCh := make(chan error, 1)
	async.Go(logger, "debug.http", func() {
		logger.Info("Debug HTTP server listening on %s", debugServer.Addr)
		debugErrCh <- debugServer.ListenAndServe()
	})

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

