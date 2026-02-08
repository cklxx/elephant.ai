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

	larkdelivery "alex/internal/delivery/channels/lark"
	"alex/internal/domain/materials"
	"alex/internal/infra/attachments"
	runtimeconfig "alex/internal/shared/config"
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

	obs, cleanupObs := InitObservability(observabilityConfigPath, logger)
	_ = obs
	if cleanupObs != nil {
		defer cleanupObs()
	}

	cr, err := LoadConfig()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config := cr.Config

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

	LogServerConfiguration(logger, config)

	if cr.RuntimeCache != nil {
		for _, configPath := range runtimeconfig.DefaultRuntimeConfigWatchPaths(runtimeconfig.DefaultEnvLookup, nil) {
			configWatcher, err := runtimeconfig.NewRuntimeConfigWatcher(
				configPath,
				cr.RuntimeCache,
				runtimeconfig.WithConfigWatchLogger(logger),
				runtimeconfig.WithConfigWatchBeforeReload(func(ctx context.Context) error {
					_, err := cr.ConfigManager.RefreshOverrides(ctx)
					return err
				}),
			)
			if err != nil {
				logger.Warn("Config watcher disabled for %s: %v", configPath, err)
				continue
			}
			if err := configWatcher.Start(context.Background()); err != nil {
				logger.Warn("Config watcher failed to start for %s: %v", configPath, err)
				continue
			}
			defer configWatcher.Stop()
		}
	}

	hostEnv, hostSummary := CaptureHostEnvironment(20)
	_ = hostEnv
	config.EnvironmentSummary = hostSummary

	container, err := BuildContainer(config)
	if err != nil {
		return fmt.Errorf("build container: %w", err)
	}
	if cr.Resolver != nil && container != nil && container.AgentCoordinator != nil {
		container.AgentCoordinator.SetRuntimeConfigResolver(cr.Resolver)
	}
	defer func() {
		drainCtx, drainCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer drainCancel()
		if err := container.Drain(drainCtx); err != nil {
			logger.Warn("Failed to drain/shutdown container: %v", err)
		}
	}()

	if err := container.Start(); err != nil {
		logger.Warn("Container start failed: %v (continuing with limited functionality)", err)
	}

	if summary := config.EnvironmentSummary; summary != "" {
		container.AgentCoordinator.SetEnvironmentSummary(summary)
	}

	// ── Phase 2: Optional services (attachments only) ──

	degraded := NewDegradedComponents()

	config.Attachment = attachments.NormalizeConfig(config.Attachment)
	optionalStages := []BootstrapStage{
		{
			Name: "attachments", Required: false,
			Init: func() error {
				store, err := attachments.NewStore(config.Attachment)
				if err != nil {
					return err
				}
				migrator := materials.NewAttachmentStoreMigrator(store, nil, config.Attachment.CloudflarePublicBaseURL, logger)
				container.AgentCoordinator.SetAttachmentMigrator(migrator)
				container.AgentCoordinator.SetAttachmentPersister(
					attachments.NewStorePersister(store),
				)
				return nil
			},
		},
	}

	if err := RunStages(optionalStages, degraded, logger); err != nil {
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
		{
			Name: "scheduler", Required: false,
			Init: func() error {
				if !config.Runtime.Proactive.Scheduler.Enabled {
					return nil
				}
				return subsystems.Start(context.Background(), &gatewaySubsystem{
					name: "scheduler",
					startFn: func(ctx context.Context) (func(), error) {
						sched := startScheduler(ctx, config, container, logger)
						if sched == nil {
							return nil, fmt.Errorf("scheduler init returned nil")
						}
						container.Drainables = append(container.Drainables, sched)
						return sched.Stop, nil
					},
				})
			},
		},
		{
			Name: "timer-manager", Required: false,
			Init: func() error {
				if !config.Runtime.Proactive.Timer.Enabled {
					return nil
				}
				return subsystems.Start(context.Background(), &gatewaySubsystem{
					name: "timer-manager",
					startFn: func(ctx context.Context) (func(), error) {
						mgr := startTimerManager(ctx, config, container, logger)
						if mgr == nil {
							return nil, fmt.Errorf("timer-manager init returned nil")
						}
						return mgr.Stop, nil
					},
				})
			},
		},
	}

	if err := RunStages(gatewayStages, degraded, logger); err != nil {
		return fmt.Errorf("gateway stages: %w", err)
	}

	if !degraded.IsEmpty() {
		logger.Warn("[Bootstrap] Lark standalone starting in degraded mode: %v", degraded.Map())
	}

	// ── Phase 3b: Card callback HTTP (optional) ──
	var cardCallbackServer *http.Server
	if container.LarkGateway != nil && config.Channels.Lark.CardsEnabled {
		cardHandler := larkdelivery.NewCardCallbackHandler(container.LarkGateway, logger)
		if cardHandler != nil {
			port := strings.TrimSpace(config.Channels.Lark.CardCallbackPort)
			if port == "" {
				port = "9292"
			}
			mux := http.NewServeMux()
			mux.Handle("POST /api/lark/card/callback", cardHandler)
			mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			cardCallbackServer = &http.Server{
				Addr:         ":" + port,
				Handler:      mux,
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  60 * time.Second,
			}
			go func() {
				logger.Info("Card callback HTTP server listening on :%s", port)
				if err := cardCallbackServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Warn("Card callback HTTP server error: %v", err)
				}
			}()
		}
	}
	defer func() {
		if cardCallbackServer != nil {
			shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = cardCallbackServer.Shutdown(shutCtx)
		}
	}()

	// ── Phase 4: Block until signal ──

	cardPortMsg := ""
	if cardCallbackServer != nil {
		cardPortMsg = fmt.Sprintf(", card callback on %s", cardCallbackServer.Addr)
	}
	logger.Info("Lark standalone mode running (WS + card callback%s). Waiting for signal...", cardPortMsg)
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
