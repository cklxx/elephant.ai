package http

import (
	"net/http"
	"strings"

	"alex/internal/server/app"
	"alex/internal/server/http/auth"
	"alex/internal/utils"
)

// NewRouter creates a new HTTP router with all endpoints
func NewRouter(coordinator *app.ServerCoordinator, broadcaster *app.EventBroadcaster, healthChecker *app.HealthCheckerImpl, craftService *app.CraftService, workbenchService *app.WorkbenchService, environment string) http.Handler {
	logger := utils.NewComponentLogger("Router")

	// Create handlers
	sseHandler := NewSSEHandler(broadcaster)
	apiHandler := NewAPIHandler(coordinator, healthChecker, craftService, workbenchService)

	// Create mux
	mux := http.NewServeMux()

	// SSE endpoint
	mux.HandleFunc("/api/sse", sseHandler.HandleSSEStream)

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

	// Crafts endpoints
	mux.HandleFunc("/api/crafts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			apiHandler.HandleListCrafts(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/crafts/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/crafts/")
		if strings.HasSuffix(path, "/download") {
			apiHandler.HandleDownloadCraft(w, r)
			return
		}
		switch r.Method {
		case http.MethodDelete:
			apiHandler.HandleDeleteCraft(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/workbench/image/concepts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			apiHandler.HandleGenerateImageConcepts(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/workbench/web/blueprint", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			apiHandler.HandleGenerateWebBlueprint(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/workbench/code/plan", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			apiHandler.HandleGenerateCodePlan(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/workbench/article/insights", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			apiHandler.HandleGenerateArticleInsights(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/workbench/article/crafts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			apiHandler.HandleListArticleDrafts(w, r)
		case http.MethodPost:
			apiHandler.HandleSaveArticleDraft(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/workbench/article/crafts/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(strings.TrimPrefix(r.URL.Path, "/api/workbench/article/crafts/"), "/") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		switch r.Method {
		case http.MethodDelete:
			apiHandler.HandleDeleteArticleDraft(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Health check endpoint
	mux.HandleFunc("/health", apiHandler.HandleHealthCheck)

	// Apply middleware
	var handler http.Handler = mux
	handler = LoggingMiddleware(logger)(handler)
	handler = CORSMiddleware(environment)(handler)
	handler = auth.Middleware(handler)

	return handler
}
