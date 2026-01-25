package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	agentports "alex/internal/agent/ports"
)

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

type ContextWindowPreviewResponse struct {
	SessionID     string                   `json:"session_id"`
	TokenEstimate int                      `json:"token_estimate"`
	TokenLimit    int                      `json:"token_limit"`
	PersonaKey    string                   `json:"persona_key,omitempty"`
	ToolMode      string                   `json:"tool_mode,omitempty"`
	ToolPreset    string                   `json:"tool_preset,omitempty"`
	Window        agentports.ContextWindow `json:"window"`
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

// HandleDevSessionRequest routes development-only session endpoints.
func (h *APIHandler) HandleDevSessionRequest(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/context-window") {
		h.HandleGetContextWindowPreview(w, r)
		return
	}

	http.NotFound(w, r)
}

// HandleGetContextWindowPreview handles GET /api/dev/sessions/:id/context-window.
func (h *APIHandler) HandleGetContextWindowPreview(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/dev/sessions/")
	path = strings.TrimSuffix(path, "/context-window")
	sessionID := strings.Trim(path, "/")
	if sessionID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Session ID required", fmt.Errorf("invalid session id"))
		return
	}

	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	preview, err := h.coordinator.PreviewContextWindow(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Failed to build context window", err)
		return
	}

	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)
	sanitizedWindow := preview.Window
	sanitizedWindow.SessionID = sessionID
	sanitizedWindow.Messages = sanitizeMessagesForDelivery(preview.Window.Messages, sentAttachments)

	response := ContextWindowPreviewResponse{
		SessionID:     sessionID,
		TokenEstimate: preview.TokenEstimate,
		TokenLimit:    preview.TokenLimit,
		PersonaKey:    preview.PersonaKey,
		ToolMode:      preview.ToolMode,
		ToolPreset:    preview.ToolPreset,
		Window:        sanitizedWindow,
	}

	h.writeJSON(w, http.StatusOK, response)
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

	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)

	for i, snapshot := range snapshots {
		item := ContextSnapshotItem{
			RequestID:    snapshot.RequestID,
			Iteration:    snapshot.Iteration,
			Timestamp:    snapshot.Timestamp.Format(time.RFC3339Nano),
			TaskID:       snapshot.TaskID,
			ParentTaskID: snapshot.ParentTaskID,
		}
		item.Messages = sanitizeMessagesForDelivery(snapshot.Messages, sentAttachments)
		if len(snapshot.Excluded) > 0 {
			item.ExcludedMessages = sanitizeMessagesForDelivery(snapshot.Excluded, sentAttachments)
		}
		response.Snapshots[i] = item
	}

	h.writeJSON(w, http.StatusOK, response)
}

func sanitizeMessagesForDelivery(messages []agentports.Message, sentAttachments *stringLRU) []agentports.Message {
	if len(messages) == 0 {
		return nil
	}

	sanitized := make([]agentports.Message, 0, len(messages))
	for _, msg := range messages {
		cloned := msg
		if sanitizedAttachments := sanitizeAttachmentsForStream(msg.Attachments, sentAttachments, nil, nil, false); len(sanitizedAttachments) > 0 {
			cloned.Attachments = sanitizedAttachments
		} else {
			cloned.Attachments = nil
		}

		sanitized = append(sanitized, cloned)
	}

	return sanitized
}
