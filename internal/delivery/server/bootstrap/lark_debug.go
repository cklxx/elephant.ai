package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"
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

// debugServerOption configures optional components of the debug HTTP server.
type debugServerOption func(*serverHTTP.DebugRouterDeps)

// withStartupProfile attaches a startup-profile HTTP handler to the debug router.
func withStartupProfile(sp *startupProfile) debugServerOption {
	return func(deps *serverHTTP.DebugRouterDeps) {
		deps.StartupProfileHandler = &startupProfileHandler{profile: sp}
	}
}

// BuildDebugHTTPServer creates a lightweight HTTP server for the Lark standalone
// binary. It exposes health, SSE, dev/debug, config, hooks-bridge, and runtime
// endpoints on cfg.DebugPort (default "9090") — no auth, no rate limiting.
// It returns the server and the runtime event bus so the caller can wire
// StallDetector, LeaderAgent, and other bus consumers.
func BuildDebugHTTPServer(f *Foundation, broadcaster *serverApp.EventBroadcaster, container *di.Container, cfg Config, opts ...debugServerOption) (*http.Server, hooks.Bus, error) {
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
		startHandoffNotifier(ctx, runtimeBus, container.LarkGateway, cfg.HooksBridge.DefaultChatID, logger)
	}

	// Runtime subsystem: Runtime + StallDetector + LeaderAgent + PanePool.
	// The returned *Runtime is wrapped as HTTP handlers for session and pool management.
	var runtimeAPI http.Handler
	var runtimePoolAPI http.Handler
	if rt := startRuntimeSubsystem(ctx, runtimeBus, container, logger); rt != nil {
		runtimeAPI = NewRuntimeSessionHandler(rt, logger)
		runtimePoolAPI = NewRuntimePoolHandler(rt, logger)
	}

	deps := serverHTTP.DebugRouterDeps{
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
		RuntimePoolAPI:         runtimePoolAPI,
	}
	for _, o := range opts {
		o(&deps)
	}
	router := serverHTTP.NewDebugRouter(deps)

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

const debugPortMaxRetries = 5

// listenDebugPort tries to bind a TCP listener starting at basePort, falling
// back to basePort+1 … basePort+debugPortMaxRetries when the port is busy
// (e.g. a previous process still holds it). Returns nil, nil when all ports
// are unavailable — the caller should treat this as graceful degradation.
func listenDebugPort(basePort string, logger logging.Logger) (net.Listener, error) {
	port, err := strconv.Atoi(basePort)
	if err != nil {
		return nil, fmt.Errorf("invalid debug port %q: %w", basePort, err)
	}

	for i := 0; i <= debugPortMaxRetries; i++ {
		addr := fmt.Sprintf(":%d", port+i)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			if i > 0 {
				logger.Warn("Debug port :%d busy, using fallback %s", port, addr)
			}
			logger.Info("Debug HTTP server bound to %s", addr)
			return ln, nil
		}
		logger.Warn("Debug port %s unavailable: %v", addr, err)
	}

	logger.Warn("All debug ports :%d–:%d unavailable; debug HTTP server disabled", port, port+debugPortMaxRetries)
	return nil, nil
}

// startupProfile records per-phase timing from RunLark. It is created before
// Phase 1 and populated as each phase completes. The HTTP handler reads
// whatever data is available at request time.
type startupProfile struct {
	mu     sync.RWMutex
	phases []phaseTiming
	total  time.Duration
}

type phaseTiming struct {
	Name       string `json:"name"`
	DurationMS int64  `json:"duration_ms"`
}

func newStartupProfile() *startupProfile {
	return &startupProfile{}
}

func (sp *startupProfile) record(name string, d time.Duration) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.phases = append(sp.phases, phaseTiming{
		Name:       name,
		DurationMS: d.Milliseconds(),
	})
}

func (sp *startupProfile) finalize(total time.Duration) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.total = total
}

func (sp *startupProfile) snapshot() startupProfileResponse {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	phases := make([]phaseTiming, len(sp.phases))
	copy(phases, sp.phases)
	return startupProfileResponse{
		Phases:  phases,
		TotalMS: sp.total.Milliseconds(),
	}
}

type startupProfileResponse struct {
	Phases  []phaseTiming `json:"phases"`
	TotalMS int64         `json:"total_ms"`
}

// startupProfileHandler serves GET /api/health/startup-profile.
type startupProfileHandler struct {
	profile *startupProfile
}

func (h *startupProfileHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.profile.snapshot())
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
