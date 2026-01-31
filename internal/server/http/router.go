package http

import (
	"net/http"
	"strings"

	"alex/internal/attachments"
	"alex/internal/auth/domain"
	"alex/internal/config"
	"alex/internal/logging"
	"alex/internal/sandbox"
)

// NewRouter creates a new HTTP router with all endpoints.
// Routes use Go 1.22+ method-specific patterns ("METHOD /path/{param}").
func NewRouter(deps RouterDeps, cfg RouterConfig) http.Handler {
	logger := logging.NewComponentLogger("Router")
	latencyLogger := logging.NewLatencyLogger("HTTP")
	attachmentCfg := attachments.NormalizeConfig(deps.AttachmentCfg)
	var attachmentStore *AttachmentStore
	if store, err := NewAttachmentStore(attachmentCfg); err != nil {
		logger.Warn("Attachment store disabled: %v", err)
	} else {
		attachmentStore = store
	}
	taskBodyLimit := cfg.MaxTaskBodyBytes
	if taskBodyLimit <= 0 {
		taskBodyLimit = defaultMaxCreateTaskBodySize
	}
	normalizedEnv := strings.TrimSpace(cfg.Environment)

	// Create handlers
	sseHandler := NewSSEHandler(deps.Broadcaster, WithSSEObservability(deps.Obs), WithSSEAttachmentStore(attachmentStore), WithSSERunTracker(deps.RunTracker))
	shareHandler := NewShareHandler(deps.Coordinator, sseHandler)
	internalMode := strings.EqualFold(normalizedEnv, "internal") || strings.EqualFold(normalizedEnv, "evaluation")
	devMode := strings.EqualFold(normalizedEnv, "development") || strings.EqualFold(normalizedEnv, "dev")
	sandboxClient := sandbox.NewClient(sandbox.Config{BaseURL: deps.SandboxBaseURL})
	apiHandler := NewAPIHandler(
		deps.Coordinator,
		deps.HealthChecker,
		internalMode,
		WithAPIObservability(deps.Obs),
		WithEvaluationService(deps.Evaluation),
		WithAttachmentStore(attachmentStore),
		WithSandboxClient(sandboxClient),
		WithDevMode(devMode),
		WithMaxCreateTaskBodySize(taskBodyLimit),
		WithMemoryService(deps.MemoryService),
	)

	var authMiddleware func(http.Handler) http.Handler
	if deps.AuthHandler != nil && deps.AuthService != nil {
		authMiddleware = AuthMiddleware(deps.AuthService)
	}

	wrap := func(handler http.Handler) http.Handler {
		if authMiddleware == nil {
			return handler
		}
		return authMiddleware(handler)
	}

	// Create mux using Go 1.22+ method-specific patterns.
	mux := http.NewServeMux()

	// ── Internal / dev endpoints ──

	mux.Handle("GET /api/internal/sessions/{session_id}/context", routeHandler("/api/internal/sessions/:session_id/context", http.HandlerFunc(apiHandler.HandleGetContextSnapshots)))

	if devMode {
		mux.Handle("GET /api/dev/sessions/{session_id}/context-window", routeHandler("/api/dev/sessions/:session_id/context-window", wrap(http.HandlerFunc(apiHandler.HandleGetContextWindowPreview))))
		mux.Handle("GET /api/dev/logs", routeHandler("/api/dev/logs", wrap(http.HandlerFunc(apiHandler.HandleDevLogTrace))))
		mux.Handle("GET /api/dev/memory", routeHandler("/api/dev/memory", wrap(http.HandlerFunc(apiHandler.HandleDevMemory))))

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
	if internalMode {
		appsConfigHandler := NewAppsConfigHandler(config.LoadAppsConfig, config.SaveAppsConfig)
		if appsConfigHandler != nil {
			mux.Handle("GET /api/internal/config/apps", routeHandler("/api/internal/config/apps", wrap(http.HandlerFunc(appsConfigHandler.HandleGetAppsConfig))))
			mux.Handle("PUT /api/internal/config/apps", routeHandler("/api/internal/config/apps", wrap(http.HandlerFunc(appsConfigHandler.HandleUpdateAppsConfig))))
		}
	}

	// ── SSE / streaming ──

	mux.Handle("GET /api/sse", routeHandler("/api/sse", wrap(http.HandlerFunc(sseHandler.HandleSSEStream))))
	mux.Handle("GET /api/share/sessions/{session_id}", routeHandler("/api/share/sessions/:session_id", http.HandlerFunc(shareHandler.HandleSharedSession)))
	if attachmentStore != nil {
		mux.Handle("/api/attachments/", routeHandler("/api/attachments", attachmentStore.Handler()))
	}
	mux.Handle("POST /api/metrics/web-vitals", routeHandler("/api/metrics/web-vitals", http.HandlerFunc(apiHandler.HandleWebVitals)))
	mux.Handle("GET /api/sandbox/browser-info", routeHandler("/api/sandbox/browser-info", wrap(http.HandlerFunc(apiHandler.HandleSandboxBrowserInfo))))
	mux.Handle("GET /api/sandbox/browser-screenshot", routeHandler("/api/sandbox/browser-screenshot", wrap(http.HandlerFunc(apiHandler.HandleSandboxBrowserScreenshot))))

	// ── Auth endpoints ──
	// Auth handlers manage their own method constraints; registered without method prefix.

	if deps.AuthHandler != nil {
		mux.Handle("/api/auth/register", routeHandler("/api/auth/register", http.HandlerFunc(deps.AuthHandler.HandleRegister)))
		mux.Handle("/api/auth/login", routeHandler("/api/auth/login", http.HandlerFunc(deps.AuthHandler.HandleLogin)))
		mux.Handle("/api/auth/logout", routeHandler("/api/auth/logout", http.HandlerFunc(deps.AuthHandler.HandleLogout)))
		mux.Handle("/api/auth/refresh", routeHandler("/api/auth/refresh", http.HandlerFunc(deps.AuthHandler.HandleRefresh)))
		mux.Handle("/api/auth/me", routeHandler("/api/auth/me", http.HandlerFunc(deps.AuthHandler.HandleMe)))
		mux.Handle("/api/auth/plans", routeHandler("/api/auth/plans", http.HandlerFunc(deps.AuthHandler.HandleListPlans)))
		if internalMode {
			mux.Handle("/api/auth/points", routeHandler("/api/auth/points", wrap(http.HandlerFunc(deps.AuthHandler.HandleAdjustPoints))))
			mux.Handle("/api/auth/subscription", routeHandler("/api/auth/subscription", wrap(http.HandlerFunc(deps.AuthHandler.HandleUpdateSubscription))))
		}
		mux.Handle("/api/auth/google/login", routeHandler("/api/auth/google/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deps.AuthHandler.HandleOAuthStart(domain.ProviderGoogle, w, r)
		})))
		mux.Handle("/api/auth/google/callback", routeHandler("/api/auth/google/callback", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deps.AuthHandler.HandleOAuthCallback(domain.ProviderGoogle, w, r)
		})))
		mux.Handle("/api/auth/wechat/login", routeHandler("/api/auth/wechat/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deps.AuthHandler.HandleOAuthStart(domain.ProviderWeChat, w, r)
		})))
		mux.Handle("/api/auth/wechat/callback", routeHandler("/api/auth/wechat/callback", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deps.AuthHandler.HandleOAuthCallback(domain.ProviderWeChat, w, r)
		})))
	} else {
		authDisabled := func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Authentication module not configured", http.StatusServiceUnavailable)
		}
		mux.Handle("/api/auth/register", routeHandler("/api/auth/register", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/login", routeHandler("/api/auth/login", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/logout", routeHandler("/api/auth/logout", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/refresh", routeHandler("/api/auth/refresh", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/me", routeHandler("/api/auth/me", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/plans", routeHandler("/api/auth/plans", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/points", routeHandler("/api/auth/points", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/subscription", routeHandler("/api/auth/subscription", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/google/login", routeHandler("/api/auth/google/login", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/google/callback", routeHandler("/api/auth/google/callback", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/wechat/login", routeHandler("/api/auth/wechat/login", http.HandlerFunc(authDisabled)))
		mux.Handle("/api/auth/wechat/callback", routeHandler("/api/auth/wechat/callback", http.HandlerFunc(authDisabled)))
	}

	// ── Task endpoints ──

	mux.Handle("POST /api/tasks", routeHandler("/api/tasks", wrap(http.HandlerFunc(apiHandler.HandleCreateTask))))
	mux.Handle("GET /api/tasks", routeHandler("/api/tasks", wrap(http.HandlerFunc(apiHandler.HandleListTasks))))
	mux.Handle("GET /api/tasks/{task_id}", routeHandler("/api/tasks/:task_id", wrap(http.HandlerFunc(apiHandler.HandleGetTask))))
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
