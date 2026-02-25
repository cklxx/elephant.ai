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

	// Identity function — auth middleware removed.
	wrap := func(handler http.Handler) http.Handler { return handler }

	// Create mux using Go 1.22+ method-specific patterns.
	mux := http.NewServeMux()

	// ── Internal / dev endpoints ──

	mux.Handle("GET /api/internal/sessions/{session_id}/context", routeHandler("/api/internal/sessions/:session_id/context", http.HandlerFunc(apiHandler.HandleGetContextSnapshots)))

	if devMode {
		mux.Handle("GET /api/dev/sessions/{session_id}/context-window", routeHandler("/api/dev/sessions/:session_id/context-window", wrap(http.HandlerFunc(apiHandler.HandleGetContextWindowPreview))))
		// Keep local logs-ui usable without account login in development mode.
		mux.Handle("GET /api/dev/logs", routeHandler("/api/dev/logs", http.HandlerFunc(apiHandler.HandleDevLogTrace)))
		mux.Handle("GET /api/dev/logs/structured", routeHandler("/api/dev/logs/structured", http.HandlerFunc(apiHandler.HandleDevLogStructured)))
		mux.Handle("GET /api/dev/logs/index", routeHandler("/api/dev/logs/index", http.HandlerFunc(apiHandler.HandleDevLogIndex)))
		mux.Handle("GET /api/dev/memory", routeHandler("/api/dev/memory", wrap(http.HandlerFunc(apiHandler.HandleGetMemorySnapshot))))

		contextConfigHandler := NewContextConfigHandler("")
		if contextConfigHandler != nil {
			mux.Handle("GET /api/dev/context-config", routeHandler("/api/dev/context-config", wrap(http.HandlerFunc(contextConfigHandler.HandleGetContextConfig))))
			mux.Handle("PUT /api/dev/context-config", routeHandler("/api/dev/context-config", wrap(http.HandlerFunc(contextConfigHandler.HandleUpdateContextConfig))))
			mux.Handle("GET /api/dev/context-config/preview", routeHandler("/api/dev/context-config/preview", wrap(http.HandlerFunc(contextConfigHandler.HandleContextPreview))))
		}
	}

	if (internalMode || devMode) && deps.ConfigHandler != nil {
		mux.Handle("GET /api/internal/config/runtime", routeHandler("/api/internal/config/runtime", wrap(http.HandlerFunc(deps.ConfigHandler.HandleGetRuntimeConfig))))
		mux.Handle("PUT /api/internal/config/runtime", routeHandler("/api/internal/config/runtime", wrap(http.HandlerFunc(deps.ConfigHandler.HandleUpdateRuntimeConfig))))
		mux.Handle("GET /api/internal/config/runtime/stream", routeHandler("/api/internal/config/runtime/stream", wrap(http.HandlerFunc(deps.ConfigHandler.HandleRuntimeStream))))
		mux.Handle("GET /api/internal/config/runtime/models", routeHandler("/api/internal/config/runtime/models", wrap(http.HandlerFunc(deps.ConfigHandler.HandleGetRuntimeModels))))
		mux.Handle("GET /api/internal/subscription/catalog", routeHandler("/api/internal/subscription/catalog", wrap(http.HandlerFunc(deps.ConfigHandler.HandleGetSubscriptionCatalog))))
	}
	if (internalMode || devMode) && deps.OnboardingStateHandler != nil {
		mux.Handle("GET /api/internal/onboarding/state", routeHandler("/api/internal/onboarding/state", wrap(http.HandlerFunc(deps.OnboardingStateHandler.HandleGetOnboardingState))))
		mux.Handle("PUT /api/internal/onboarding/state", routeHandler("/api/internal/onboarding/state", wrap(http.HandlerFunc(deps.OnboardingStateHandler.HandleUpdateOnboardingState))))
	}
	if internalMode {
		appsConfigHandler := NewAppsConfigHandler(config.LoadAppsConfig, config.SaveAppsConfig)
		if appsConfigHandler != nil {
			mux.Handle("GET /api/internal/config/apps", routeHandler("/api/internal/config/apps", wrap(http.HandlerFunc(appsConfigHandler.HandleGetAppsConfig))))
			mux.Handle("PUT /api/internal/config/apps", routeHandler("/api/internal/config/apps", wrap(http.HandlerFunc(appsConfigHandler.HandleUpdateAppsConfig))))
		}
	}

	// ── SSE / streaming ──

	if deps.LarkOAuthHandler != nil {
		mux.Handle("GET /api/lark/oauth/start", routeHandler("/api/lark/oauth/start", http.HandlerFunc(deps.LarkOAuthHandler.HandleStart)))
		mux.Handle("GET /api/lark/oauth/callback", routeHandler("/api/lark/oauth/callback", http.HandlerFunc(deps.LarkOAuthHandler.HandleCallback)))
	}

	mux.Handle("GET /api/sse", routeHandler("/api/sse", wrap(http.HandlerFunc(sseHandler.HandleSSEStream))))
	mux.Handle("GET /api/share/sessions/{session_id}", routeHandler("/api/share/sessions/:session_id", http.HandlerFunc(shareHandler.HandleSharedSession)))
	if attachmentStore != nil {
		mux.Handle("/api/attachments/", routeHandler("/api/attachments", attachmentStore.Handler()))
	}
	if dataCache != nil {
		mux.Handle("/api/data/", routeHandler("/api/data", dataCache.Handler()))
	}
	mux.Handle("POST /api/metrics/web-vitals", routeHandler("/api/metrics/web-vitals", http.HandlerFunc(apiHandler.HandleWebVitals)))

	// ── Task endpoints ──

	mux.Handle("POST /api/tasks", routeHandler("/api/tasks", wrap(http.HandlerFunc(apiHandler.HandleCreateTask))))
	mux.Handle("GET /api/tasks", routeHandler("/api/tasks", wrap(http.HandlerFunc(apiHandler.HandleListTasks))))
	mux.Handle("GET /api/tasks/active", routeHandler("/api/tasks/active", wrap(http.HandlerFunc(apiHandler.HandleListActiveTasks))))
	mux.Handle("GET /api/tasks/stats", routeHandler("/api/tasks/stats", wrap(http.HandlerFunc(apiHandler.HandleGetTaskStats))))
	mux.Handle("GET /api/tasks/{task_id}", routeHandler("/api/tasks/:task_id", wrap(http.HandlerFunc(apiHandler.HandleGetTask))))
	mux.Handle("GET /api/tasks/{task_id}/events", routeHandler("/api/tasks/:task_id/events", wrap(http.HandlerFunc(sseHandler.HandleTaskSSEStream))))
	mux.Handle("POST /api/tasks/{task_id}/cancel", routeHandler("/api/tasks/:task_id/cancel", wrap(http.HandlerFunc(apiHandler.HandleCancelTask))))

	// ── Evaluation endpoints ──

	mux.Handle("GET /api/evaluations", routeHandler("/api/evaluations", wrap(http.HandlerFunc(apiHandler.HandleListEvaluations))))
	mux.Handle("POST /api/evaluations", routeHandler("/api/evaluations", wrap(http.HandlerFunc(apiHandler.HandleStartEvaluation))))
	mux.Handle("GET /api/evaluations/{evaluation_id}", routeHandler("/api/evaluations/:evaluation_id", wrap(http.HandlerFunc(apiHandler.HandleGetEvaluation))))
	mux.Handle("DELETE /api/evaluations/{evaluation_id}", routeHandler("/api/evaluations/:evaluation_id", wrap(http.HandlerFunc(apiHandler.HandleDeleteEvaluation))))

	// ── Agent catalog endpoints ──

	mux.Handle("GET /api/agents", routeHandler("/api/agents", wrap(http.HandlerFunc(apiHandler.HandleListAgents))))
	mux.Handle("GET /api/agents/{agent_id}", routeHandler("/api/agents/:agent_id", wrap(http.HandlerFunc(apiHandler.HandleGetAgent))))
	mux.Handle("GET /api/agents/{agent_id}/evaluations", routeHandler("/api/agents/:agent_id/evaluations", wrap(http.HandlerFunc(apiHandler.HandleListAgentEvaluations))))

	// ── Session endpoints ──

	mux.Handle("GET /api/sessions", routeHandler("/api/sessions", wrap(http.HandlerFunc(apiHandler.HandleListSessions))))
	mux.Handle("POST /api/sessions", routeHandler("/api/sessions", wrap(http.HandlerFunc(apiHandler.HandleCreateSession))))
	mux.Handle("GET /api/sessions/{session_id}", routeHandler("/api/sessions/:session_id", wrap(http.HandlerFunc(apiHandler.HandleGetSession))))
	mux.Handle("DELETE /api/sessions/{session_id}", routeHandler("/api/sessions/:session_id", wrap(http.HandlerFunc(apiHandler.HandleDeleteSession))))
	mux.Handle("GET /api/sessions/{session_id}/persona", routeHandler("/api/sessions/:session_id/persona", wrap(http.HandlerFunc(apiHandler.HandleGetSessionPersona))))
	mux.Handle("PUT /api/sessions/{session_id}/persona", routeHandler("/api/sessions/:session_id/persona", wrap(http.HandlerFunc(apiHandler.HandleUpdateSessionPersona))))
	mux.Handle("GET /api/sessions/{session_id}/snapshots", routeHandler("/api/sessions/:session_id/snapshots", wrap(http.HandlerFunc(apiHandler.HandleListSnapshots))))
	mux.Handle("GET /api/sessions/{session_id}/turns/{turn_id}", routeHandler("/api/sessions/:session_id/turns/:turn_id", wrap(http.HandlerFunc(apiHandler.HandleGetTurnSnapshot))))
	mux.Handle("POST /api/sessions/{session_id}/replay", routeHandler("/api/sessions/:session_id/replay", wrap(http.HandlerFunc(apiHandler.HandleReplaySession))))
	mux.Handle("POST /api/sessions/{session_id}/share", routeHandler("/api/sessions/:session_id/share", wrap(http.HandlerFunc(apiHandler.HandleCreateSessionShare))))
	mux.Handle("POST /api/sessions/{session_id}/fork", routeHandler("/api/sessions/:session_id/fork", wrap(http.HandlerFunc(apiHandler.HandleForkSession))))

	// ── Claude Code hooks bridge ──

	if deps.HooksBridge != nil {
		mux.Handle("POST /api/hooks/claude-code", routeHandler("/api/hooks/claude-code", deps.HooksBridge))
	}

	// ── Health check ──

	mux.Handle("GET /health", routeHandler("/health", http.HandlerFunc(apiHandler.HandleHealthCheck)))

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
