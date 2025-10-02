package http

import (
	"net/http"

	"alex/internal/server/app"
	"alex/internal/utils"
)

// NewRouter creates a new HTTP router with all endpoints
func NewRouter(coordinator *app.ServerCoordinator, broadcaster *app.EventBroadcaster) http.Handler {
	logger := utils.NewComponentLogger("Router")

	// Create handlers
	sseHandler := NewSSEHandler(broadcaster)
	apiHandler := NewAPIHandler(coordinator)

	// Create mux
	mux := http.NewServeMux()

	// SSE endpoint
	mux.HandleFunc("/api/sse", sseHandler.HandleSSEStream)

	// REST API endpoints
	mux.HandleFunc("/api/tasks", apiHandler.HandleCreateTask)
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/sessions" {
			apiHandler.HandleListSessions(w, r)
		} else {
			// Handle /api/sessions/:id
			switch r.Method {
			case http.MethodGet:
				apiHandler.HandleGetSession(w, r)
			case http.MethodDelete:
				apiHandler.HandleDeleteSession(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", apiHandler.HandleHealthCheck)

	// Apply middleware
	var handler http.Handler = mux
	handler = LoggingMiddleware(logger)(handler)
	handler = CORSMiddleware(handler)

	return handler
}
