package http

import (
	"net/http"

	"alex/internal/delivery/server/app"
	"alex/internal/infra/observability"
	"alex/internal/shared/logging"
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
	MemoryEngine           MemoryEngine     // may be nil
	HooksBridge            http.Handler     // may be nil
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
//   - CORS / Rate-limit / Stream-guard / Request-timeout middleware
//   - Lark OAuth
func NewDebugRouter(deps DebugRouterDeps) http.Handler {
	logger := logging.NewComponentLogger("DebugRouter")
	latencyLogger := logging.NewLatencyLogger("DebugHTTP")

	// SSE handler — core for observing Lark task events.
	sseHandler := NewSSEHandler(
		deps.Broadcaster,
		WithSSEObservability(deps.Obs),
		WithSSERunTracker(deps.RunTracker),
	)

	// API handler — coordinator is nil in debug mode (task endpoints disabled).
	apiHandler := NewAPIHandler(
		nil, // no ServerCoordinator
		deps.HealthChecker,
		true, // internalMode
		WithAPIObservability(deps.Obs),
		WithDevMode(true),
		WithMemoryEngine(deps.MemoryEngine),
	)

	mux := http.NewServeMux()

	// ── Health ──
	mux.Handle("GET /health", routeHandler("/health", http.HandlerFunc(apiHandler.HandleHealthCheck)))

	// ── SSE event stream ──
	mux.Handle("GET /api/sse", routeHandler("/api/sse", http.HandlerFunc(sseHandler.HandleSSEStream)))

	// ── Dev / debug endpoints ──
	mux.Handle("GET /api/dev/logs", routeHandler("/api/dev/logs", http.HandlerFunc(apiHandler.HandleDevLogTrace)))
	mux.Handle("GET /api/dev/logs/structured", routeHandler("/api/dev/logs/structured", http.HandlerFunc(apiHandler.HandleDevLogStructured)))
	mux.Handle("GET /api/dev/logs/index", routeHandler("/api/dev/logs/index", http.HandlerFunc(apiHandler.HandleDevLogIndex)))
	mux.Handle("GET /api/dev/memory", routeHandler("/api/dev/memory", http.HandlerFunc(apiHandler.HandleGetMemorySnapshot)))

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

	// ── Minimal middleware stack (no CORS / rate-limit / auth) ──
	var handler http.Handler = mux
	handler = ObservabilityMiddleware(deps.Obs, latencyLogger)(handler)
	handler = LoggingMiddleware(logger)(handler)
	handler = CompressionMiddleware()(handler)

	return handler
}
