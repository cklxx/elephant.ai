package bootstrap

import (
	"net/http"
	"time"

	"alex/internal/app/di"
	"alex/internal/app/subscription"
	serverApp "alex/internal/delivery/server/app"
	serverHTTP "alex/internal/delivery/server/http"
	"alex/internal/infra/observability"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// BuildDebugHTTPServer creates a lightweight HTTP server for the Lark standalone
// binary. It exposes health, SSE, dev/debug, config, and hooks-bridge endpoints
// on cfg.DebugPort (default "9090") — no auth, no CORS, no rate limiting.
func BuildDebugHTTPServer(f *Foundation, broadcaster *serverApp.EventBroadcaster, container *di.Container, cfg Config) (*http.Server, error) {
	logger := logging.OrNop(f.Logger)

	// Health checker — mirrors the probes from RunServer minus MCP.
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
	if container != nil {
		if container.LarkGateway != nil {
			hooksBridge = buildHooksBridge(cfg, container, logger)
		}
		memoryEngine = container.MemoryEngine
	}

	router := serverHTTP.NewDebugRouter(serverHTTP.DebugRouterDeps{
		Broadcaster:            broadcaster,
		HealthChecker:          healthChecker,
		ConfigHandler:          configHandler,
		OnboardingStateHandler: onboardingStateHandler,
		Obs:                    f.Obs,
		MemoryEngine:           memoryEngine,
		HooksBridge:            hooksBridge,
	})

	port := cfg.DebugPort
	if port == "" {
		port = "9090"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  5 * time.Minute,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	logger.Info("Debug HTTP server configured on :%s", port)
	return server, nil
}

// buildDebugBroadcaster creates an in-memory-only EventBroadcaster for Lark
// standalone mode. No Postgres history store — debug mode doesn't need
// persistent event history.
func buildDebugBroadcaster(obs *observability.Observability) *serverApp.EventBroadcaster {
	_ = obs // reserved for future metrics wiring
	return serverApp.NewEventBroadcaster(
		serverApp.WithMaxHistory(500),
		serverApp.WithMaxSessions(50),
		serverApp.WithSessionTTL(1*time.Hour),
	)
}
