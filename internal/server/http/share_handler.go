package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"alex/internal/logging"
	"alex/internal/server/app"
)

// ShareHandler serves read-only share endpoints.
type ShareHandler struct {
	coordinator *app.ServerCoordinator
	sseHandler  *SSEHandler
	logger      logging.Logger
}

// NewShareHandler creates a share handler.
func NewShareHandler(coordinator *app.ServerCoordinator, sseHandler *SSEHandler) *ShareHandler {
	return &ShareHandler{
		coordinator: coordinator,
		sseHandler:  sseHandler,
		logger:      logging.NewComponentLogger("ShareHandler"),
	}
}

// HandleSharedSession handles GET /api/share/sessions/:session_id
func (h *ShareHandler) HandleSharedSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := strings.TrimPrefix(r.URL.Path, "/api/share/sessions/")
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || strings.Contains(sessionID, "/") {
		http.Error(w, "Invalid session path", http.StatusBadRequest)
		return
	}
	if err := validateSessionID(sessionID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	token := strings.TrimSpace(r.URL.Query().Get("token"))
	session, err := h.coordinator.ValidateShareToken(r.Context(), sessionID, token)
	if err != nil {
		if errors.Is(err, app.ErrShareTokenInvalid) {
			http.Error(w, "Invalid share token", http.StatusForbidden)
			return
		}
		if strings.Contains(err.Error(), "session not found") {
			http.Error(w, "Session not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to validate share token: %v", err)
		http.Error(w, "Unable to load shared session", http.StatusInternalServerError)
		return
	}

	events := h.sseHandler.broadcaster.GetEventHistory(sessionID)
	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)
	finalAnswerCache := newStringLRU(sseFinalAnswerCacheSize)
	serialized := make([]map[string]interface{}, 0, len(events))
	for _, event := range events {
		if !h.sseHandler.shouldStreamEvent(event, false) {
			continue
		}
		if isDelegationToolEvent(event) {
			continue
		}
		payload, err := h.sseHandler.buildEventData(event, sentAttachments, finalAnswerCache, false)
		if err != nil {
			h.logger.Error("Failed to serialize shared event: %v", err)
			http.Error(w, "Failed to serialize shared session", http.StatusInternalServerError)
			return
		}
		serialized = append(serialized, payload)
	}

	title := ""
	if session.Metadata != nil {
		title = strings.TrimSpace(session.Metadata["title"])
	}

	resp := ShareSessionResponse{
		SessionID: session.ID,
		Title:     title,
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
		UpdatedAt: session.UpdatedAt.Format(time.RFC3339),
		Events:    serialized,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
