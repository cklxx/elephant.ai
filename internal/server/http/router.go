package http

import (
	"net/http"
	"strings"

	"alex/internal/auth/domain"
	"alex/internal/server/app"
	"alex/internal/utils"
)

// NewRouter creates a new HTTP router with all endpoints
func NewRouter(coordinator *app.ServerCoordinator, broadcaster *app.EventBroadcaster, healthChecker *app.HealthCheckerImpl, authHandler *AuthHandler, environment string) http.Handler {
	logger := utils.NewComponentLogger("Router")

	// Create handlers
	sseHandler := NewSSEHandler(broadcaster)
	apiHandler := NewAPIHandler(coordinator, healthChecker)

	// Create mux
	mux := http.NewServeMux()

	// SSE endpoint
	mux.HandleFunc("/api/sse", sseHandler.HandleSSEStream)

	if authHandler != nil {
		mux.HandleFunc("/api/auth/register", authHandler.HandleRegister)
		mux.HandleFunc("/api/auth/login", authHandler.HandleLogin)
		mux.HandleFunc("/api/auth/logout", authHandler.HandleLogout)
		mux.HandleFunc("/api/auth/refresh", authHandler.HandleRefresh)
		mux.HandleFunc("/api/auth/me", authHandler.HandleMe)
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
	}

	// Task endpoints
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			apiHandler.HandleCreateTask(w, r)
		case http.MethodGet:
			apiHandler.HandleListTasks(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tasks/", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// Session endpoints
	mux.HandleFunc("/api/sessions/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sessions/" || r.URL.Path == "/api/sessions" {
			apiHandler.HandleListSessions(w, r)
		} else {
			path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")

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
	})

	// Health check endpoint
	mux.HandleFunc("/health", apiHandler.HandleHealthCheck)

	// Apply middleware
	var handler http.Handler = mux
	handler = LoggingMiddleware(logger)(handler)
	handler = CORSMiddleware(environment)(handler)

	return handler
}
