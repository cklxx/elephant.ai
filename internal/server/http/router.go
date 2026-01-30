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

	mux.Handle("/api/internal/sessions/", routeHandler("/api/internal/sessions", http.HandlerFunc(apiHandler.HandleInternalSessionRequest)))

	if devMode {
		devSessionHandler := routeHandler("/api/dev/sessions/:session_id/context-window", wrap(http.HandlerFunc(apiHandler.HandleDevSessionRequest)))
		mux.Handle("/api/dev/sessions", devSessionHandler)
		mux.Handle("/api/dev/sessions/", devSessionHandler)
		mux.Handle("/api/dev/logs", routeHandler("/api/dev/logs", wrap(http.HandlerFunc(apiHandler.HandleDevLogTrace))))
		mux.Handle("/api/dev/memory", routeHandler("/api/dev/memory", wrap(http.HandlerFunc(apiHandler.HandleDevMemory))))

		contextConfigHandler := NewContextConfigHandler("")
		if contextConfigHandler != nil {
			devContextHandler := routeHandler("/api/dev/context-config", wrap(http.HandlerFunc(contextConfigHandler.HandleContextConfig)))
			devContextPreviewHandler := routeHandler("/api/dev/context-config/preview", wrap(http.HandlerFunc(contextConfigHandler.HandleContextPreview)))
			mux.Handle("/api/dev/context-config", devContextHandler)
			mux.Handle("/api/dev/context-config/", devContextHandler)
			mux.Handle("/api/dev/context-config/preview", devContextPreviewHandler)
			mux.Handle("/api/dev/context-config/preview/", devContextPreviewHandler)
		}
	}

	if (internalMode || devMode) && deps.ConfigHandler != nil {
		runtimeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				deps.ConfigHandler.HandleGetRuntimeConfig(w, r)
			case http.MethodPut:
				deps.ConfigHandler.HandleUpdateRuntimeConfig(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.Handle("/api/internal/config/runtime", routeHandler("/api/internal/config/runtime", wrap(runtimeHandler)))
		mux.Handle("/api/internal/config/runtime/stream", routeHandler("/api/internal/config/runtime/stream", wrap(http.HandlerFunc(deps.ConfigHandler.HandleRuntimeStream))))
		mux.Handle("/api/internal/config/runtime/models", routeHandler("/api/internal/config/runtime/models", wrap(http.HandlerFunc(deps.ConfigHandler.HandleGetRuntimeModels))))
		mux.Handle("/api/internal/subscription/catalog", routeHandler("/api/internal/subscription/catalog", wrap(http.HandlerFunc(deps.ConfigHandler.HandleGetSubscriptionCatalog))))
	}
	if internalMode {
		appsConfigHandler := NewAppsConfigHandler(config.LoadAppsConfig, config.SaveAppsConfig)
		if appsConfigHandler != nil {
			mux.Handle("/api/internal/config/apps", routeHandler("/api/internal/config/apps", wrap(http.HandlerFunc(appsConfigHandler.HandleAppsConfig))))
		}
	}

	// ── SSE / streaming ──

	mux.Handle("/api/sse", routeHandler("/api/sse", wrap(http.HandlerFunc(sseHandler.HandleSSEStream))))
	mux.Handle("/api/share/sessions/", routeHandler("/api/share/sessions/:session_id", http.HandlerFunc(shareHandler.HandleSharedSession)))
	if attachmentStore != nil {
		mux.Handle("/api/attachments/", routeHandler("/api/attachments", attachmentStore.Handler()))
	}
	mux.Handle("/api/metrics/web-vitals", routeHandler("/api/metrics/web-vitals", http.HandlerFunc(apiHandler.HandleWebVitals)))
	mux.Handle("/api/sandbox/browser-info", routeHandler("/api/sandbox/browser-info", wrap(http.HandlerFunc(apiHandler.HandleSandboxBrowserInfo))))
	mux.Handle("/api/sandbox/browser-screenshot", routeHandler("/api/sandbox/browser-screenshot", wrap(http.HandlerFunc(apiHandler.HandleSandboxBrowserScreenshot))))

	// ── Auth endpoints ──

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

	mux.Handle("/api/tasks", routeHandler("/api/tasks", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			apiHandler.HandleCreateTask(w, r)
		case http.MethodGet:
			apiHandler.HandleListTasks(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	mux.Handle("/api/tasks/", routeHandler("/api/tasks/:task_id", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")

		if strings.HasSuffix(path, "/cancel") {
			annotateRequestRoute(r, "/api/tasks/:task_id/cancel")
			apiHandler.HandleCancelTask(w, r)
			return
		}

		if !strings.Contains(path, "/") {
			annotateRequestRoute(r, "/api/tasks/:task_id")
			apiHandler.HandleGetTask(w, r)
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
	}))))

	// ── Evaluation endpoints ──

	mux.Handle("/api/evaluations", routeHandler("/api/evaluations", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			apiHandler.HandleListEvaluations(w, r)
		case http.MethodPost:
			apiHandler.HandleStartEvaluation(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	mux.Handle("/api/evaluations/", routeHandler("/api/evaluations/:evaluation_id", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/evaluations/")
		if strings.Contains(path, "/") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			apiHandler.HandleGetEvaluation(w, r)
		case http.MethodDelete:
			apiHandler.HandleDeleteEvaluation(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// ── Agent catalog endpoints ──

	mux.Handle("/api/agents", routeHandler("/api/agents", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			apiHandler.HandleListAgents(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	mux.Handle("/api/agents/", routeHandler("/api/agents/:agent_id/evaluations", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/agents/")

		switch {
		case strings.HasSuffix(path, "/evaluations"):
			annotateRequestRoute(r, "/api/agents/:agent_id/evaluations")
			apiHandler.HandleListAgentEvaluations(w, r)
			return
		case strings.Contains(path, "/"):
			http.Error(w, "Not found", http.StatusNotFound)
			return
		default:
			annotateRequestRoute(r, "/api/agents/:agent_id")
			apiHandler.HandleGetAgent(w, r)
			return
		}
	}))))

	// ── Session endpoints ──

	sessionsHandler := routeHandler("/api/sessions", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sessions/" || r.URL.Path == "/api/sessions" {
			annotateRequestRoute(r, "/api/sessions")
			switch r.Method {
			case http.MethodGet:
				apiHandler.HandleListSessions(w, r)
			case http.MethodPost:
				apiHandler.HandleCreateSession(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
		if strings.HasSuffix(path, "/persona") {
			annotateRequestRoute(r, "/api/sessions/:session_id/persona")
			switch r.Method {
			case http.MethodGet:
				apiHandler.HandleGetSessionPersona(w, r)
			case http.MethodPut:
				apiHandler.HandleUpdateSessionPersona(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if strings.HasSuffix(path, "/snapshots") {
			annotateRequestRoute(r, "/api/sessions/:session_id/snapshots")
			apiHandler.HandleListSnapshots(w, r)
			return
		}
		if strings.Contains(path, "/turns/") {
			annotateRequestRoute(r, "/api/sessions/:session_id/turns/:turn_id")
			apiHandler.HandleGetTurnSnapshot(w, r)
			return
		}
		if strings.HasSuffix(path, "/replay") {
			annotateRequestRoute(r, "/api/sessions/:session_id/replay")
			apiHandler.HandleReplaySession(w, r)
			return
		}
		if strings.HasSuffix(path, "/share") {
			annotateRequestRoute(r, "/api/sessions/:session_id/share")
			apiHandler.HandleCreateSessionShare(w, r)
			return
		}
		if strings.HasSuffix(path, "/fork") {
			annotateRequestRoute(r, "/api/sessions/:session_id/fork")
			apiHandler.HandleForkSession(w, r)
			return
		}
		if !strings.Contains(path, "/") {
			switch r.Method {
			case http.MethodGet:
				annotateRequestRoute(r, "/api/sessions/:session_id")
				apiHandler.HandleGetSession(w, r)
			case http.MethodDelete:
				annotateRequestRoute(r, "/api/sessions/:session_id")
				apiHandler.HandleDeleteSession(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
	})))
	mux.Handle("/api/sessions", sessionsHandler)
	mux.Handle("/api/sessions/", sessionsHandler)

	// ── Health check ──

	mux.Handle("/health", routeHandler("/health", http.HandlerFunc(apiHandler.HandleHealthCheck)))

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
