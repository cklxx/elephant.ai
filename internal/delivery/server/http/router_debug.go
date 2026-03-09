package http

import (
	"net/http"
	"net/http/pprof"

	"alex/internal/delivery/server/app"
	"alex/internal/infra/observability"
	"alex/internal/shared/logging"
	promclient "github.com/prometheus/client_golang/prometheus/promhttp"
)

// DebugRouterDeps holds the minimal dependencies for the debug-only HTTP router.
// This router is embedded into the Lark standalone binary and exposes diagnostics,
// SSE event streaming, and runtime config endpoints on a separate port.
type DebugRouterDeps struct {
	Broadcaster            *app.EventBroadcaster
	RunTracker             app.RunTracker // may be nil
	HealthChecker          *app.HealthCheckerImpl
	ConfigHandler          *ConfigHandler
	OnboardingStateHandler *OnboardingStateHandler
	Obs                    *observability.Observability
	Environment            string
	AllowedOrigins         []string
	MemoryEngine           MemoryEngine      // may be nil
	HooksBridge            http.Handler      // may be nil
	LarkInjectGateway      LarkInjectGateway // may be nil; set when Lark gateway is available
	LarkOAuthHandler       *LarkOAuthHandler // may be nil
	RuntimeHooksBridge     http.Handler      // may be nil; POST /api/hooks/runtime
	RuntimeAPI             http.Handler      // may be nil; POST+GET /api/runtime/sessions
	RuntimePoolAPI         http.Handler      // may be nil; POST+GET /api/runtime/pool
	StartupProfileHandler  http.Handler      // may be nil; GET /api/health/startup-profile
}

// NewDebugRouter creates an HTTP handler exposing only debug / diagnostic
// endpoints.  It is designed for the embedded debug HTTP server in Lark
// standalone mode (typically on :9090).
//
// Excluded from this router:
//   - Auth (register/login/logout/refresh/me/plans/points/subscription)
//   - Tasks (create/list/get/cancel)
//   - Evaluations
//   - Sessions (CRUD, replay, share, fork)
//   - Agents catalog
//   - Attachments / data cache / sandbox browser
//   - Rate-limit / Stream-guard / Request-timeout middleware
func NewDebugRouter(deps DebugRouterDeps) http.Handler {
	logger := logging.NewComponentLogger("DebugRouter")
	latencyLogger := logging.NewLatencyLogger("DebugHTTP")

	// SSE handler — core for observing Lark task events.
	sseHandler := NewSSEHandler(
		deps.Broadcaster,
		WithSSEObservability(deps.Obs),
		WithSSERunTracker(deps.RunTracker),
	)

	// API handler — services are nil in debug mode (task endpoints disabled).
	apiHandler := NewAPIHandler(
		nil, // no TaskExecutionService
		nil, // no SessionService
		nil, // no SnapshotService
		deps.HealthChecker,
		true, // internalMode
		WithAPIObservability(deps.Obs),
		WithDevMode(true),
		WithMemoryEngine(deps.MemoryEngine),
	)

	mux := http.NewServeMux()

	// ── Health ──
	mux.Handle("GET /health", routeHandler("/health", http.HandlerFunc(apiHandler.HandleHealthCheck)))
	if deps.StartupProfileHandler != nil {
		mux.Handle("GET /api/health/startup-profile", routeHandler("/api/health/startup-profile", deps.StartupProfileHandler))
	}

	// ── SSE event stream ──
	mux.Handle("GET /api/sse", routeHandler("/api/sse", http.HandlerFunc(sseHandler.HandleSSEStream)))

	// ── Dev / debug endpoints ──
	mux.Handle("GET /api/dev/logs", routeHandler("/api/dev/logs", http.HandlerFunc(apiHandler.HandleDevLogTrace)))
	mux.Handle("GET /api/dev/logs/structured", routeHandler("/api/dev/logs/structured", http.HandlerFunc(apiHandler.HandleDevLogStructured)))
	mux.Handle("GET /api/dev/logs/index", routeHandler("/api/dev/logs/index", http.HandlerFunc(apiHandler.HandleDevLogIndex)))
	mux.Handle("GET /api/dev/memory", routeHandler("/api/dev/memory", http.HandlerFunc(apiHandler.HandleGetMemorySnapshot)))
	mux.Handle("/debug/pprof/", routeHandler("/debug/pprof", http.HandlerFunc(pprof.Index)))
	mux.Handle("GET /debug/pprof/cmdline", routeHandler("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline)))
	mux.Handle("GET /debug/pprof/profile", routeHandler("/debug/pprof/profile", http.HandlerFunc(pprof.Profile)))
	mux.Handle("GET /debug/pprof/symbol", routeHandler("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol)))
	mux.Handle("POST /debug/pprof/symbol", routeHandler("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol)))
	mux.Handle("GET /debug/pprof/trace", routeHandler("/debug/pprof/trace", http.HandlerFunc(pprof.Trace)))
	mux.Handle("GET /metrics", routeHandler("/metrics", promclient.Handler()))

	contextConfigHandler := NewContextConfigHandler("")
	if contextConfigHandler != nil {
		mux.Handle("GET /api/dev/context-config", routeHandler("/api/dev/context-config", http.HandlerFunc(contextConfigHandler.HandleGetContextConfig)))
		mux.Handle("PUT /api/dev/context-config", routeHandler("/api/dev/context-config", http.HandlerFunc(contextConfigHandler.HandleUpdateContextConfig)))
		mux.Handle("GET /api/dev/context-config/preview", routeHandler("/api/dev/context-config/preview", http.HandlerFunc(contextConfigHandler.HandleContextPreview)))
	}

	// context-window: returns 503 when coordinator is nil (debug mode).
	mux.Handle("GET /api/dev/sessions/{session_id}/context-window", routeHandler("/api/dev/sessions/:session_id/context-window", http.HandlerFunc(apiHandler.HandleGetContextWindowPreview)))

	// ── Internal config / onboarding / subscription ──
	mux.Handle("GET /api/internal/sessions/{session_id}/context", routeHandler("/api/internal/sessions/:session_id/context", http.HandlerFunc(apiHandler.HandleGetContextSnapshots)))

	if deps.ConfigHandler != nil {
		mux.Handle("GET /api/internal/config/runtime", routeHandler("/api/internal/config/runtime", http.HandlerFunc(deps.ConfigHandler.HandleGetRuntimeConfig)))
		mux.Handle("PUT /api/internal/config/runtime", routeHandler("/api/internal/config/runtime", http.HandlerFunc(deps.ConfigHandler.HandleUpdateRuntimeConfig)))
		mux.Handle("GET /api/internal/config/runtime/stream", routeHandler("/api/internal/config/runtime/stream", http.HandlerFunc(deps.ConfigHandler.HandleRuntimeStream)))
		mux.Handle("GET /api/internal/config/runtime/models", routeHandler("/api/internal/config/runtime/models", http.HandlerFunc(deps.ConfigHandler.HandleGetRuntimeModels)))
		mux.Handle("GET /api/internal/subscription/catalog", routeHandler("/api/internal/subscription/catalog", http.HandlerFunc(deps.ConfigHandler.HandleGetSubscriptionCatalog)))
	}
	if deps.OnboardingStateHandler != nil {
		mux.Handle("GET /api/internal/onboarding/state", routeHandler("/api/internal/onboarding/state", http.HandlerFunc(deps.OnboardingStateHandler.HandleGetOnboardingState)))
		mux.Handle("PUT /api/internal/onboarding/state", routeHandler("/api/internal/onboarding/state", http.HandlerFunc(deps.OnboardingStateHandler.HandleUpdateOnboardingState)))
	}

	// ── Claude Code hooks bridge ──
	if deps.HooksBridge != nil {
		mux.Handle("POST /api/hooks/claude-code", routeHandler("/api/hooks/claude-code", deps.HooksBridge))
	}

	// ── Runtime hooks bridge (Kaku runtime lifecycle events) ──
	if deps.RuntimeHooksBridge != nil {
		mux.Handle("POST /api/hooks/runtime", routeHandler("/api/hooks/runtime", deps.RuntimeHooksBridge))
	}

	// ── Runtime session management ──
	if deps.RuntimeAPI != nil {
		mux.Handle("POST /api/runtime/sessions", routeHandler("/api/runtime/sessions", deps.RuntimeAPI))
		mux.Handle("GET /api/runtime/sessions", routeHandler("/api/runtime/sessions", deps.RuntimeAPI))
		mux.Handle("GET /api/runtime/sessions/{id}", routeHandler("/api/runtime/sessions/:id", deps.RuntimeAPI))
	}

	// ── Runtime pane pool ──
	if deps.RuntimePoolAPI != nil {
		mux.Handle("POST /api/runtime/pool", routeHandler("/api/runtime/pool", deps.RuntimePoolAPI))
		mux.Handle("GET /api/runtime/pool", routeHandler("/api/runtime/pool", deps.RuntimePoolAPI))
	}

	// ── Lark OAuth ──
	if deps.LarkOAuthHandler != nil {
		mux.Handle("GET /api/lark/oauth/start", routeHandler("/api/lark/oauth/start", http.HandlerFunc(deps.LarkOAuthHandler.HandleStart)))
		mux.Handle("GET /api/lark/oauth/callback", routeHandler("/api/lark/oauth/callback", http.HandlerFunc(deps.LarkOAuthHandler.HandleCallback)))
	}

	// ── Lark inject (local e2e testing) ──
	if deps.LarkInjectGateway != nil {
		injectHandler := NewLarkInjectHandler(deps.LarkInjectGateway)
		mux.Handle("POST /api/dev/inject", routeHandler("/api/dev/inject", http.HandlerFunc(injectHandler.Handle)))
	}

	// ── Minimal middleware stack (CORS only, no rate-limit / auth) ──
	var handler http.Handler = mux
	handler = CORSMiddleware(deps.Environment, deps.AllowedOrigins)(handler)
	handler = ObservabilityMiddleware(deps.Obs, latencyLogger)(handler)
	handler = LoggingMiddleware(logger)(handler)
	handler = CompressionMiddleware()(handler)

	return handler
}
