package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"alex/internal/delivery/server/app"
	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
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
	UserPersona *core.UserPersonaProfile `json:"user_persona"`
}

type SessionPersonaResponse struct {
	SessionID   string                   `json:"session_id"`
	UserPersona *core.UserPersonaProfile `json:"user_persona,omitempty"`
}

type TurnSnapshotResponse struct {
	SessionID  string                 `json:"session_id"`
	TurnID     int                    `json:"turn_id"`
	LLMTurnSeq int                    `json:"llm_turn_seq"`
	Summary    string                 `json:"summary"`
	CreatedAt  string                 `json:"created_at"`
	Plans      []agent.PlanNode       `json:"plans,omitempty"`
	Beliefs    []agent.Belief         `json:"beliefs,omitempty"`
	WorldState map[string]any         `json:"world_state,omitempty"`
	Diff       map[string]any         `json:"diff,omitempty"`
	Messages   []core.Message         `json:"messages"`
	Feedback   []agent.FeedbackSignal `json:"feedback,omitempty"`
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
	SessionID  string           `json:"session_id"`
	ShareToken string           `json:"share_token"`
	Events     []map[string]any `json:"events,omitempty"`
	Title      string           `json:"title,omitempty"`
	CreatedAt  string           `json:"created_at,omitempty"`
	UpdatedAt  string           `json:"updated_at,omitempty"`
}

// HandleGetSession handles GET /api/sessions/{session_id}
func (h *APIHandler) HandleGetSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := extractRequiredSessionIDFromPath(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	session, err := h.sessions.GetSession(r.Context(), sessionID)
	if err != nil {
		h.writeMappedError(w, err, http.StatusNotFound, "Session not found")
		return
	}
	h.writeJSON(w, http.StatusOK, session)
}

// HandleGetSessionPersona handles GET /api/sessions/{session_id}/persona
func (h *APIHandler) HandleGetSessionPersona(w http.ResponseWriter, r *http.Request) {
	sessionID, err := extractRequiredSessionIDFromPath(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid session ID", err)
		return
	}

	session, err := h.sessions.GetSession(r.Context(), sessionID)
	if err != nil {
		h.writeMappedError(w, err, http.StatusNotFound, "Session not found")
		return
	}
	h.writeJSON(w, http.StatusOK, SessionPersonaResponse{
		SessionID:   session.ID,
		UserPersona: session.UserPersona,
	})
}

// HandleUpdateSessionPersona handles PUT /api/sessions/{session_id}/persona
func (h *APIHandler) HandleUpdateSessionPersona(w http.ResponseWriter, r *http.Request) {
	sessionID, err := extractRequiredSessionIDFromPath(r)
	if err != nil {
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

	session, err := h.sessions.UpdateSessionPersona(r.Context(), sessionID, req.UserPersona)
	if err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to update persona")
		return
	}
	h.writeJSON(w, http.StatusOK, SessionPersonaResponse{
		SessionID:   session.ID,
		UserPersona: session.UserPersona,
	})
}

// HandleCreateSession handles POST /api/sessions
func (h *APIHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	session, err := h.sessions.CreateSession(r.Context())
	if err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to create session")
		return
	}
	h.writeJSON(w, http.StatusCreated, CreateSessionResponse{SessionID: session.ID})
}

// HandleCreateSessionShare handles POST /api/sessions/{session_id}/share
func (h *APIHandler) HandleCreateSessionShare(w http.ResponseWriter, r *http.Request) {
	sessionID, err := extractRequiredSessionIDFromPath(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	shareToken, err := h.sessions.EnsureSessionShareToken(r.Context(), sessionID, false)
	if err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to create share token")
		return
	}
	h.writeJSON(w, http.StatusCreated, ShareSessionResponse{
		SessionID:  sessionID,
		ShareToken: shareToken,
	})
}

// HandleListSessions handles GET /api/sessions
func (h *APIHandler) HandleListSessions(w http.ResponseWriter, r *http.Request) {
	limit, ok := h.parseOptionalQueryInt(
		w,
		r,
		"limit",
		50,
		1,
		maxSessionListLimit,
		"limit must be a positive integer",
		nil,
	)
	if !ok {
		return
	}

	offset, ok := h.parseOptionalQueryInt(
		w,
		r,
		"offset",
		0,
		0,
		0,
		"offset must be a non-negative integer",
		nil,
	)
	if !ok {
		return
	}

	sessionItems, err := h.sessions.ListSessionItems(r.Context(), limit, offset)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, "Failed to list sessions", err)
		return
	}
	sessionIDs := make([]string, 0, len(sessionItems))
	for _, item := range sessionItems {
		sessionIDs = append(sessionIDs, item.ID)
	}

	taskSummaries, err := h.tasks.SummarizeSessionTasks(r.Context(), sessionIDs)
	if err != nil {
		h.logger.Warn("failed to summarize session tasks: %v", err)
		taskSummaries = map[string]app.SessionTaskSummary{}
	}

	sessions := make([]SessionResponse, 0, len(sessionItems))
	for _, item := range sessionItems {
		summary := taskSummaries[item.ID]

		sessions = append(sessions, SessionResponse{
			ID:        item.ID,
			Title:     item.Title,
			CreatedAt: item.CreatedAt.Format(time.RFC3339),
			UpdatedAt: item.UpdatedAt.Format(time.RFC3339),
			TaskCount: summary.TaskCount,
			LastTask:  summary.LastTask,
		})
	}

	response := SessionListResponse{
		Sessions: sessions,
		Total:    len(sessions),
	}
	h.writeJSON(w, http.StatusOK, response)
}

// HandleDeleteSession handles DELETE /api/sessions/{session_id}
func (h *APIHandler) HandleDeleteSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := extractRequiredSessionIDFromPath(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	if err := h.sessions.DeleteSession(r.Context(), sessionID); err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to delete session")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleListSnapshots handles GET /api/sessions/{session_id}/snapshots
func (h *APIHandler) HandleListSnapshots(w http.ResponseWriter, r *http.Request) {
	sessionID, err := extractRequiredSessionIDFromPath(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	limit, ok := h.parseOptionalQueryInt(
		w,
		r,
		"limit",
		20,
		1,
		maxSnapshotListLimit,
		"limit must be a positive integer",
		nil,
	)
	if !ok {
		return
	}

	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	items, nextCursor, err := h.snapshots.ListSnapshots(r.Context(), sessionID, cursor, limit)
	if err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to list snapshots")
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
	h.writeJSON(w, http.StatusOK, resp)
}

// HandleGetTurnSnapshot handles GET /api/sessions/{session_id}/turns/{turn_id}
func (h *APIHandler) HandleGetTurnSnapshot(w http.ResponseWriter, r *http.Request) {
	sessionID, err := extractRequiredSessionIDFromPath(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}
	turnID, err := strconv.Atoi(r.PathValue("turn_id"))
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "turn_id must be numeric", err)
		return
	}
	snapshot, err := h.snapshots.GetSnapshot(r.Context(), sessionID, turnID)
	if err != nil {
		h.writeMappedError(w, err, http.StatusNotFound, "Snapshot not found")
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
	h.writeJSON(w, http.StatusOK, resp)
}

// HandleForkSession handles POST /api/sessions/{session_id}/fork
func (h *APIHandler) HandleForkSession(w http.ResponseWriter, r *http.Request) {
	sessionID, err := extractRequiredSessionIDFromPath(r)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	newSession, err := h.sessions.ForkSession(r.Context(), sessionID)
	if err != nil {
		h.writeMappedError(w, err, http.StatusInternalServerError, "Failed to fork session")
		return
	}
	h.writeJSON(w, http.StatusCreated, newSession)
}
