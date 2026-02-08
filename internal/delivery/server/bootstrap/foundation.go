package bootstrap

import (
	"context"
	"fmt"
	"time"

	"alex/internal/app/di"
	"alex/internal/domain/materials"
	"alex/internal/infra/attachments"
	"alex/internal/infra/observability"
	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
	"alex/internal/shared/logging"
)

// Foundation holds the shared infrastructure that both RunServer and RunLark
// require: observability, config, runtime cache, DI container, and environment
// summary. Create via BootstrapFoundation and defer Cleanup().
type Foundation struct {
	Config        Config
	ConfigResult  ConfigResult
	Container     *di.Container
	Obs           *observability.Observability
	Logger        logging.Logger
	Degraded      *DegradedComponents
	HostEnv       map[string]string
	EnvCapturedAt time.Time

	cleanups []func() // cleanup functions in reverse order
}

// BootstrapFoundation performs the shared Phase 1 initialization:
// observability, config loading, config watchers, host environment capture,
// DI container build, and setter injection.
func BootstrapFoundation(observabilityConfigPath string, logger logging.Logger) (*Foundation, error) {
	f := &Foundation{
		Logger:   logger,
		Degraded: NewDegradedComponents(),
	}

	// 1. Observability
	obs, cleanupObs := InitObservability(observabilityConfigPath, logger)
	f.Obs = obs
	if cleanupObs != nil {
		f.addCleanup(cleanupObs)
	}

	// 2. Config
	cr, err := LoadConfig()
	if err != nil {
		f.Cleanup()
		return nil, fmt.Errorf("load config: %w", err)
	}
	f.ConfigResult = cr
	f.Config = cr.Config

	LogServerConfiguration(logger, f.Config)

	// 3. Config watchers
	f.startConfigWatchers(cr)

	// 4. Host environment
	hostEnv, hostSummary := CaptureHostEnvironment(20)
	f.HostEnv = hostEnv
	f.Config.EnvironmentSummary = hostSummary
	f.EnvCapturedAt = time.Now().UTC()

	// 5. DI Container
	container, err := BuildContainer(f.Config)
	if err != nil {
		f.Cleanup()
		return nil, fmt.Errorf("build container: %w", err)
	}
	f.Container = container
	f.addCleanup(func() {
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer drainCancel()
		if err := container.Drain(drainCtx); err != nil {
			logger.Warn("Failed to drain/shutdown container: %v", err)
		}
	})

	// 6. Setter injection
	if cr.Resolver != nil && container.AgentCoordinator != nil {
		container.AgentCoordinator.SetRuntimeConfigResolver(cr.Resolver)
	}

	if err := container.Start(); err != nil {
		logger.Warn("Container start failed: %v (continuing with limited functionality)", err)
	}

	if summary := f.Config.EnvironmentSummary; summary != "" {
		container.AgentCoordinator.SetEnvironmentSummary(summary)
	}

	return f, nil
}

// Cleanup releases all resources in reverse order.
func (f *Foundation) Cleanup() {
	for i := len(f.cleanups) - 1; i >= 0; i-- {
		f.cleanups[i]()
	}
	f.cleanups = nil
}

func (f *Foundation) addCleanup(fn func()) {
	f.cleanups = append(f.cleanups, fn)
}

// startConfigWatchers sets up file-based config watchers for hot reload.
func (f *Foundation) startConfigWatchers(cr ConfigResult) {
	if cr.RuntimeCache == nil {
		return
	}
	for _, configPath := range runtimeconfig.DefaultRuntimeConfigWatchPaths(runtimeconfig.DefaultEnvLookup, nil) {
		watcher, err := runtimeconfig.NewRuntimeConfigWatcher(
			configPath,
			cr.RuntimeCache,
			runtimeconfig.WithConfigWatchLogger(f.Logger),
			runtimeconfig.WithConfigWatchBeforeReload(func(ctx context.Context) error {
				_, err := cr.ConfigManager.RefreshOverrides(ctx)
				return err
			}),
		)
		if err != nil {
			f.Logger.Warn("Config watcher disabled for %s: %v", configPath, err)
			continue
		}
		if err := watcher.Start(context.Background()); err != nil {
			f.Logger.Warn("Config watcher failed to start for %s: %v", configPath, err)
			continue
		}
		f.addCleanup(watcher.Stop)
	}
}

// AttachmentStage returns a BootstrapStage that initializes attachments and
// wires the attachment migrator/persister into the coordinator.
func (f *Foundation) AttachmentStage() BootstrapStage {
	return BootstrapStage{
		Name: "attachments", Required: false,
		Init: func() error {
			f.Config.Attachment = attachments.NormalizeConfig(f.Config.Attachment)
			store, err := attachments.NewStore(f.Config.Attachment)
			if err != nil {
				return err
			}
			migrator := materials.NewAttachmentStoreMigrator(store, nil, f.Config.Attachment.CloudflarePublicBaseURL, f.Logger)
			f.Container.AgentCoordinator.SetAttachmentMigrator(migrator)
			f.Container.AgentCoordinator.SetAttachmentPersister(
				attachments.NewStorePersister(store),
			)
			return nil
		},
	}
}

// SchedulerStage returns a BootstrapStage that starts the proactive scheduler
// if enabled.
func (f *Foundation) SchedulerStage(sm *SubsystemManager) BootstrapStage {
	return BootstrapStage{
		Name: "scheduler", Required: false,
		Init: func() error {
			if !f.Config.Runtime.Proactive.Scheduler.Enabled {
				return nil
			}
			return sm.Start(context.Background(), &gatewaySubsystem{
				name: "scheduler",
				startFn: func(ctx context.Context) (func(), error) {
					sched := startScheduler(ctx, f.Config, f.Container, f.Logger)
					if sched == nil {
						return nil, fmt.Errorf("scheduler init returned nil")
					}
					f.Container.Drainables = append(f.Container.Drainables, sched)
					return sched.Stop, nil
				},
			})
		},
	}
}

// TimerManagerStage returns a BootstrapStage that starts the timer manager
// if enabled.
func (f *Foundation) TimerManagerStage(sm *SubsystemManager) BootstrapStage {
	return BootstrapStage{
		Name: "timer-manager", Required: false,
		Init: func() error {
			if !f.Config.Runtime.Proactive.Timer.Enabled {
				return nil
			}
			return sm.Start(context.Background(), &gatewaySubsystem{
				name: "timer-manager",
				startFn: func(ctx context.Context) (func(), error) {
					mgr := startTimerManager(ctx, f.Config, f.Container, f.Logger)
					if mgr == nil {
						return nil, fmt.Errorf("timer-manager init returned nil")
					}
					return mgr.Stop, nil
				},
			})
		},
	}
}

// RuntimeCacheUpdates returns a channel for runtime config update notifications
// and a reload function if the runtime cache is available.
func (f *Foundation) RuntimeCacheUpdates() (<-chan struct{}, func(context.Context) error) {
	if f.ConfigResult.RuntimeCache == nil {
		return nil, nil
	}
	return f.ConfigResult.RuntimeCache.Updates(), f.ConfigResult.RuntimeCache.Reload
}

// ConfigManager returns the config admin manager.
func (f *Foundation) ConfigManager() *configadmin.Manager {
	return f.ConfigResult.ConfigManager
}

// Resolver returns the runtime config resolver function.
func (f *Foundation) Resolver() func(context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
	return f.ConfigResult.Resolver
}
