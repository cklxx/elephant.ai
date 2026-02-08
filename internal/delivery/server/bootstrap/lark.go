package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"alex/internal/shared/logging"
)

// RunLark starts the standalone Lark WebSocket gateway and blocks until a
// shutdown signal is received. Unlike RunServer, it skips the HTTP layer,
// session migration, event history, analytics, broadcaster, and auth — only
// the Lark gateway and its direct dependencies are started.
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

	// No HTTP transport needed in standalone mode.
	config.EnableMCP = false

	// ── Phase 2: Optional services (attachments only) ──

	optionalStages := []BootstrapStage{
		f.AttachmentStage(),
	}

	if err := RunStages(optionalStages, f.Degraded, logger); err != nil {
		return fmt.Errorf("optional stages: %w", err)
	}

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
						return startLarkGateway(ctx, config, container, logger, nil)
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

	if !f.Degraded.IsEmpty() {
		logger.Warn("[Bootstrap] Lark standalone starting in degraded mode: %v", f.Degraded.Map())
	}

	// ── Phase 4: Block until signal ──

	logger.Info("Lark standalone mode running (WS). Waiting for signal...")
	return waitForSignal(logger)
}

// waitForSignal blocks until SIGINT or SIGTERM is received.
func waitForSignal(logger logging.Logger) error {
	logger = logging.OrNop(logger)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	sig := <-quit
	logger.Info("Received signal %v, shutting down...", sig)
	return nil
}
