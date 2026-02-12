package http

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	serverPorts "alex/internal/delivery/server/ports"
	agentports "alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/types"
	id "alex/internal/shared/utils/id"
)

const (
	maxTaskListLimit = 200
)

// CreateTaskRequest matches TypeScript CreateTaskRequest interface
type CreateTaskRequest struct {
	Task         string                  `json:"task"`
	SessionID    string                  `json:"session_id,omitempty"`
	AgentPreset  string                  `json:"agent_preset,omitempty"` // Agent persona preset
	ToolPreset   string                  `json:"tool_preset,omitempty"`  // Tool access preset
	Attachments  []AttachmentPayload     `json:"attachments,omitempty"`
	LLMSelection *subscription.Selection `json:"llm_selection,omitempty"`
}

// CreateTaskResponse matches TypeScript CreateTaskResponse interface
type CreateTaskResponse struct {
	RunID       string `json:"run_id"`
	SessionID   string `json:"session_id"`
	Status      string `json:"status"`
	ParentRunID string `json:"parent_run_id,omitempty"`
}

// AttachmentPayload represents an attachment sent from the client.
type AttachmentPayload struct {
	Name                string `json:"name"`
	MediaType           string `json:"media_type"`
	Data                string `json:"data,omitempty"`
	URI                 string `json:"uri,omitempty"`
	Source              string `json:"source,omitempty"`
	Description         string `json:"description,omitempty"`
	Kind                string `json:"kind,omitempty"`
	Format              string `json:"format,omitempty"`
	RetentionTTLSeconds uint64 `json:"retention_ttl_seconds,omitempty"`
}

// TaskStatusResponse matches TypeScript TaskStatusResponse interface
type TaskStatusResponse = types.AgentTask

// HandleCreateTask handles POST /api/tasks - creates and executes a new task
func (h *APIHandler) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if !h.decodeJSONBody(w, r, &req, h.maxCreateTaskBodySize) {
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
	if user, ok := CurrentUser(r.Context()); ok {
		ctx = id.WithUserID(ctx, user.ID)
	}
	if len(attachments) > 0 {
		ctx = appcontext.WithUserAttachments(ctx, attachments)
	}
	if req.LLMSelection != nil && h.selectionResolver != nil {
		if resolved, ok := h.selectionResolver.Resolve(*req.LLMSelection); ok {
			ctx = appcontext.WithLLMSelection(ctx, resolved)
		}
	}

	// Execute task asynchronously - coordinator returns immediately after creating task record
	// Background goroutine will handle actual execution and update status
	task, err := h.tasks.ExecuteTaskAsync(ctx, req.Task, req.SessionID, req.AgentPreset, req.ToolPreset)
	if err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to create task")
		return
	}

	h.logger.Info("Task created successfully: taskID=%s, sessionID=%s", task.ID, task.SessionID)

	// Return task response matching TypeScript interface
	response := CreateTaskResponse{
		RunID:       task.ID,
		SessionID:   task.SessionID,
		Status:      string(task.Status),
		ParentRunID: task.ParentTaskID,
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
		isImage := strings.HasPrefix(strings.ToLower(mediaType), "image/")

		uri := strings.TrimSpace(incoming.URI)
		data := strings.TrimSpace(incoming.Data)
		if uri == "" && data == "" {
			return nil, fmt.Errorf("attachment '%s' must include data or uri", name)
		}

		var inlineBase64 string
		lowerURI := strings.ToLower(strings.TrimSpace(uri))
		if strings.HasPrefix(lowerURI, "data:") {
			data = uri
			uri = ""
		}

		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(data)), "data:") {
			_, decoded, ok := decodeDataURI(data)
			if !ok {
				return nil, fmt.Errorf("attachment '%s' includes invalid data uri", name)
			}
			if h.attachmentStore == nil {
				return nil, fmt.Errorf("attachment '%s' must include uri (base64 uploads are disabled)", name)
			}
			storedURI, err := h.attachmentStore.StoreBytes(name, mediaType, decoded)
			if err != nil {
				return nil, fmt.Errorf("store attachment '%s': %w", name, err)
			}
			uri = storedURI
			if isImage {
				inlineBase64 = base64.StdEncoding.EncodeToString(decoded)
			}
		} else if uri == "" && data != "" {
			decoded, err := base64.StdEncoding.DecodeString(data)
			if err != nil {
				return nil, fmt.Errorf("attachment '%s' includes invalid base64 payload", name)
			}
			if len(decoded) == 0 {
				return nil, fmt.Errorf("attachment '%s' payload is empty", name)
			}
			if h.attachmentStore == nil {
				return nil, fmt.Errorf("attachment '%s' must include uri (base64 uploads are disabled)", name)
			}
			storedURI, err := h.attachmentStore.StoreBytes(name, mediaType, decoded)
			if err != nil {
				return nil, fmt.Errorf("store attachment '%s': %w", name, err)
			}
			uri = storedURI
			if isImage {
				inlineBase64 = base64.StdEncoding.EncodeToString(decoded)
			}
		}

		if uri == "" || strings.HasPrefix(strings.ToLower(strings.TrimSpace(uri)), "data:") {
			return nil, fmt.Errorf("attachment '%s' must include uri (base64 uploads are disabled)", name)
		}

		source := strings.TrimSpace(incoming.Source)
		if source == "" {
			source = "user_upload"
		}
		attachment := agentports.Attachment{
			Name:                name,
			MediaType:           mediaType,
			Data:                inlineBase64,
			URI:                 uri,
			Description:         strings.TrimSpace(incoming.Description),
			Source:              source,
			Kind:                strings.TrimSpace(incoming.Kind),
			Format:              strings.TrimSpace(incoming.Format),
			RetentionTTLSeconds: incoming.RetentionTTLSeconds,
		}
		attachments = append(attachments, attachment)
	}

	return attachments, nil
}

// HandleGetTask handles GET /api/tasks/{task_id}
func (h *APIHandler) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Task ID required", fmt.Errorf("task id is empty"))
		return
	}

	task, err := h.tasks.GetTask(r.Context(), taskID)
	if err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to retrieve task")
		return
	}

	// Convert to TaskStatusResponse
	response := TaskStatusResponse{
		RunID:       task.ID,
		SessionID:   task.SessionID,
		ParentRunID: task.ParentTaskID,
		Status:      string(task.Status),
		CreatedAt:   task.CreatedAt.Format(time.RFC3339),
		Error:       task.Error,
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
	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID != "" {
		if err := validateSessionID(sessionID); err != nil {
			h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
	}

	// Parse pagination parameters
	limit := 10
	offset := 0

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if limit > maxTaskListLimit {
		limit = maxTaskListLimit
	}

	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	var tasks []*serverPorts.Task
	var total int

	if sessionID != "" {
		sessionTasks, err := h.tasks.ListSessionTasks(r.Context(), sessionID)
		if err != nil {
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to list tasks", err)
			return
		}

		total = len(sessionTasks)
		if offset >= total {
			tasks = []*serverPorts.Task{}
		} else {
			end := offset + limit
			if end > total {
				end = total
			}
			tasks = sessionTasks[offset:end]
		}
	} else {
		allTasks, totalCount, err := h.tasks.ListTasks(r.Context(), limit, offset)
		if err != nil {
			h.writeJSONError(w, http.StatusInternalServerError, "Failed to list tasks", err)
			return
		}
		tasks = allTasks
		total = totalCount
	}

	// Convert to TaskStatusResponse array
	taskResponses := make([]TaskStatusResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = TaskStatusResponse{
			RunID:       task.ID,
			SessionID:   task.SessionID,
			ParentRunID: task.ParentTaskID,
			Status:      string(task.Status),
			CreatedAt:   task.CreatedAt.Format(time.RFC3339),
			Error:       task.Error,
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

// HandleCancelTask handles POST /api/tasks/{task_id}/cancel
func (h *APIHandler) HandleCancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("task_id")
	if taskID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Task ID required", fmt.Errorf("task id is empty"))
		return
	}

	if err := h.tasks.CancelTask(r.Context(), taskID); err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to cancel task")
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

// HandleListActiveTasks handles GET /api/tasks/active — returns all currently running/pending tasks.
func (h *APIHandler) HandleListActiveTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.tasks.ListActiveTasks(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list active tasks", err)
		return
	}

	taskResponses := make([]TaskStatusResponse, len(tasks))
	for i, task := range tasks {
		taskResponses[i] = TaskStatusResponse{
			RunID:       task.ID,
			SessionID:   task.SessionID,
			ParentRunID: task.ParentTaskID,
			Status:      string(task.Status),
			CreatedAt:   task.CreatedAt.Format(time.RFC3339),
			Error:       task.Error,
		}
		if task.CompletedAt != nil {
			completedStr := task.CompletedAt.Format(time.RFC3339)
			taskResponses[i].CompletedAt = &completedStr
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"tasks": taskResponses,
		"total": len(taskResponses),
	}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleGetTaskStats handles GET /api/tasks/stats — returns aggregated task metrics.
func (h *APIHandler) HandleGetTaskStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.tasks.GetTaskStats(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to get task stats", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}
