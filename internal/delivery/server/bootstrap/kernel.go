package bootstrap

import (
	"context"
	"fmt"
	"time"

	kernel "alex/internal/app/agent/kernel"
	"alex/internal/app/lifecycle"
	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
)

// KernelStage returns a BootstrapStage that starts the kernel agent loop engine
// if enabled.
func (f *Foundation) KernelStage(sm *SubsystemManager) BootstrapStage {
	logger := logging.NewKernelLogger("KernelStage")
	return BootstrapStage{
		Name: "kernel", Required: false,
		Init: func() error {
			if f.Container.KernelEngine == nil {
				return fmt.Errorf("kernel engine unavailable")
			}

			// Wire cycle notifier via LarkGateway /notice binding.
			if gw := f.Container.LarkGateway; gw != nil {
				defaults := kernel.DefaultRuntimeSettings()
				kernelID := defaults.KernelID
				if kernelID == "" {
					kernelID = kernel.DefaultKernelID
				}
				loader := gw.NoticeLoader()

				// Raw sender: resolves /notice target and delivers text.
				rawSender := func(ctx context.Context, text string) {
					chatID, ok, loadErr := loader()
					if loadErr != nil {
						logger.Warn("Kernel notifier: load notice target: %v", loadErr)
						return
					}
					if !ok {
						return // notice not bound, skip
					}
					if sendErr := gw.SendNotification(ctx, chatID, text); sendErr != nil {
						logger.Warn("Kernel notifier: send failed: %v", sendErr)
					}
				}

				windowMins := defaults.NotifyWindowMinutes
				if windowMins > 0 {
					// Aggregated mode: buffer routine successes, flush periodically.
					aggregator := kernel.NewCycleAggregator(
						kernelID,
						time.Duration(windowMins)*time.Minute,
						rawSender,
					)
					f.Container.KernelEngine.SetNotifier(aggregator.HandleCycle)

					// Ensure aggregator flushes on shutdown.
					f.Container.Drainables = append(f.Container.Drainables,
						lifecycle.DrainFunc{
							DrainName: "kernel-cycle-aggregator",
							Fn:        func(ctx context.Context) { aggregator.Close(ctx) },
						},
					)
				} else {
					// Legacy per-cycle notification.
					f.Container.KernelEngine.SetNotifier(func(ctx context.Context, result *kerneldomain.CycleResult, err error) {
						text := kernel.FormatCycleNotification(kernelID, result, err)
						rawSender(ctx, text)
					})
				}
			}

			logger.Info("Starting kernel engine subsystem")
			return sm.Start(context.Background(), &gatewaySubsystem{
				name: "kernel",
				startFn: func(ctx context.Context) (func(), error) {
					engine := f.Container.KernelEngine
					async.Go(f.Logger, "kernel-engine", func() {
						engine.Run(ctx)
					})
					// Register as drainable so graceful shutdown waits for in-flight cycles.
					if drainable, ok := engine.(lifecycle.Drainable); ok {
						f.Container.Drainables = append(f.Container.Drainables, drainable)
					}
					logger.Info("Kernel engine subsystem started")
					return engine.Stop, nil
				},
			})
		},
	}
}
