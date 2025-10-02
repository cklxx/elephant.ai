package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"alex/internal/server/app"
	"alex/internal/utils"
)

// APIHandler handles REST API endpoints
type APIHandler struct {
	coordinator *app.ServerCoordinator
	logger      *utils.Logger
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(coordinator *app.ServerCoordinator) *APIHandler {
	return &APIHandler{
		coordinator: coordinator,
		logger:      utils.NewComponentLogger("APIHandler"),
	}
}

// HandleCreateTask handles POST /api/tasks - creates and executes a new task
func (h *APIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req struct {
		Task      string `json:"task"`
		SessionID string `json:"session_id,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Task == "" {
		http.Error(w, "Task is required", http.StatusBadRequest)
		return
	}

	h.logger.Info("Creating task: task='%s', sessionID='%s'", req.Task, req.SessionID)

	// Execute task asynchronously
	// The task will stream events via SSE
	go func() {
		result, err := h.coordinator.ExecuteTaskAsync(r.Context(), req.Task, req.SessionID)
		if err != nil {
			h.logger.Error("Task execution failed: %v", err)
			return
		}
		h.logger.Info("Task completed: iterations=%d, tokens=%d", result.Iterations, result.TokensUsed)
	}()

	// Return task accepted response
	response := map[string]interface{}{
		"status":     "accepted",
		"session_id": req.SessionID,
		"message":    "Task is being executed. Connect to SSE endpoint to receive events.",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleGetSession handles GET /api/sessions/:id
func (h *APIHandler) HandleGetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from URL path
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	session, err := h.coordinator.GetSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(session); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleListSessions handles GET /api/sessions
func (h *APIHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessions, err := h.coordinator.ListSessions(r.Context())
	if err != nil {
		http.Error(w, "Failed to list sessions", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"sessions": sessions,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleDeleteSession handles DELETE /api/sessions/:id
func (h *APIHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from URL path
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	if sessionID == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	if err := h.coordinator.DeleteSession(r.Context(), sessionID); err != nil {
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleHealthCheck handles GET /health
func (h *APIHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status": "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
