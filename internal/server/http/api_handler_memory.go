package http

import (
	"net/http"
	"strconv"
	"strings"

	"alex/internal/memory"
)

const defaultDevMemoryLimit = 20

// HandleDevMemory handles GET /api/dev/memory?session_id=...&limit=20
// It queries the memory store for entries associated with the session's user.
func (h *APIHandler) HandleDevMemory(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.memoryService == nil {
		h.writeJSONError(w, http.StatusServiceUnavailable, "memory service not available", nil)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "session_id is required", nil)
		return
	}

	limit := defaultDevMemoryLimit
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Resolve user_id: load session metadata, then fall back to session ID prefix heuristic.
	userID := ""
	session, err := h.coordinator.GetSession(r.Context(), sessionID)
	if err == nil && session != nil && session.Metadata != nil {
		userID = strings.TrimSpace(session.Metadata["user_id"])
	}
	if userID == "" {
		if strings.HasPrefix(sessionID, "lark-") || strings.HasPrefix(sessionID, "wechat-") {
			userID = sessionID
		}
	}
	if userID == "" {
		h.writeJSONError(w, http.StatusBadRequest, "cannot resolve user_id for session", nil)
		return
	}

	entries, err := h.memoryService.Recall(r.Context(), memory.Query{
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "failed to recall memories", err)
		return
	}

	h.writeJSON(w, http.StatusOK, entries)
}
