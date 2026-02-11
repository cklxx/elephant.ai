package bootstrap

import (
	"context"

	"alex/internal/shared/async"
)

// KernelStage returns a BootstrapStage that starts the kernel agent loop engine
// if enabled.
func (f *Foundation) KernelStage(sm *SubsystemManager) BootstrapStage {
	return BootstrapStage{
		Name: "kernel", Required: false,
		Init: func() error {
			if !f.Config.Runtime.Proactive.Kernel.Enabled || f.Container.KernelEngine == nil {
				return nil
			}
			return sm.Start(context.Background(), &gatewaySubsystem{
				name: "kernel",
				startFn: func(ctx context.Context) (func(), error) {
					engine := f.Container.KernelEngine
					async.Go(f.Logger, "kernel-engine", func() {
						engine.Run(ctx)
					})
					return engine.Stop, nil
				},
			})
		},
	}
}
