package http

import (
	"net/http"
	"strings"

	"alex/internal/infra/memory"
)

// MemoryDailyEntry is the JSON-serialisable form of a daily memory entry.
type MemoryDailyEntry struct {
	Date    string `json:"date"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// MemorySnapshot is the JSON response for GET /api/dev/memory.
type MemorySnapshot struct {
	UserID   string             `json:"user_id"`
	LongTerm string             `json:"long_term"`
	Daily    []MemoryDailyEntry `json:"daily"`
}

// HandleGetMemorySnapshot handles GET /api/dev/memory.
func (h *APIHandler) HandleGetMemorySnapshot(w http.ResponseWriter, r *http.Request) {
	if !h.devMode {
		http.NotFound(w, r)
		return
	}
	if h.memoryEngine == nil {
		h.writeJSONError(w, http.StatusNotFound, "memory engine not configured", nil)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	session, err := h.sessions.GetSession(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Session not found", err)
		return
	}

	userID := strings.TrimSpace(session.Metadata["user_id"])

	longTerm, err := h.memoryEngine.LoadLongTerm(r.Context(), userID)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to load memory", err)
		return
	}

	entries, err := h.memoryEngine.ListDailyEntries(r.Context(), userID)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to load daily memory", err)
		return
	}

	daily := snapshotsToEntries(entries)

	h.writeJSON(w, http.StatusOK, MemorySnapshot{
		UserID:   userID,
		LongTerm: longTerm,
		Daily:    daily,
	})
}

func snapshotsToEntries(snapshots []memory.DailySnapshot) []MemoryDailyEntry {
	if len(snapshots) == 0 {
		return nil
	}
	entries := make([]MemoryDailyEntry, len(snapshots))
	for i, s := range snapshots {
		entries[i] = MemoryDailyEntry{
			Date:    s.Date,
			Path:    s.Path,
			Content: s.Content,
		}
	}
	return entries
}
