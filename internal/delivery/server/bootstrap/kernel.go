package bootstrap

import (
	"context"

	kernel "alex/internal/app/agent/kernel"
	kerneldomain "alex/internal/domain/kernel"
	"alex/internal/app/lifecycle"
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
			if !f.Config.Runtime.Proactive.Kernel.Enabled || f.Container.KernelEngine == nil {
				return nil
			}

			// Wire cycle notifier via LarkGateway /notice binding.
			if gw := f.Container.LarkGateway; gw != nil {
				kernelID := f.Config.Runtime.Proactive.Kernel.KernelID
				loader := gw.NoticeLoader()
				f.Container.KernelEngine.SetNotifier(func(ctx context.Context, result *kerneldomain.CycleResult, err error) {
					chatID, ok, loadErr := loader()
					if loadErr != nil {
						logger.Warn("Kernel notifier: load notice target: %v", loadErr)
						return
					}
					if !ok {
						return // notice not bound, skip
					}
					text := kernel.FormatCycleNotification(kernelID, result, err)
					if sendErr := gw.SendNotification(ctx, chatID, text); sendErr != nil {
						logger.Warn("Kernel notifier: send failed: %v", sendErr)
					}
				})
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
