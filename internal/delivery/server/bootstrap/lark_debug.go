package bootstrap

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"alex/internal/app/di"
	"alex/internal/app/lifecycle"
	"alex/internal/app/subscription"
	serverApp "alex/internal/delivery/server/app"
	serverHTTP "alex/internal/delivery/server/http"
	"alex/internal/infra/observability"
	"alex/internal/runtime/hooks"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// BuildDebugHTTPServer creates a lightweight HTTP server for the Lark standalone
// binary. It exposes health, SSE, dev/debug, config, hooks-bridge, and runtime
// endpoints on cfg.DebugPort (default "9090") — no auth, no rate limiting.
// It returns the server and the runtime event bus so the caller can wire
// StallDetector, LeaderAgent, and other bus consumers.
func BuildDebugHTTPServer(f *Foundation, broadcaster *serverApp.EventBroadcaster, container *di.Container, cfg Config) (*http.Server, hooks.Bus, error) {
	logger := logging.OrNop(f.Logger)
	ctx := context.Background()

	// Health checker — mirrors the probes from RunServer.
	healthChecker := serverApp.NewHealthChecker()
	if container != nil {
		healthChecker.RegisterProbe(serverApp.NewLLMFactoryProbe(container))
	}
	healthChecker.RegisterProbe(serverApp.NewDegradedProbe(f.Degraded))

	// Config handler for runtime config inspection/mutation.
	runtimeUpdates, runtimeReloader := f.RuntimeCacheUpdates()
	configHandler := serverHTTP.NewConfigHandler(f.ConfigManager(), f.Resolver(), runtimeUpdates, runtimeReloader)

	// Onboarding state handler.
	onboardingStore := subscription.NewOnboardingStateStore(
		subscription.ResolveOnboardingStatePath(runtimeconfig.DefaultEnvLookup, nil),
	)
	onboardingStateHandler := serverHTTP.NewOnboardingStateHandler(onboardingStore)

	// Hooks bridge — forward Claude Code hook events to Lark gateway.
	var hooksBridge http.Handler
	var memoryEngine serverHTTP.MemoryEngine
	var larkInjectGateway serverHTTP.LarkInjectGateway
	var larkOAuthHandler *serverHTTP.LarkOAuthHandler
	if container != nil {
		if container.LarkGateway != nil {
			if hb := buildHooksBridge(cfg, container, logger); hb != nil {
				hooksBridge = hb
				container.Drainables = append(container.Drainables,
					lifecycle.DrainFunc{DrainName: "hooks-bridge", Fn: func(ctx context.Context) { hb.Close(ctx) }},
				)
			}
			larkInjectGateway = container.LarkGateway
		}
		if container.LarkOAuth != nil {
			larkOAuthHandler = serverHTTP.NewLarkOAuthHandler(container.LarkOAuth, logger)
		}
		memoryEngine = container.MemoryEngine
	}

	// Runtime hooks bridge — translates CC hook events into runtime bus events.
	runtimeHooksHandler, runtimeBus := buildRuntimeHooksHandler(logger)
	startRuntimeBusLogger(ctx, runtimeBus, logger)
	if container != nil && container.LarkGateway != nil {
		startRuntimeCompletionNotifier(ctx, runtimeBus, container.LarkGateway, cfg.HooksBridge.DefaultChatID, logger)
	}

	// Runtime subsystem: Runtime + StallDetector + LeaderAgent.
	// The returned *Runtime is wrapped as an HTTP handler for session management.
	var runtimeAPI http.Handler
	if rt := startRuntimeSubsystem(ctx, runtimeBus, container, logger); rt != nil {
		runtimeAPI = NewRuntimeSessionHandler(rt, logger)
	}

	router := serverHTTP.NewDebugRouter(serverHTTP.DebugRouterDeps{
		Broadcaster:            broadcaster,
		HealthChecker:          healthChecker,
		ConfigHandler:          configHandler,
		OnboardingStateHandler: onboardingStateHandler,
		Obs:                    f.Obs,
		Environment:            cfg.Runtime.Environment,
		AllowedOrigins:         append([]string(nil), cfg.AllowedOrigins...),
		MemoryEngine:           memoryEngine,
		HooksBridge:            hooksBridge,
		LarkInjectGateway:      larkInjectGateway,
		LarkOAuthHandler:       larkOAuthHandler,
		RuntimeHooksBridge:     runtimeHooksHandler,
		RuntimeAPI:             runtimeAPI,
	})

	port := cfg.DebugPort
	if port == "" {
		port = "9090"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 10 * time.Minute, // Long timeout for inject endpoint (blocks until task completes)
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("Debug HTTP server configured on :%s", port)
	return server, runtimeBus, nil
}

// buildDebugBroadcaster creates the EventBroadcaster for Lark standalone mode.
// It keeps an in-memory window and also persists event history to local files
// (when sessionDir is provided) so diagnostics can replay timing for recent runs.
func buildDebugBroadcaster(obs *observability.Observability, sessionDir string, logger logging.Logger) *serverApp.EventBroadcaster {
	_ = obs // reserved for future metrics wiring
	opts := []serverApp.EventBroadcasterOption{
		serverApp.WithMaxHistory(500),
		serverApp.WithMaxSessions(50),
		serverApp.WithSessionTTL(1 * time.Hour),
	}
	if sessionDir != "" {
		eventsDir := filepath.Join(sessionDir, "_server")
		fileHistory := serverApp.NewFileEventHistoryStore(eventsDir)
		if err := fileHistory.EnsureSchema(context.Background()); err != nil {
			logging.OrNop(logger).Warn("Lark debug event history disabled: %v", err)
		} else {
			opts = append(opts, serverApp.WithEventHistoryStore(fileHistory))
		}
	}
	return serverApp.NewEventBroadcaster(opts...)
}
