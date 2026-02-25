package bootstrap

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"alex/internal/shared/logging"
)

// RunKernelDaemon starts only the kernel subsystem and blocks until shutdown.
func RunKernelDaemon(observabilityConfigPath string) error {
	logger := logging.NewKernelLogger("KernelDaemon")
	logger.Info("Starting elephant.ai kernel daemon mode...")

	f, err := BootstrapFoundation(observabilityConfigPath, logger)
	if err != nil {
		return err
	}
	defer f.Cleanup()

	if f.Container.KernelEngine == nil {
		return fmt.Errorf("kernel daemon requires initialized kernel engine")
	}

	subsystems := NewSubsystemManager(logger)
	defer subsystems.StopAll()

	stage := f.KernelStage(subsystems)
	stage.Required = true
	if err := RunStages([]BootstrapStage{stage}, f.Degraded, logger); err != nil {
		return fmt.Errorf("kernel stage: %w", err)
	}

	if !f.Degraded.IsEmpty() {
		logger.Warn("[Bootstrap] Kernel daemon starting in degraded mode: %v", f.Degraded.Map())
	}

	logger.Info("Kernel daemon running. Waiting for signal...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)

	sig := <-quit
	logger.Info("Kernel daemon received signal %v, shutting down...", sig)
	logger.Info("Kernel daemon stopped")
	return nil
}
