package http

import (
	"net/http"
	"strconv"
	"strings"

	"alex/internal/attachments"
	authapp "alex/internal/auth/app"
	"alex/internal/auth/domain"
	runtimeconfig "alex/internal/config"
	"alex/internal/logging"
	"alex/internal/observability"
	"alex/internal/server/app"
)

func createTaskBodyLimit(env runtimeconfig.EnvLookup) int64 {
	if env == nil {
		return defaultMaxCreateTaskBodySize
	}

	raw, ok := env("ALEX_WEB_MAX_TASK_BODY_BYTES")
	if !ok {
		return defaultMaxCreateTaskBodySize
	}

	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return defaultMaxCreateTaskBodySize
	}

	return value
}

// NewRouter creates a new HTTP router with all endpoints
func NewRouter(coordinator *app.ServerCoordinator, broadcaster *app.EventBroadcaster, healthChecker *app.HealthCheckerImpl, authHandler *AuthHandler, authService *authapp.Service, environment string, allowedOrigins []string, configHandler *ConfigHandler, evaluationService *app.EvaluationService, obs *observability.Observability, attachmentCfg attachments.StoreConfig) http.Handler {
	logger := logging.NewComponentLogger("Router")
	latencyLogger := logging.NewLatencyLogger("HTTP")
	envLookup := runtimeconfig.DefaultEnvLookup
	attachmentStore := (*AttachmentStore)(nil)
	if strings.TrimSpace(attachmentCfg.Dir) == "" {
		attachmentCfg.Dir = "~/.alex-web-attachments"
	}
	if strings.TrimSpace(attachmentCfg.Provider) == "" {
		attachmentCfg.Provider = attachments.ProviderLocal
	}
	if store, err := NewAttachmentStore(attachmentCfg); err != nil {
		logger.Warn("Attachment store disabled: %v", err)
	} else {
		attachmentStore = store
	}
	taskBodyLimit := createTaskBodyLimit(envLookup)

	// Create handlers
	sseHandler := NewSSEHandler(broadcaster, WithSSEObservability(obs), WithSSEAttachmentStore(attachmentStore))
	internalMode := strings.EqualFold(environment, "internal") || strings.EqualFold(environment, "evaluation")
	apiHandler := NewAPIHandler(
		coordinator,
		healthChecker,
		internalMode,
		WithAPIObservability(obs),
		WithEvaluationService(evaluationService),
		WithAttachmentStore(attachmentStore),
		WithMaxCreateTaskBodySize(taskBodyLimit),
	)

	var authMiddleware func(http.Handler) http.Handler
	if authHandler != nil && authService != nil {
		authMiddleware = AuthMiddleware(authService)
	}

	wrap := func(handler http.Handler) http.Handler {
		if authMiddleware == nil {
			return handler
		}
		return authMiddleware(handler)
	}

	// Create mux
	mux := http.NewServeMux()

	mux.Handle("/api/internal/sessions/", routeHandler("/api/internal/sessions", http.HandlerFunc(apiHandler.HandleInternalSessionRequest)))

	if internalMode && configHandler != nil {
		runtimeHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				configHandler.HandleGetRuntimeConfig(w, r)
			case http.MethodPut:
				configHandler.HandleUpdateRuntimeConfig(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})
		mux.Handle("/api/internal/config/runtime", routeHandler("/api/internal/config/runtime", wrap(runtimeHandler)))
		mux.Handle("/api/internal/config/runtime/stream", routeHandler("/api/internal/config/runtime/stream", wrap(http.HandlerFunc(configHandler.HandleRuntimeStream))))
	}

	// SSE endpoint
	mux.Handle("/api/sse", routeHandler("/api/sse", wrap(http.HandlerFunc(sseHandler.HandleSSEStream))))
	if attachmentStore != nil {
		mux.Handle("/api/attachments/", routeHandler("/api/attachments", attachmentStore.Handler()))
	}
	mux.Handle("/api/metrics/web-vitals", routeHandler("/api/metrics/web-vitals", http.HandlerFunc(apiHandler.HandleWebVitals)))

	if authHandler != nil {
		mux.Handle("/api/auth/register", routeHandler("/api/auth/register", http.HandlerFunc(authHandler.HandleRegister)))
		mux.Handle("/api/auth/login", routeHandler("/api/auth/login", http.HandlerFunc(authHandler.HandleLogin)))
		mux.Handle("/api/auth/logout", routeHandler("/api/auth/logout", http.HandlerFunc(authHandler.HandleLogout)))
		mux.Handle("/api/auth/refresh", routeHandler("/api/auth/refresh", http.HandlerFunc(authHandler.HandleRefresh)))
		mux.Handle("/api/auth/me", routeHandler("/api/auth/me", http.HandlerFunc(authHandler.HandleMe)))
		mux.Handle("/api/auth/plans", routeHandler("/api/auth/plans", http.HandlerFunc(authHandler.HandleListPlans)))
		if internalMode {
			mux.Handle("/api/auth/points", routeHandler("/api/auth/points", wrap(http.HandlerFunc(authHandler.HandleAdjustPoints))))
			mux.Handle("/api/auth/subscription", routeHandler("/api/auth/subscription", wrap(http.HandlerFunc(authHandler.HandleUpdateSubscription))))
		}
		mux.Handle("/api/auth/google/login", routeHandler("/api/auth/google/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHandler.HandleOAuthStart(domain.ProviderGoogle, w, r)
		})))
		mux.Handle("/api/auth/google/callback", routeHandler("/api/auth/google/callback", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHandler.HandleOAuthCallback(domain.ProviderGoogle, w, r)
		})))
		mux.Handle("/api/auth/wechat/login", routeHandler("/api/auth/wechat/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHandler.HandleOAuthStart(domain.ProviderWeChat, w, r)
		})))
		mux.Handle("/api/auth/wechat/callback", routeHandler("/api/auth/wechat/callback", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHandler.HandleOAuthCallback(domain.ProviderWeChat, w, r)
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

	// Task endpoints
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

	// Evaluation endpoints
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

	// Agent catalog endpoints
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
			// Only GET is supported for now; more agent-specific endpoints can be added later.
			annotateRequestRoute(r, "/api/agents/:agent_id")
			apiHandler.HandleGetAgent(w, r)
			return
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

	mux.Handle("/api/tasks/", routeHandler("/api/tasks/:task_id", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")

		// Handle /api/tasks/:id/cancel
		if strings.HasSuffix(path, "/cancel") {
			annotateRequestRoute(r, "/api/tasks/:task_id/cancel")
			apiHandler.HandleCancelTask(w, r)
			return
		}

		// Handle /api/tasks/:id
		if !strings.Contains(path, "/") {
			annotateRequestRoute(r, "/api/tasks/:task_id")
			apiHandler.HandleGetTask(w, r)
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
	}))))

	// Session endpoints
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
	// Handle both `/api/sessions` and `/api/sessions/` without relying on ServeMux redirects.
	mux.Handle("/api/sessions", sessionsHandler)
	mux.Handle("/api/sessions/", sessionsHandler)

	// Health check endpoint
	mux.Handle("/health", routeHandler("/health", http.HandlerFunc(apiHandler.HandleHealthCheck)))

	// Apply middleware
	var handler http.Handler = mux
	handler = ObservabilityMiddleware(obs, latencyLogger)(handler)
	handler = LoggingMiddleware(logger)(handler)
	handler = CORSMiddleware(environment, allowedOrigins)(handler)

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
