package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alex/internal/server/app"
	"alex/internal/utils"
)

const maxCreateTaskBodySize = 1 << 20 // 1 MiB

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

// CreateTaskRequest matches TypeScript CreateTaskRequest interface
type CreateTaskRequest struct {
	Task        string `json:"task"`
	SessionID   string `json:"session_id,omitempty"`
	AgentPreset string `json:"agent_preset,omitempty"` // Agent persona preset
	ToolPreset  string `json:"tool_preset,omitempty"`  // Tool access preset
}

// CreateTaskResponse matches TypeScript CreateTaskResponse interface
type CreateTaskResponse struct {
	TaskID    string `json:"task_id"`
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
}

// HandleCreateTask handles POST /api/tasks - creates and executes a new task
func (h *APIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Limit request body size to avoid resource exhaustion attacks
	body := http.MaxBytesReader(w, r.Body, maxCreateTaskBodySize)
	defer body.Close()

	// Parse request body
	var req CreateTaskRequest

	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&req); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.Is(err, io.EOF):
			http.Error(w, "Request body is empty", http.StatusBadRequest)
			return
		case errors.As(err, &syntaxErr):
			http.Error(w, fmt.Sprintf("Invalid JSON at position %d", syntaxErr.Offset), http.StatusBadRequest)
			return
		case errors.As(err, &typeErr):
			http.Error(w, fmt.Sprintf("Invalid value for field '%s'", typeErr.Field), http.StatusBadRequest)
			return
		case errors.As(err, &maxBytesErr):
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		default:
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}

	// Ensure there are no extra JSON tokens
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "Request body must contain a single JSON object", http.StatusBadRequest)
		return
	}

	if req.Task == "" {
		http.Error(w, "Task is required", http.StatusBadRequest)
		return
	}

	sessionID, err := isValidOptionalSessionID(req.SessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.SessionID = sessionID

	h.logger.Info("Creating task: task='%s', sessionID='%s'", req.Task, req.SessionID)

	// Execute task asynchronously - coordinator returns immediately after creating task record
	// Background goroutine will handle actual execution and update status
	task, err := h.coordinator.ExecuteTaskAsync(r.Context(), req.Task, req.SessionID, req.AgentPreset, req.ToolPreset)
	if err != nil {
		h.logger.Error("Failed to create task: %v", err)
		http.Error(w, fmt.Sprintf("Failed to create task: %v", err), http.StatusInternalServerError)
		return
	}

	h.logger.Info("Task created successfully: taskID=%s, sessionID=%s", task.ID, task.SessionID)

	// Return task response matching TypeScript interface
	response := CreateTaskResponse{
		TaskID:    task.ID,
		SessionID: task.SessionID,
		Status:    string(task.Status),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
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
	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

// SessionResponse matches TypeScript Session interface
type SessionResponse struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	TaskCount int    `json:"task_count"`
	LastTask  string `json:"last_task,omitempty"`
}

// SessionListResponse matches TypeScript SessionListResponse interface
type SessionListResponse struct {
	Sessions []SessionResponse `json:"sessions"`
	Total    int               `json:"total"`
}

// HandleListSessions handles GET /api/sessions
func (h *APIHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionIDs, err := h.coordinator.ListSessions(r.Context())
	if err != nil {
		http.Error(w, "Failed to list sessions", http.StatusInternalServerError)
		return
	}

	// Convert session IDs to full session objects
	sessions := make([]SessionResponse, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		session, err := h.coordinator.GetSession(r.Context(), id)
		if err != nil {
			continue // Skip sessions that can't be loaded
		}

		// Get tasks for this session to populate task_count and last_task
		tasks, _ := h.coordinator.ListSessionTasks(r.Context(), id)
		taskCount := len(tasks)
		lastTask := ""
		if taskCount > 0 {
			// Tasks are sorted newest first
			lastTask = tasks[0].Description
		}

		sessions = append(sessions, SessionResponse{
			ID:        session.ID,
			CreatedAt: session.CreatedAt.Format(time.RFC3339),
			UpdatedAt: session.UpdatedAt.Format(time.RFC3339),
			TaskCount: taskCount,
			LastTask:  lastTask,
		})
	}

	response := SessionListResponse{
		Sessions: sessions,
		Total:    len(sessions),
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
	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.coordinator.DeleteSession(r.Context(), sessionID); err != nil {
		http.Error(w, "Failed to delete session", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TaskStatusResponse matches TypeScript TaskStatusResponse interface
type TaskStatusResponse struct {
	TaskID      string  `json:"task_id"`
	SessionID   string  `json:"session_id"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
	Error       string  `json:"error,omitempty"`
}

// HandleGetTask handles GET /api/tasks/:id
func (h *APIHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from URL path
	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" || strings.Contains(taskID, "/") {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	task, err := h.coordinator.GetTask(r.Context(), taskID)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Convert to TaskStatusResponse
	response := TaskStatusResponse{
		TaskID:    task.ID,
		SessionID: task.SessionID,
		Status:    string(task.Status),
		CreatedAt: task.CreatedAt.Format(time.RFC3339),
		Error:     task.Error,
	}

	if task.CompletedAt != nil {
		completedStr := task.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedStr
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleListTasks handles GET /api/tasks
func (h *APIHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse pagination parameters
	limit := 10
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	tasks, total, err := h.coordinator.ListTasks(r.Context(), limit, offset)
	if err != nil {
		http.Error(w, "Failed to list tasks", http.StatusInternalServerError)
		return
	}

	// Convert to TaskStatusResponse array
	taskResponses := make([]TaskStatusResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = TaskStatusResponse{
			TaskID:    task.ID,
			SessionID: task.SessionID,
			Status:    string(task.Status),
			CreatedAt: task.CreatedAt.Format(time.RFC3339),
			Error:     task.Error,
		}
		if task.CompletedAt != nil {
			completedStr := task.CompletedAt.Format(time.RFC3339)
			taskResponses[i].CompletedAt = &completedStr
		}
	}

	response := map[string]interface{}{
		"tasks":  taskResponses,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// HandleCancelTask handles POST /api/tasks/:id/cancel
func (h *APIHandler) HandleCancelTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract task ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	taskID := strings.TrimSuffix(path, "/cancel")
	if taskID == "" || taskID == path {
		http.Error(w, "Task ID required", http.StatusBadRequest)
		return
	}

	if err := h.coordinator.CancelTask(r.Context(), taskID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel task: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "cancelled",
		"task_id": taskID,
	}); err != nil {
		h.logger.Error("Failed to encode cancel response: %v", err)
	}
}

// HandleForkSession handles POST /api/sessions/:id/fork
func (h *APIHandler) HandleForkSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract session ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID := strings.TrimSuffix(path, "/fork")
	if sessionID == "" || sessionID == path {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}

	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	newSession, err := h.coordinator.ForkSession(r.Context(), sessionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fork session: %v", err), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(newSession); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
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
