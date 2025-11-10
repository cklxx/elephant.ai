package http

import (
	"context"
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
	"alex/internal/server/app"
	"alex/internal/utils"
	id "alex/internal/utils/id"
)

const maxCreateTaskBodySize = 1 << 20      // 1 MiB
const maxArticleInsightsBodySize = 1 << 19 // 512 KiB
const maxImageConceptsBodySize = 1 << 18   // 256 KiB
const maxWebBlueprintBodySize = 1 << 18    // 256 KiB
const maxCodePlanBodySize = 1 << 18        // 256 KiB

// APIHandler handles REST API endpoints
type APIHandler struct {
	coordinator      *app.ServerCoordinator
	healthChecker    *app.HealthCheckerImpl
	craftService     *app.CraftService
	workbenchService *app.WorkbenchService
	logger           *utils.Logger
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(coordinator *app.ServerCoordinator, healthChecker *app.HealthCheckerImpl, craftService *app.CraftService, workbenchService *app.WorkbenchService) *APIHandler {
	return &APIHandler{
		coordinator:      coordinator,
		healthChecker:    healthChecker,
		craftService:     craftService,
		workbenchService: workbenchService,
		logger:           utils.NewComponentLogger("APIHandler"),
	}
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
	Name        string `json:"name"`
	MediaType   string `json:"media_type"`
	Data        string `json:"data,omitempty"`
	URI         string `json:"uri,omitempty"`
	Description string `json:"description,omitempty"`
}

type articleInsightsRequest struct {
	Content string `json:"content"`
}

type articleSaveRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Content   string `json:"content"`
	Summary   string `json:"summary,omitempty"`
}

type articleDraftListResponse struct {
	Drafts []app.ArticleDraft `json:"drafts"`
}

type imageConceptsRequest struct {
	Brief      string   `json:"brief"`
	Style      string   `json:"style,omitempty"`
	References []string `json:"references,omitempty"`
}

type webBlueprintRequest struct {
	Goal      string   `json:"goal"`
	Audience  string   `json:"audience,omitempty"`
	Tone      string   `json:"tone,omitempty"`
	MustHaves []string `json:"must_haves,omitempty"`
}

type webBlueprintResponse struct {
	Blueprint app.WebBlueprint `json:"blueprint"`
	SessionID string           `json:"session_id,omitempty"`
	TaskID    string           `json:"task_id,omitempty"`
	RawAnswer string           `json:"raw_answer,omitempty"`
}

type codePlanRequest struct {
	ServiceName  string   `json:"service_name"`
	Objective    string   `json:"objective"`
	Language     string   `json:"language,omitempty"`
	Features     []string `json:"features,omitempty"`
	Integrations []string `json:"integrations,omitempty"`
}

type codePlanResponse struct {
	Plan      app.CodeServicePlan `json:"plan"`
	SessionID string              `json:"session_id,omitempty"`
	TaskID    string              `json:"task_id,omitempty"`
	RawAnswer string              `json:"raw_answer,omitempty"`
}

// HandleDeleteArticleDraft handles DELETE /api/workbench/article/crafts/:id
func (h *APIHandler) HandleDeleteArticleDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.workbenchService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Workbench service not configured", fmt.Errorf("workbench service unavailable"))
		return
	}

	userID, err := h.ensureUser(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	craftID := strings.TrimPrefix(r.URL.Path, "/api/workbench/article/crafts/")
	craftID = strings.TrimSpace(craftID)
	if craftID == "" || strings.Contains(craftID, "/") {
		h.writeJSONError(w, http.StatusBadRequest, "Craft ID is required", fmt.Errorf("invalid craft id"))
		return
	}

	ctx := id.WithUserID(r.Context(), userID)
	if err := h.workbenchService.DeleteArticleDraft(ctx, craftID); err != nil {
		switch {
		case errors.Is(err, app.ErrWorkbenchMissingUser):
			h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		case errors.Is(err, app.ErrWorkbenchDraftNotFound):
			h.writeJSONError(w, http.StatusNotFound, "Draft not found", err)
		default:
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to delete draft", err)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleGenerateImageConcepts handles POST /api/workbench/image/concepts
func (h *APIHandler) HandleGenerateImageConcepts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.workbenchService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Workbench service not configured", fmt.Errorf("workbench service unavailable"))
		return
	}

	userID, err := h.ensureUser(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxImageConceptsBodySize)
	defer func() {
		_ = body.Close()
	}()

	var req imageConceptsRequest
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

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return
	}

	if strings.TrimSpace(req.Brief) == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Brief is required", app.ErrWorkbenchContentRequired)
		return
	}

	ctx := id.WithUserID(r.Context(), userID)
	result, err := h.workbenchService.GenerateImageConcepts(ctx, app.GenerateImageConceptsRequest{
		Brief:      req.Brief,
		Style:      req.Style,
		References: req.References,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrWorkbenchContentRequired):
			h.writeJSONError(w, http.StatusBadRequest, "Brief is required", err)
		case errors.Is(err, app.ErrWorkbenchMissingUser):
			h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		default:
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to generate image concepts", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleGenerateWebBlueprint handles POST /api/workbench/web/blueprint
func (h *APIHandler) HandleGenerateWebBlueprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.workbenchService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Workbench service not configured", fmt.Errorf("workbench service unavailable"))
		return
	}

	userID, err := h.ensureUser(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxWebBlueprintBodySize)
	defer func() {
		_ = body.Close()
	}()

	var req webBlueprintRequest
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

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return
	}

	if strings.TrimSpace(req.Goal) == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Goal is required", app.ErrWorkbenchContentRequired)
		return
	}

	ctx := id.WithUserID(r.Context(), userID)
	result, err := h.workbenchService.GenerateWebBlueprint(ctx, app.GenerateWebBlueprintRequest{
		Goal:      req.Goal,
		Audience:  req.Audience,
		Tone:      req.Tone,
		MustHaves: req.MustHaves,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrWorkbenchContentRequired):
			h.writeJSONError(w, http.StatusBadRequest, "Goal is required", err)
			return
		case errors.Is(err, app.ErrWorkbenchMissingUser):
			h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
			return
		default:
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to generate web blueprint", err)
			return
		}
	}

	response := webBlueprintResponse{
		Blueprint: result.Blueprint,
		SessionID: result.SessionID,
		TaskID:    result.TaskID,
		RawAnswer: result.RawAnswer,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleGenerateCodePlan handles POST /api/workbench/code/plan
func (h *APIHandler) HandleGenerateCodePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.workbenchService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Workbench service not configured", fmt.Errorf("workbench service unavailable"))
		return
	}

	userID, err := h.ensureUser(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxCodePlanBodySize)
	defer func() {
		_ = body.Close()
	}()

	var req codePlanRequest
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

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return
	}

	if strings.TrimSpace(req.ServiceName) == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Service name is required", app.ErrWorkbenchServiceNameRequired)
		return
	}
	if strings.TrimSpace(req.Objective) == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Objective is required", app.ErrWorkbenchContentRequired)
		return
	}

	ctx := id.WithUserID(r.Context(), userID)
	result, err := h.workbenchService.GenerateCodeServicePlan(ctx, app.GenerateCodeServicePlanRequest{
		ServiceName:  req.ServiceName,
		Objective:    req.Objective,
		Language:     req.Language,
		Features:     req.Features,
		Integrations: req.Integrations,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrWorkbenchServiceNameRequired):
			h.writeJSONError(w, http.StatusBadRequest, "Service name is required", err)
			return
		case errors.Is(err, app.ErrWorkbenchContentRequired):
			h.writeJSONError(w, http.StatusBadRequest, "Objective is required", err)
			return
		case errors.Is(err, app.ErrWorkbenchMissingUser):
			h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
			return
		default:
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to generate code plan", err)
			return
		}
	}

	response := codePlanResponse{
		Plan:      result.Plan,
		SessionID: result.SessionID,
		TaskID:    result.TaskID,
		RawAnswer: result.RawAnswer,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleCreateTask handles POST /api/tasks - creates and executes a new task
func (h *APIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
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
			Name:        name,
			MediaType:   mediaType,
			Data:        data,
			URI:         uri,
			Description: strings.TrimSpace(incoming.Description),
			Source:      "user_upload",
		}
		if attachment.URI == "" && attachment.Data != "" {
			attachment.URI = fmt.Sprintf("data:%s;base64,%s", attachment.MediaType, attachment.Data)
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

func (h *APIHandler) ensureUser(ctx context.Context) (string, error) {
	userID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if userID == "" {
		return "", fmt.Errorf("missing user context")
	}
	return userID, nil
}

// HandleGetSession handles GET /api/sessions/:id
func (h *APIHandler) HandleGetSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
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

type CraftListResponse struct {
	Crafts []app.Craft `json:"crafts"`
}

type CraftDownloadResponse struct {
	URL string `json:"url"`
}

// HandleListSessions handles GET /api/sessions
func (h *APIHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
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

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
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

// TaskStatusResponse matches TypeScript TaskStatusResponse interface
type TaskStatusResponse = types.AgentTask

// HandleGetTask handles GET /api/tasks/:id
func (h *APIHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
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

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
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

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
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

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
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

// HandleListCrafts handles GET /api/crafts
func (h *APIHandler) HandleListCrafts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}
	if h.craftService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Craft service unavailable", fmt.Errorf("craft service not configured"))
		return
	}

	crafts, err := h.craftService.List(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list crafts", err)
		return
	}

	response := CraftListResponse{Crafts: crafts}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleDeleteCraft handles DELETE /api/crafts/:id
func (h *APIHandler) HandleDeleteCraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}
	if h.craftService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Craft service unavailable", fmt.Errorf("craft service not configured"))
		return
	}

	craftID := strings.TrimPrefix(r.URL.Path, "/api/crafts/")
	craftID = strings.TrimSpace(craftID)
	if craftID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Craft ID required", fmt.Errorf("missing craft id"))
		return
	}

	if err := h.craftService.Delete(r.Context(), craftID); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to delete craft", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDownloadCraft handles GET /api/crafts/:id/download
func (h *APIHandler) HandleDownloadCraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if _, err := h.ensureUser(r.Context()); err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}
	if h.craftService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Craft service unavailable", fmt.Errorf("craft service not configured"))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/crafts/")
	path = strings.TrimSuffix(path, "/download")
	craftID := strings.Trim(path, "/")
	if craftID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Craft ID required", fmt.Errorf("missing craft id"))
		return
	}

	url, err := h.craftService.DownloadURL(r.Context(), craftID, 10*time.Minute)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to generate download URL", err)
		return
	}

	response := CraftDownloadResponse{URL: url}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleGenerateArticleInsights handles POST /api/workbench/article/insights
func (h *APIHandler) HandleGenerateArticleInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.workbenchService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Workbench service not configured", fmt.Errorf("workbench service unavailable"))
		return
	}

	userID, err := h.ensureUser(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxArticleInsightsBodySize)
	defer func() {
		_ = body.Close()
	}()

	var req articleInsightsRequest
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

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Content is required", app.ErrWorkbenchContentRequired)
		return
	}

	ctx := id.WithUserID(r.Context(), userID)
	insights, err := h.workbenchService.GenerateArticleInsights(ctx, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrWorkbenchContentRequired):
			h.writeJSONError(w, http.StatusBadRequest, "Content is required", err)
		case errors.Is(err, app.ErrWorkbenchMissingUser):
			h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		default:
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to generate insights", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(insights); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleListArticleDrafts handles GET /api/workbench/article/crafts
func (h *APIHandler) HandleListArticleDrafts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.workbenchService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Workbench service not configured", fmt.Errorf("workbench service unavailable"))
		return
	}

	userID, err := h.ensureUser(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	ctx := id.WithUserID(r.Context(), userID)
	drafts, err := h.workbenchService.ListArticleDrafts(ctx)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrWorkbenchMissingUser):
			h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		default:
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to list article drafts", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(articleDraftListResponse{Drafts: drafts}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleSaveArticleDraft handles POST /api/workbench/article/crafts
func (h *APIHandler) HandleSaveArticleDraft(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	if h.workbenchService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "Workbench service not configured", fmt.Errorf("workbench service unavailable"))
		return
	}

	userID, err := h.ensureUser(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		return
	}

	body := http.MaxBytesReader(w, r.Body, maxArticleInsightsBodySize)
	defer func() {
		_ = body.Close()
	}()

	var req articleSaveRequest
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

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		h.writeJSONError(w, http.StatusBadRequest, "Request body must contain a single JSON object", fmt.Errorf("unexpected extra JSON token"))
		return
	}

	if strings.TrimSpace(req.Content) == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Content is required", app.ErrWorkbenchContentRequired)
		return
	}

	ctx := id.WithUserID(r.Context(), userID)
	result, err := h.workbenchService.SaveArticleDraft(ctx, app.SaveArticleDraftRequest{
		SessionID: strings.TrimSpace(req.SessionID),
		Title:     req.Title,
		Content:   req.Content,
		Summary:   req.Summary,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrWorkbenchContentRequired):
			h.writeJSONError(w, http.StatusBadRequest, "Content is required", err)
		case errors.Is(err, app.ErrWorkbenchMissingUser):
			h.writeJSONError(w, http.StatusUnauthorized, "Unauthorized", err)
		default:
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to save draft", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
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
