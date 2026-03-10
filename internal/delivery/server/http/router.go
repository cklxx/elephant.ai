package http

import (
	"net/http"
	"strings"
	"time"

	"alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// NewRouter creates a new HTTP router with all endpoints.
// Routes use Go 1.22+ method-specific patterns ("METHOD /path/{param}").
func NewRouter(deps RouterDeps, cfg RouterConfig) http.Handler {
	logger := logging.NewComponentLogger("Router")
	latencyLogger := logging.NewLatencyLogger("HTTP")
	var attachmentStore *AttachmentStore
	if store, err := NewAttachmentStore(deps.AttachmentCfg); err != nil {
		logger.Warn("Attachment store disabled: %v", err)
	} else {
		attachmentStore = store
	}
	taskBodyLimit := cfg.MaxTaskBodyBytes
	if taskBodyLimit <= 0 {
		taskBodyLimit = defaultMaxCreateTaskBodySize
	}
	normalizedEnv := strings.TrimSpace(cfg.Environment)

	dataCache := deps.DataCache
	if dataCache == nil {
		dataCache = NewDataCache(512, 30*time.Minute)
	}

	// Create handlers
	sseHandler := NewSSEHandler(
		deps.Broadcaster,
		WithSSEObservability(deps.Obs),
		WithSSEAttachmentStore(attachmentStore),
		WithSSEDataCache(dataCache),
		WithSSERunTracker(deps.RunTracker),
	)
	shareHandler := NewShareHandler(deps.Sessions, sseHandler)
	internalMode := strings.EqualFold(normalizedEnv, "internal") || strings.EqualFold(normalizedEnv, "evaluation")
	devMode := strings.EqualFold(normalizedEnv, "development") || strings.EqualFold(normalizedEnv, "dev")
	apiHandler := NewAPIHandler(
		deps.Tasks,
		deps.Sessions,
		deps.Snapshots,
		deps.HealthChecker,
		internalMode,
		WithAPIObservability(deps.Obs),
		WithEvaluationService(deps.Evaluation),
		WithAttachmentStore(attachmentStore),
		WithDevMode(devMode),
		WithMaxCreateTaskBodySize(taskBodyLimit),
		WithMemoryEngine(deps.MemoryEngine),
	)

	// Create mux using Go 1.22+ method-specific patterns.
	mux := http.NewServeMux()

	// ── Internal / dev endpoints ──

	registerHandler(mux, "GET /api/internal/sessions/{session_id}/context", "/api/internal/sessions/:session_id/context", apiHandler.HandleGetContextSnapshots)

	if devMode {
		registerHandler(mux, "GET /api/dev/sessions/{session_id}/context-window", "/api/dev/sessions/:session_id/context-window", apiHandler.HandleGetContextWindowPreview)
		// Keep local logs-ui usable without account login in development mode.
		registerHandler(mux, "GET /api/dev/logs", "/api/dev/logs", apiHandler.HandleDevLogTrace)
		registerHandler(mux, "GET /api/dev/logs/structured", "/api/dev/logs/structured", apiHandler.HandleDevLogStructured)
		registerHandler(mux, "GET /api/dev/logs/index", "/api/dev/logs/index", apiHandler.HandleDevLogIndex)
		registerHandler(mux, "GET /api/dev/memory", "/api/dev/memory", apiHandler.HandleGetMemorySnapshot)
		registerContextConfigRoutes(mux, NewContextConfigHandler(""))
	}

	if internalMode || devMode {
		registerRuntimeConfigRoutes(mux, deps.ConfigHandler)
	}
	if internalMode || devMode {
		registerOnboardingStateRoutes(mux, deps.OnboardingStateHandler)
	}
	if internalMode {
		appsConfigHandler := NewAppsConfigHandler(config.LoadAppsConfig, config.SaveAppsConfig)
		if appsConfigHandler != nil {
			registerHandler(mux, "GET /api/internal/config/apps", "/api/internal/config/apps", appsConfigHandler.HandleGetAppsConfig)
			registerHandler(mux, "PUT /api/internal/config/apps", "/api/internal/config/apps", appsConfigHandler.HandleUpdateAppsConfig)
		}
	}

	// ── SSE / streaming ──

	registerLarkOAuthRoutes(mux, deps.LarkOAuthHandler)
	registerHandler(mux, "GET /api/sse", "/api/sse", sseHandler.HandleSSEStream)
	registerHandler(mux, "GET /api/share/sessions/{session_id}", "/api/share/sessions/:session_id", shareHandler.HandleSharedSession)
	if attachmentStore != nil {
		registerRoute(mux, "/api/attachments/", "/api/attachments", attachmentStore.Handler())
	}
	if dataCache != nil {
		registerRoute(mux, "/api/data/", "/api/data", dataCache.Handler())
	}
	registerHandler(mux, "POST /api/metrics/web-vitals", "/api/metrics/web-vitals", apiHandler.HandleWebVitals)

	// ── Task endpoints ──

	registerTaskRoutes(mux, apiHandler, sseHandler)

	// ── Evaluation endpoints ──

	registerEvaluationRoutes(mux, apiHandler)

	// ── Session endpoints ──

	registerSessionRoutes(mux, apiHandler)

	// ── Leader dashboard ──

	registerLeaderRoutes(mux, deps.LeaderDashboard, cfg.LeaderAPIToken)

	// ── Claude Code hooks bridge ──

	registerHookRoutes(mux, deps.HooksBridge, deps.RuntimeHooksBridge)

	// ── Health check ──

	registerHandler(mux, "GET /health", "/health", apiHandler.HandleHealthCheck)

	// ── Middleware stack ──

	var handler http.Handler = mux
	handler = ObservabilityMiddleware(deps.Obs, latencyLogger)(handler)
	handler = LoggingMiddleware(logger)(handler)
	handler = RateLimitMiddleware(cfg.RateLimit)(handler)
	handler = RequestTimeoutMiddleware(cfg.NonStreamTimeout)(handler)
	handler = StreamGuardMiddleware(cfg.StreamGuard)(handler)
	handler = CompressionMiddleware()(handler)
	handler = CORSMiddleware(cfg.Environment, cfg.AllowedOrigins)(handler)

	return handler
}

func routeHandler(route string, handler http.Handler) http.Handler {
	if route == "" {
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		annotateRequestRoute(r, route)
		handler.ServeHTTP(w, r)
	})
}
