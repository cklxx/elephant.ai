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

	agentapp "alex/internal/agent/app"
	agentports "alex/internal/agent/ports"
	"alex/internal/agent/types"
	"alex/internal/observability"
	"alex/internal/server/app"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

const maxCreateTaskBodySize = 1 << 20 // 1 MiB

// APIHandler handles REST API endpoints
type APIHandler struct {
	coordinator   *app.ServerCoordinator
	healthChecker *app.HealthCheckerImpl
	logger        *utils.Logger
	internalMode  bool
	obs           *observability.Observability
}

// APIHandlerOption configures API handler behavior.
type APIHandlerOption func(*APIHandler)

// WithAPIObservability wires observability components into the handler.
func WithAPIObservability(obs *observability.Observability) APIHandlerOption {
	return func(handler *APIHandler) {
		handler.obs = obs
	}
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(coordinator *app.ServerCoordinator, healthChecker *app.HealthCheckerImpl, internalMode bool, opts ...APIHandlerOption) *APIHandler {
	handler := &APIHandler{
		coordinator:   coordinator,
		healthChecker: healthChecker,
		logger:        utils.NewComponentLogger("APIHandler"),
		internalMode:  internalMode,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(handler)
	}
	return handler
}

// CreateTaskRequest matches TypeScript CreateTaskRequest interface
type CreateTaskRequest struct {
	Task        string              `json:"task"`
	SessionID   string              `json:"session_id,omitempty"`
	AgentPreset string              `json:"agent_preset,omitempty"` // Agent persona preset
	ToolPreset  string              `json:"tool_preset,omitempty"`  // Tool access preset
	Attachments []AttachmentPayload `json:"attachments,omitempty"`
}

// CreateTaskResponse matches TypeScript CreateTaskResponse interface
type CreateTaskResponse struct {
	TaskID       string `json:"task_id"`
	SessionID    string `json:"session_id"`
	Status       string `json:"status"`
	ParentTaskID string `json:"parent_task_id,omitempty"`
}

type apiErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// AttachmentPayload represents an attachment sent from the client.
type AttachmentPayload struct {
	Name                string `json:"name"`
	MediaType           string `json:"media_type"`
	Data                string `json:"data,omitempty"`
	URI                 string `json:"uri,omitempty"`
	Description         string `json:"description,omitempty"`
	Kind                string `json:"kind,omitempty"`
	Format              string `json:"format,omitempty"`
	RetentionTTLSeconds uint64 `json:"retention_ttl_seconds,omitempty"`
}

type ContextSnapshotItem struct {
	RequestID        string               `json:"request_id"`
	Iteration        int                  `json:"iteration"`
	Timestamp        string               `json:"timestamp"`
	TaskID           string               `json:"task_id,omitempty"`
	ParentTaskID     string               `json:"parent_task_id,omitempty"`
	Messages         []agentports.Message `json:"messages"`
	ExcludedMessages []agentports.Message `json:"excluded_messages,omitempty"`
}

type ContextSnapshotResponse struct {
	SessionID string                `json:"session_id"`
	Snapshots []ContextSnapshotItem `json:"snapshots"`
}

type SessionSnapshotItem struct {
	TurnID     int    `json:"turn_id"`
	LLMTurnSeq int    `json:"llm_turn_seq"`
	Summary    string `json:"summary"`
	CreatedAt  string `json:"created_at"`
}

type SessionSnapshotsResponse struct {
	SessionID  string                `json:"session_id"`
	Items      []SessionSnapshotItem `json:"items"`
	NextCursor string                `json:"next_cursor,omitempty"`
}

type TurnSnapshotResponse struct {
	SessionID  string                      `json:"session_id"`
	TurnID     int                         `json:"turn_id"`
	LLMTurnSeq int                         `json:"llm_turn_seq"`
	Summary    string                      `json:"summary"`
	CreatedAt  string                      `json:"created_at"`
	Plans      []agentports.PlanNode       `json:"plans,omitempty"`
	Beliefs    []agentports.Belief         `json:"beliefs,omitempty"`
	WorldState map[string]any              `json:"world_state,omitempty"`
	Diff       map[string]any              `json:"diff,omitempty"`
	Messages   []agentports.Message        `json:"messages"`
	Feedback   []agentports.FeedbackSignal `json:"feedback,omitempty"`
}

type webVitalPayload struct {
	Name           string  `json:"name"`
	Value          float64 `json:"value"`
	Delta          float64 `json:"delta,omitempty"`
	ID             string  `json:"id,omitempty"`
	Label          string  `json:"label,omitempty"`
	Page           string  `json:"page,omitempty"`
	NavigationType string  `json:"navigation_type,omitempty"`
	Timestamp      int64   `json:"ts,omitempty"`
}

const maxWebVitalBodySize = 1 << 14

// HandleCreateTask handles POST /api/tasks - creates and executes a new task
func (h *APIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Limit request body size to avoid resource exhaustion attacks
	body := http.MaxBytesReader(w, r.Body, maxCreateTaskBodySize)
	defer func() {
		_ = body.Close()
	}()

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
			h.writeJSONError(w, http.StatusBadRequest, "Request body is empty", err)
			return
		case errors.As(err, &syntaxErr):
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON at position %d", syntaxErr.Offset), err)
			return
		case errors.As(err, &typeErr):
			h.writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("Invalid value for field '%s'", typeErr.Field), err)
			return
		case errors.As(err, &maxBytesErr):
			h.writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large", err)
			return
		default:
			h.writeJSONError(w, http.StatusBadRequest, "Invalid request body", err)
			return
		}
	}

	// Ensure there are no extra JSON tokens
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return
	}

	if req.Task == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Task is required", fmt.Errorf("task field empty"))
		return
	}

	sessionID, err := isValidOptionalSessionID(req.SessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}
	req.SessionID = sessionID

	attachments, err := h.parseAttachments(req.Attachments)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	h.logger.Info("Creating task: task='%s', sessionID='%s'", req.Task, req.SessionID)

	ctx := id.WithSessionID(r.Context(), req.SessionID)
	if len(attachments) > 0 {
		ctx = agentapp.WithUserAttachments(ctx, attachments)
	}

	// Execute task asynchronously - coordinator returns immediately after creating task record
	// Background goroutine will handle actual execution and update status
	task, err := h.coordinator.ExecuteTaskAsync(ctx, req.Task, req.SessionID, req.AgentPreset, req.ToolPreset)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create task", err)
		return
	}

	h.logger.Info("Task created successfully: taskID=%s, sessionID=%s", task.ID, task.SessionID)

	// Return task response matching TypeScript interface
	response := CreateTaskResponse{
		TaskID:       task.ID,
		SessionID:    task.SessionID,
		Status:       string(task.Status),
		ParentTaskID: task.ParentTaskID,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleWebVitals ingests frontend performance signals.
func (h *APIHandler) HandleWebVitals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body := http.MaxBytesReader(w, r.Body, maxWebVitalBodySize)
	defer body.Close()
	var payload webVitalPayload
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if payload.Name == "" {
		h.writeJSONError(w, http.StatusBadRequest, "name is required", fmt.Errorf("name missing"))
		return
	}
	page := canonicalPath(payload.Page)
	if h.obs != nil {
		h.obs.Metrics.RecordWebVital(r.Context(), payload.Name, payload.Label, page, payload.Value, payload.Delta)
	}
	w.WriteHeader(http.StatusAccepted)
}

func (h *APIHandler) parseAttachments(payloads []AttachmentPayload) ([]agentports.Attachment, error) {
	if len(payloads) == 0 {
		return nil, nil
	}

	attachments := make([]agentports.Attachment, 0, len(payloads))
	for _, incoming := range payloads {
		name := strings.TrimSpace(incoming.Name)
		if name == "" {
			return nil, fmt.Errorf("attachment name is required")
		}

		mediaType := strings.TrimSpace(incoming.MediaType)
		if mediaType == "" {
			return nil, fmt.Errorf("attachment media_type is required")
		}

		data := strings.TrimSpace(incoming.Data)
		uri := strings.TrimSpace(incoming.URI)
		if data == "" && uri == "" {
			return nil, fmt.Errorf("attachment '%s' must include data or uri", name)
		}

		attachment := agentports.Attachment{
			Name:                name,
			MediaType:           mediaType,
			Data:                data,
			URI:                 uri,
			Description:         strings.TrimSpace(incoming.Description),
			Source:              "user_upload",
			Kind:                strings.TrimSpace(incoming.Kind),
			Format:              strings.TrimSpace(incoming.Format),
			RetentionTTLSeconds: incoming.RetentionTTLSeconds,
		}
		if attachment.URI == "" && attachment.Data != "" {
			attachment.URI = fmt.Sprintf("data:%s;base64,%s", attachment.MediaType, attachment.Data)
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

// HandleGetSession handles GET /api/sessions/:id
func (h *APIHandler) HandleGetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract session ID from URL path
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	session, err := h.coordinator.GetSession(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Session not found", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(session); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
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
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	sessionIDs, err := h.coordinator.ListSessions(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list sessions", err)
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
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleDeleteSession handles DELETE /api/sessions/:id
func (h *APIHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract session ID from URL path
	sessionID := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	if err := h.coordinator.DeleteSession(r.Context(), sessionID); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to delete session", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleListSnapshots handles GET /api/sessions/:id/snapshots
func (h *APIHandler) HandleListSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	trimmed = strings.TrimSuffix(trimmed, "/snapshots")
	sessionID := strings.TrimSuffix(strings.TrimSpace(trimmed), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		} else {
			h.writeJSONError(w, http.StatusBadRequest, "limit must be a positive integer", err)
			return
		}
	}
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	items, nextCursor, err := h.coordinator.ListSnapshots(r.Context(), sessionID, cursor, limit)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list snapshots", err)
		return
	}
	responseItems := make([]SessionSnapshotItem, 0, len(items))
	for _, meta := range items {
		responseItems = append(responseItems, SessionSnapshotItem{
			TurnID:     meta.TurnID,
			LLMTurnSeq: meta.LLMTurnSeq,
			Summary:    meta.Summary,
			CreatedAt:  meta.CreatedAt.Format(time.RFC3339),
		})
	}
	resp := SessionSnapshotsResponse{SessionID: sessionID, Items: responseItems}
	if nextCursor != "" {
		resp.NextCursor = nextCursor
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleGetTurnSnapshot handles GET /api/sessions/:id/turns/:turnID
func (h *APIHandler) HandleGetTurnSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	parts := strings.Split(trimmed, "/turns/")
	if len(parts) != 2 {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid snapshot path", fmt.Errorf("invalid path %s", r.URL.Path))
		return
	}
	sessionID := strings.TrimSuffix(strings.TrimSpace(parts[0]), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}
	turnStr := parts[1]
	if strings.Contains(turnStr, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid turn path", fmt.Errorf("invalid path segment %s", turnStr))
		return
	}
	turnID, err := strconv.Atoi(strings.TrimSpace(turnStr))
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "turn_id must be numeric", err)
		return
	}
	snapshot, err := h.coordinator.GetSnapshot(r.Context(), sessionID, turnID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Snapshot not found", err)
		return
	}
	resp := TurnSnapshotResponse{
		SessionID:  snapshot.SessionID,
		TurnID:     snapshot.TurnID,
		LLMTurnSeq: snapshot.LLMTurnSeq,
		Summary:    snapshot.Summary,
		CreatedAt:  snapshot.CreatedAt.Format(time.RFC3339),
		Plans:      snapshot.Plans,
		Beliefs:    snapshot.Beliefs,
		WorldState: snapshot.World,
		Diff:       snapshot.Diff,
		Messages:   snapshot.Messages,
		Feedback:   snapshot.Feedback,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleReplaySession handles POST /api/sessions/:id/replay
func (h *APIHandler) HandleReplaySession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}
	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/replay")
	sessionID = strings.TrimSuffix(strings.TrimSpace(sessionID), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}
	if err := h.coordinator.ReplaySession(r.Context(), sessionID); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to schedule replay", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":     "scheduled",
		"session_id": sessionID,
	})
}

// TaskStatusResponse matches TypeScript TaskStatusResponse interface
type TaskStatusResponse = types.AgentTask

// HandleGetTask handles GET /api/tasks/:id
func (h *APIHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract task ID from URL path
	taskID := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if taskID == "" || strings.Contains(taskID, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Task ID required", fmt.Errorf("invalid task id '%s'", taskID))
		return
	}

	task, err := h.coordinator.GetTask(r.Context(), taskID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Task not found", err)
		return
	}

	// Convert to TaskStatusResponse
	response := TaskStatusResponse{
		TaskID:       task.ID,
		SessionID:    task.SessionID,
		ParentTaskID: task.ParentTaskID,
		Status:       string(task.Status),
		CreatedAt:    task.CreatedAt.Format(time.RFC3339),
		Error:        task.Error,
	}

	if task.CompletedAt != nil {
		completedStr := task.CompletedAt.Format(time.RFC3339)
		response.CompletedAt = &completedStr
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleListTasks handles GET /api/tasks
func (h *APIHandler) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
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
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list tasks", err)
		return
	}

	// Convert to TaskStatusResponse array
	taskResponses := make([]TaskStatusResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = TaskStatusResponse{
			TaskID:       task.ID,
			SessionID:    task.SessionID,
			ParentTaskID: task.ParentTaskID,
			Status:       string(task.Status),
			CreatedAt:    task.CreatedAt.Format(time.RFC3339),
			Error:        task.Error,
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
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleCancelTask handles POST /api/tasks/:id/cancel
func (h *APIHandler) HandleCancelTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract task ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	taskID := strings.TrimSuffix(path, "/cancel")
	if taskID == "" || taskID == path {
		h.writeJSONError(w, http.StatusBadRequest, "Task ID required", fmt.Errorf("invalid task id '%s'", taskID))
		return
	}

	if err := h.coordinator.CancelTask(r.Context(), taskID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Failed to cancel task", err)
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
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	// Extract session ID from URL path
	path := strings.TrimPrefix(r.URL.Path, "/api/sessions/")
	sessionID := strings.TrimSuffix(path, "/fork")
	if sessionID == "" || sessionID == path {
		h.writeJSONError(w, http.StatusBadRequest, "Session ID required", fmt.Errorf("invalid session id '%s'", sessionID))
		return
	}

	sessionID = strings.TrimSpace(sessionID)
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	newSession, err := h.coordinator.ForkSession(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Failed to fork session", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(newSession); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleInternalSessionRequest routes internal session endpoints.
func (h *APIHandler) HandleInternalSessionRequest(w http.ResponseWriter, r *http.Request) {
	if !h.internalMode {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/context") {
		h.HandleGetContextSnapshots(w, r)
		return
	}

	http.NotFound(w, r)
}

// HandleGetContextSnapshots handles GET /api/internal/sessions/:id/context.
func (h *APIHandler) HandleGetContextSnapshots(w http.ResponseWriter, r *http.Request) {
	if !h.internalMode {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/internal/sessions/")
	path = strings.TrimSuffix(path, "/context")
	sessionID := strings.Trim(path, "/")
	if sessionID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Session ID required", fmt.Errorf("invalid session id"))
		return
	}

	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	snapshots := h.coordinator.GetContextSnapshots(sessionID)
	response := ContextSnapshotResponse{
		SessionID: sessionID,
		Snapshots: make([]ContextSnapshotItem, len(snapshots)),
	}

	for i, snapshot := range snapshots {
		item := ContextSnapshotItem{
			RequestID:    snapshot.RequestID,
			Iteration:    snapshot.Iteration,
			Timestamp:    snapshot.Timestamp.Format(time.RFC3339Nano),
			TaskID:       snapshot.TaskID,
			ParentTaskID: snapshot.ParentTaskID,
			Messages:     snapshot.Messages,
		}
		if len(snapshot.Excluded) > 0 {
			item.ExcludedMessages = snapshot.Excluded
		}
		response.Snapshots[i] = item
	}

	h.writeJSON(w, http.StatusOK, response)
}

// HandleHealthCheck handles GET /health
func (h *APIHandler) HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check all component health
	components := h.healthChecker.CheckAll(r.Context())

	// Determine overall status
	overallStatus := "healthy"
	allReady := true
	for _, comp := range components {
		// Only care about components that should be ready (not disabled)
		if comp.Status != "disabled" && comp.Status != "ready" {
			allReady = false
		}
		if comp.Status == "error" {
			overallStatus = "unhealthy"
			break
		}
	}

	if !allReady && overallStatus != "unhealthy" {
		overallStatus = "degraded"
	}

	response := map[string]interface{}{
		"status":     overallStatus,
		"components": components,
	}

	// Set HTTP status based on health
	httpStatus := http.StatusOK
	if overallStatus == "unhealthy" {
		httpStatus = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("Failed to encode health check response: %v", err)
	}
}

func (h *APIHandler) writeJSONError(w http.ResponseWriter, status int, message string, err error) {
	if err != nil {
		h.logger.Error("HTTP %d - %s: %v", status, message, err)
	} else {
		h.logger.Warn("HTTP %d - %s", status, message)
	}

	resp := apiErrorResponse{Error: message}
	if err != nil {
		resp.Details = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(resp); encodeErr != nil {
		h.logger.Error("Failed to encode error response: %v", encodeErr)
	}
}

func (h *APIHandler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		h.logger.Error("Failed to encode JSON response: %v", err)
	}
}
