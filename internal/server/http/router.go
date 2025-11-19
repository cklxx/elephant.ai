package http

import (
	"net/http"
	"strings"

	authapp "alex/internal/auth/app"
	"alex/internal/auth/domain"
	"alex/internal/server/app"
	"alex/internal/utils"
)

// NewRouter creates a new HTTP router with all endpoints
func NewRouter(coordinator *app.ServerCoordinator, broadcaster *app.EventBroadcaster, healthChecker *app.HealthCheckerImpl, authHandler *AuthHandler, authService *authapp.Service, environment string, allowedOrigins []string, configHandler *ConfigHandler) http.Handler {
	logger := utils.NewComponentLogger("Router")

	// Create handlers
	sseHandler := NewSSEHandler(broadcaster)
	internalMode := strings.EqualFold(environment, "internal") || strings.EqualFold(environment, "evaluation")
	apiHandler := NewAPIHandler(coordinator, healthChecker, internalMode)

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

	mux.HandleFunc("/api/internal/sessions/", apiHandler.HandleInternalSessionRequest)

	if internalMode && configHandler != nil {
		mux.Handle("/api/internal/config/runtime", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				configHandler.HandleGetRuntimeConfig(w, r)
			case http.MethodPut:
				configHandler.HandleUpdateRuntimeConfig(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		})))
		mux.Handle("/api/internal/config/runtime/stream", wrap(http.HandlerFunc(configHandler.HandleRuntimeStream)))
	}

	// SSE endpoint
	mux.Handle("/api/sse", wrap(http.HandlerFunc(sseHandler.HandleSSEStream)))

	if authHandler != nil {
		mux.HandleFunc("/api/auth/register", authHandler.HandleRegister)
		mux.HandleFunc("/api/auth/login", authHandler.HandleLogin)
		mux.HandleFunc("/api/auth/logout", authHandler.HandleLogout)
		mux.HandleFunc("/api/auth/refresh", authHandler.HandleRefresh)
		mux.HandleFunc("/api/auth/me", authHandler.HandleMe)
		mux.HandleFunc("/api/auth/plans", authHandler.HandleListPlans)
		if internalMode {
			mux.Handle("/api/auth/points", wrap(http.HandlerFunc(authHandler.HandleAdjustPoints)))
			mux.Handle("/api/auth/subscription", wrap(http.HandlerFunc(authHandler.HandleUpdateSubscription)))
		}
		mux.HandleFunc("/api/auth/google/login", func(w http.ResponseWriter, r *http.Request) {
			authHandler.HandleOAuthStart(domain.ProviderGoogle, w, r)
		})
		mux.HandleFunc("/api/auth/google/callback", func(w http.ResponseWriter, r *http.Request) {
			authHandler.HandleOAuthCallback(domain.ProviderGoogle, w, r)
		})
		mux.HandleFunc("/api/auth/wechat/login", func(w http.ResponseWriter, r *http.Request) {
			authHandler.HandleOAuthStart(domain.ProviderWeChat, w, r)
		})
		mux.HandleFunc("/api/auth/wechat/callback", func(w http.ResponseWriter, r *http.Request) {
			authHandler.HandleOAuthCallback(domain.ProviderWeChat, w, r)
		})
	} else {
		authDisabled := func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Authentication module not configured", http.StatusServiceUnavailable)
		}
		mux.HandleFunc("/api/auth/register", authDisabled)
		mux.HandleFunc("/api/auth/login", authDisabled)
		mux.HandleFunc("/api/auth/logout", authDisabled)
		mux.HandleFunc("/api/auth/refresh", authDisabled)
		mux.HandleFunc("/api/auth/me", authDisabled)
		mux.HandleFunc("/api/auth/plans", authDisabled)
		mux.HandleFunc("/api/auth/points", authDisabled)
		mux.HandleFunc("/api/auth/subscription", authDisabled)
		mux.HandleFunc("/api/auth/google/login", authDisabled)
		mux.HandleFunc("/api/auth/google/callback", authDisabled)
		mux.HandleFunc("/api/auth/wechat/login", authDisabled)
		mux.HandleFunc("/api/auth/wechat/callback", authDisabled)
	}

	// Task endpoints
	mux.Handle("/api/tasks", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			apiHandler.HandleCreateTask(w, r)
		case http.MethodGet:
			apiHandler.HandleListTasks(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))

	mux.Handle("/api/tasks/", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")

		// Handle /api/tasks/:id/cancel
		if strings.HasSuffix(path, "/cancel") {
			apiHandler.HandleCancelTask(w, r)
			return
		}

		// Handle /api/tasks/:id
		if !strings.Contains(path, "/") {
			apiHandler.HandleGetTask(w, r)
			return
		}

		http.Error(w, "Not found", http.StatusNotFound)
	})))

	// Session endpoints
	mux.Handle("/api/sessions/", wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sessions/" || r.URL.Path == "/api/sessions" {
			apiHandler.HandleListSessions(w, r)
		} else {
			path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")

			if strings.HasSuffix(path, "/snapshots") {
				apiHandler.HandleListSnapshots(w, r)
				return
			}
			if strings.Contains(path, "/turns/") {
				apiHandler.HandleGetTurnSnapshot(w, r)
				return
			}
			if strings.HasSuffix(path, "/replay") {
				apiHandler.HandleReplaySession(w, r)
				return
			}

			// Handle /api/sessions/:id/fork
			if strings.HasSuffix(path, "/fork") {
				apiHandler.HandleForkSession(w, r)
				return
			}

			// Handle /api/sessions/:id
			if !strings.Contains(path, "/") {
				switch r.Method {
				case http.MethodGet:
					apiHandler.HandleGetSession(w, r)
				case http.MethodDelete:
					apiHandler.HandleDeleteSession(w, r)
				default:
					http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				}
				return
			}

			http.Error(w, "Not found", http.StatusNotFound)
		}
	})))

	// Health check endpoint
	mux.HandleFunc("/health", apiHandler.HandleHealthCheck)

	// Apply middleware
	var handler http.Handler = mux
	handler = LoggingMiddleware(logger)(handler)
	handler = CORSMiddleware(environment, allowedOrigins)(handler)

	return handler
}
