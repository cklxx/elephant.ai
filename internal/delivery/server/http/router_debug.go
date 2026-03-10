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
	registerHandler(mux, "GET /health", "/health", apiHandler.HandleHealthCheck)
	registerHandler(mux, "GET /api/debug/health/models", "/api/debug/health/models", apiHandler.HandleModelHealthDebug)
	if deps.StartupProfileHandler != nil {
		registerRoute(mux, "GET /api/health/startup-profile", "/api/health/startup-profile", deps.StartupProfileHandler)
	}

	// ── SSE event stream ──
	registerHandler(mux, "GET /api/sse", "/api/sse", sseHandler.HandleSSEStream)

	// ── Dev / debug endpoints ──
	registerHandler(mux, "GET /api/dev/logs", "/api/dev/logs", apiHandler.HandleDevLogTrace)
	registerHandler(mux, "GET /api/dev/logs/structured", "/api/dev/logs/structured", apiHandler.HandleDevLogStructured)
	registerHandler(mux, "GET /api/dev/logs/index", "/api/dev/logs/index", apiHandler.HandleDevLogIndex)
	registerHandler(mux, "GET /api/dev/memory", "/api/dev/memory", apiHandler.HandleGetMemorySnapshot)
	registerRoute(mux, "/debug/pprof/", "/debug/pprof", http.HandlerFunc(pprof.Index))
	registerHandler(mux, "GET /debug/pprof/cmdline", "/debug/pprof/cmdline", pprof.Cmdline)
	registerHandler(mux, "GET /debug/pprof/profile", "/debug/pprof/profile", pprof.Profile)
	registerHandler(mux, "GET /debug/pprof/symbol", "/debug/pprof/symbol", pprof.Symbol)
	registerHandler(mux, "POST /debug/pprof/symbol", "/debug/pprof/symbol", pprof.Symbol)
	registerHandler(mux, "GET /debug/pprof/trace", "/debug/pprof/trace", pprof.Trace)
	registerRoute(mux, "GET /metrics", "/metrics", promclient.Handler())
	registerContextConfigRoutes(mux, NewContextConfigHandler(""))

	// context-window: returns 503 when coordinator is nil (debug mode).
	registerHandler(mux, "GET /api/dev/sessions/{session_id}/context-window", "/api/dev/sessions/:session_id/context-window", apiHandler.HandleGetContextWindowPreview)

	// ── Internal config / onboarding / subscription ──
	registerHandler(mux, "GET /api/internal/sessions/{session_id}/context", "/api/internal/sessions/:session_id/context", apiHandler.HandleGetContextSnapshots)

	registerRuntimeConfigRoutes(mux, deps.ConfigHandler)
	registerOnboardingStateRoutes(mux, deps.OnboardingStateHandler)

	// ── Claude Code hooks bridge ──
	registerHookRoutes(mux, deps.HooksBridge, deps.RuntimeHooksBridge)

	// ── Runtime session management ──
	if deps.RuntimeAPI != nil {
		registerRoute(mux, "POST /api/runtime/sessions", "/api/runtime/sessions", deps.RuntimeAPI)
		registerRoute(mux, "GET /api/runtime/sessions", "/api/runtime/sessions", deps.RuntimeAPI)
		registerRoute(mux, "GET /api/runtime/sessions/{id}", "/api/runtime/sessions/:id", deps.RuntimeAPI)
	}

	// ── Runtime pane pool ──
	if deps.RuntimePoolAPI != nil {
		registerRoute(mux, "POST /api/runtime/pool", "/api/runtime/pool", deps.RuntimePoolAPI)
		registerRoute(mux, "GET /api/runtime/pool", "/api/runtime/pool", deps.RuntimePoolAPI)
	}

	// ── Lark OAuth ──
	registerLarkOAuthRoutes(mux, deps.LarkOAuthHandler)

	// ── Lark inject (local e2e testing) ──
	if deps.LarkInjectGateway != nil {
		injectHandler := NewLarkInjectHandler(deps.LarkInjectGateway)
		registerHandler(mux, "POST /api/dev/inject", "/api/dev/inject", injectHandler.Handle)
	}

	// ── Minimal middleware stack (CORS only, no rate-limit / auth) ──
	var handler http.Handler = mux
	handler = CORSMiddleware(deps.Environment, deps.AllowedOrigins)(handler)
	handler = ObservabilityMiddleware(deps.Obs, latencyLogger)(handler)
	handler = LoggingMiddleware(logger)(handler)
	handler = CompressionMiddleware()(handler)

	return handler
}
