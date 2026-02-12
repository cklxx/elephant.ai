package bootstrap

import (
	"context"
	"fmt"
	"time"

	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/logging"
)

type kernelCycleRunner interface {
	RunCycle(ctx context.Context) (*kerneldomain.CycleResult, error)
}

// RunKernelOnce executes one real kernel cycle using the current runtime config.
func RunKernelOnce(observabilityConfigPath string) error {
	logger := logging.NewKernelLogger("KernelOnce")
	logger.Info("Starting elephant.ai kernel single-cycle mode...")

	f, err := BootstrapFoundation(observabilityConfigPath, logger)
	if err != nil {
		return err
	}
	defer f.Cleanup()

	cfg := f.Config.Runtime.Proactive.Kernel
	if !cfg.Enabled {
		return fmt.Errorf("kernel single-cycle requires runtime.proactive.kernel.enabled=true")
	}
	if f.Container.KernelEngine == nil {
		return fmt.Errorf("kernel single-cycle requires initialized kernel engine (session database and kernel config)")
	}

	runner, ok := f.Container.KernelEngine.(kernelCycleRunner)
	if !ok {
		return fmt.Errorf("kernel engine does not support single-cycle execution")
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	// Buffer for setup/flush work around the core cycle timeout.
	timeout += 30 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	startedAt := time.Now()
	result, cycleErr := runner.RunCycle(ctx)
	if result == nil {
		if cycleErr != nil {
			return fmt.Errorf("kernel cycle failed without result: %w", cycleErr)
		}
		return fmt.Errorf("kernel cycle returned nil result")
	}

	fmt.Printf(
		"kernel_once: kernel=%s cycle=%s status=%s dispatched=%d succeeded=%d failed=%d duration=%s elapsed=%s\n",
		result.KernelID,
		result.CycleID,
		result.Status,
		result.Dispatched,
		result.Succeeded,
		result.Failed,
		result.Duration,
		time.Since(startedAt).Round(time.Millisecond),
	)
	if len(result.FailedAgents) > 0 {
		fmt.Printf("kernel_once: failed_agents=%v\n", result.FailedAgents)
	}

	if cycleErr != nil {
		return fmt.Errorf("kernel cycle failed: %w", cycleErr)
	}
	if result.Status != kerneldomain.CycleSuccess {
		return fmt.Errorf("kernel cycle finished with status=%s", result.Status)
	}
	return nil
}
