package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentports "alex/internal/agent/ports"
)

const (
	maxSessionListLimit  = 200
	maxSnapshotListLimit = 200
)

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

type SessionPersonaRequest struct {
	UserPersona *agentports.UserPersonaProfile `json:"user_persona"`
}

type SessionPersonaResponse struct {
	SessionID   string                         `json:"session_id"`
	UserPersona *agentports.UserPersonaProfile `json:"user_persona,omitempty"`
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

// SessionResponse matches TypeScript Session interface
type SessionResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title,omitempty"`
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

type CreateSessionResponse struct {
	SessionID string `json:"session_id"`
}

type ShareSessionResponse struct {
	SessionID  string                   `json:"session_id"`
	ShareToken string                   `json:"share_token"`
	Events     []map[string]interface{} `json:"events,omitempty"`
	Title      string                   `json:"title,omitempty"`
	CreatedAt  string                   `json:"created_at,omitempty"`
	UpdatedAt  string                   `json:"updated_at,omitempty"`
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

// HandleGetSessionPersona handles GET /api/sessions/:id/persona
func (h *APIHandler) HandleGetSessionPersona(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/persona")
	sessionID = strings.TrimSuffix(strings.TrimSpace(sessionID), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	session, err := h.coordinator.GetSession(r.Context(), sessionID)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "Session not found", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionPersonaResponse{
		SessionID:   session.ID,
		UserPersona: session.UserPersona,
	}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleUpdateSessionPersona handles PUT /api/sessions/:id/persona
func (h *APIHandler) HandleUpdateSessionPersona(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/persona")
	sessionID = strings.TrimSuffix(strings.TrimSpace(sessionID), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	var req SessionPersonaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	if req.UserPersona == nil {
		h.writeJSONError(w, http.StatusBadRequest, "user_persona is required", fmt.Errorf("user_persona is required"))
		return
	}
	if req.UserPersona.UpdatedAt.IsZero() {
		req.UserPersona.UpdatedAt = time.Now()
	}

	session, err := h.coordinator.UpdateSessionPersona(r.Context(), sessionID, req.UserPersona)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to update persona", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(SessionPersonaResponse{
		SessionID:   session.ID,
		UserPersona: session.UserPersona,
	}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleCreateSession handles POST /api/sessions
func (h *APIHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	session, err := h.coordinator.CreateSession(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create session", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(CreateSessionResponse{SessionID: session.ID}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleCreateSessionShare handles POST /api/sessions/:id/share
func (h *APIHandler) HandleCreateSessionShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/sessions/"), "/share")
	sessionID = strings.TrimSuffix(strings.TrimSpace(sessionID), "/")
	if err := validateSessionID(sessionID); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	shareToken, err := h.coordinator.EnsureSessionShareToken(r.Context(), sessionID, false)
	if err != nil {
		if strings.Contains(err.Error(), "session not found") {
			h.writeJSONError(w, http.StatusNotFound, "Session not found", err)
			return
		}
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to create share token", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(ShareSessionResponse{
		SessionID:  sessionID,
		ShareToken: shareToken,
	}); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to encode response", err)
	}
}

// HandleListSessions handles GET /api/sessions
func (h *APIHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method not allowed", fmt.Errorf("method %s not allowed", r.Method))
		return
	}

	limit := 50
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			h.writeJSONError(w, http.StatusBadRequest, "limit must be a positive integer", err)
			return
		}
		limit = parsed
	}
	if limit > maxSessionListLimit {
		limit = maxSessionListLimit
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			h.writeJSONError(w, http.StatusBadRequest, "offset must be a non-negative integer", err)
			return
		}
		offset = parsed
	}

	sessionIDs, err := h.coordinator.ListSessions(r.Context(), limit, offset)
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
			Title:     strings.TrimSpace(session.Metadata["title"]),
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
	if limit > maxSnapshotListLimit {
		limit = maxSnapshotListLimit
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
