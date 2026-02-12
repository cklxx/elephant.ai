package http

import (
	"net/http"
	"time"

	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

type ContextSnapshotItem struct {
	RequestID        string         `json:"request_id"`
	Iteration        int            `json:"iteration"`
	Timestamp        string         `json:"timestamp"`
	RunID            string         `json:"run_id,omitempty"`
	ParentRunID      string         `json:"parent_run_id,omitempty"`
	Messages         []core.Message `json:"messages"`
	ExcludedMessages []core.Message `json:"excluded_messages,omitempty"`
}

type ContextSnapshotResponse struct {
	SessionID string                `json:"session_id"`
	Snapshots []ContextSnapshotItem `json:"snapshots"`
}

type ContextWindowPreviewResponse struct {
	SessionID     string              `json:"session_id"`
	TokenEstimate int                 `json:"token_estimate"`
	TokenLimit    int                 `json:"token_limit"`
	PersonaKey    string              `json:"persona_key,omitempty"`
	ToolMode      string              `json:"tool_mode,omitempty"`
	ToolPreset    string              `json:"tool_preset,omitempty"`
	Window        agent.ContextWindow `json:"window"`
}

// HandleGetContextWindowPreview handles GET /api/dev/sessions/{session_id}/context-window.
func (h *APIHandler) HandleGetContextWindowPreview(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}
	if h.snapshots == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "context-window preview unavailable (no coordinator)", nil)
		return
	}

	sessionID := r.PathValue("session_id")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	preview, err := h.snapshots.PreviewContextWindow(r.Context(), sessionID)
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

// HandleGetContextSnapshots handles GET /api/internal/sessions/{session_id}/context.
func (h *APIHandler) HandleGetContextSnapshots(w http.ResponseWriter, r *http.Request) {
	if !h.internalMode {
		http.NotFound(w, r)
		return
	}
	if h.snapshots == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "context snapshots unavailable (no coordinator)", nil)
		return
	}

	sessionID := r.PathValue("session_id")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	snapshots := h.snapshots.GetContextSnapshots(sessionID)
	response := ContextSnapshotResponse{
		SessionID: sessionID,
		Snapshots: make([]ContextSnapshotItem, len(snapshots)),
	}

	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)

	for i, snapshot := range snapshots {
		item := ContextSnapshotItem{
			RequestID:   snapshot.RequestID,
			Iteration:   snapshot.Iteration,
			Timestamp:   snapshot.Timestamp.Format(time.RFC3339Nano),
			RunID:       snapshot.RunID,
			ParentRunID: snapshot.ParentRunID,
		}
		item.Messages = sanitizeMessagesForDelivery(snapshot.Messages, sentAttachments)
		if len(snapshot.Excluded) > 0 {
			item.ExcludedMessages = sanitizeMessagesForDelivery(snapshot.Excluded, sentAttachments)
		}
		response.Snapshots[i] = item
	}

	h.writeJSON(w, http.StatusOK, response)
}

func sanitizeMessagesForDelivery(messages []core.Message, sentAttachments *stringLRU) []core.Message {
	if len(messages) == 0 {
		return nil
	}

	sanitized := make([]core.Message, 0, len(messages))
	for _, msg := range messages {
		cloned := msg
		if sanitizedAttachments := sanitizeAttachmentsForStream(msg.Attachments, sentAttachments, nil, false); len(sanitizedAttachments) > 0 {
			cloned.Attachments = sanitizedAttachments
		} else {
			cloned.Attachments = nil
		}

		sanitized = append(sanitized, cloned)
	}

	return sanitized
}
