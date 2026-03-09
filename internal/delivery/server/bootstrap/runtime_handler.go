package bootstrap

import (
	"encoding/json"
	"net/http"
	"strings"

	"alex/internal/runtime"
	"alex/internal/runtime/pool"
	"alex/internal/runtime/session"
	"alex/internal/shared/logging"
)

// RuntimeSessionHandler exposes the Runtime as an HTTP handler.
//
//	POST /api/runtime/sessions        — create + start a session
//	GET  /api/runtime/sessions        — list all sessions
//	GET  /api/runtime/sessions/{id}   — get a single session
type RuntimeSessionHandler struct {
	rt     *runtime.Runtime
	logger logging.Logger
}

// NewRuntimeSessionHandler creates a RuntimeSessionHandler.
func NewRuntimeSessionHandler(rt *runtime.Runtime, logger logging.Logger) *RuntimeSessionHandler {
	return &RuntimeSessionHandler{rt: rt, logger: logging.OrNop(logger)}
}

// ServeHTTP dispatches to the appropriate handler based on method and path.
func (h *RuntimeSessionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Strip prefix to isolate the path suffix after /api/runtime/sessions.
	path := strings.TrimPrefix(r.URL.Path, "/api/runtime/sessions")
	path = strings.TrimPrefix(path, "/")

	switch {
	case r.Method == http.MethodPost && path == "":
		h.handleCreate(w, r)
	case r.Method == http.MethodGet && path == "":
		h.handleList(w, r)
	case r.Method == http.MethodGet && path != "":
		h.handleGet(w, r, path)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// createRequest is the body for POST /api/runtime/sessions.
//
// parent_pane_id semantics:
//   - omitted / null  → 400 Bad Request (callers must be explicit)
//   - -1              → no pane split (pool mode if pool is configured, otherwise tracking-only)
//   - >0              → split a new pane from that pane ID and launch CC there
type createRequest struct {
	Member          string `json:"member"`
	Goal            string `json:"goal"`
	WorkDir         string `json:"work_dir"`
	ParentPaneID    *int   `json:"parent_pane_id"`
	ParentSessionID string `json:"parent_session_id,omitempty"`
}

func (h *RuntimeSessionHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// parent_pane_id is mandatory — callers must be explicit.
	// Use -1 to intentionally skip pane creation; omitting the field is an error.
	if req.ParentPaneID == nil {
		h.logger.Warn("runtime_handler: create request missing parent_pane_id (member=%s goal=%q)", req.Member, req.Goal)
		http.Error(w, "parent_pane_id required (use -1 to disable pane creation)", http.StatusBadRequest)
		return
	}
	parentPaneID := *req.ParentPaneID
	h.logger.Info("runtime_handler: create session member=%s parent_pane_id=%d parent_session=%s goal=%q",
		req.Member, parentPaneID, req.ParentSessionID, req.Goal)

	member := session.MemberType(req.Member)
	if member == "" {
		member = session.MemberClaudeCode
	}

	s, err := h.rt.CreateSession(member, req.Goal, req.WorkDir, req.ParentSessionID)
	if err != nil {
		h.logger.Warn("runtime_handler: create session: %v", err)
		http.Error(w, "create session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := h.rt.StartSession(r.Context(), s.ID, parentPaneID); err != nil {
		h.logger.Warn("runtime_handler: start session %s: %v", s.ID, err)
		http.Error(w, "start session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	snap := s.Snapshot()
	writeJSON(w, http.StatusCreated, snap)
}

func (h *RuntimeSessionHandler) handleList(w http.ResponseWriter, r *http.Request) {
	sessions := h.rt.ListSessions()
	writeJSON(w, http.StatusOK, sessions)
}

func (h *RuntimeSessionHandler) handleGet(w http.ResponseWriter, r *http.Request, id string) {
	snap, ok := h.rt.GetSession(id)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

// RuntimePoolHandler exposes the PanePool as an HTTP handler.
//
//	POST /api/runtime/pool  — register pane IDs into the pool
//	GET  /api/runtime/pool  — list all pool slots with their state
type RuntimePoolHandler struct {
	rt     *runtime.Runtime
	logger logging.Logger
}

// NewRuntimePoolHandler creates a RuntimePoolHandler.
func NewRuntimePoolHandler(rt *runtime.Runtime, logger logging.Logger) *RuntimePoolHandler {
	return &RuntimePoolHandler{rt: rt, logger: logging.OrNop(logger)}
}

// ServeHTTP dispatches to register or status based on method.
func (h *RuntimePoolHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		h.handleRegister(w, r)
	case http.MethodGet:
		h.handleStatus(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type poolRegisterRequest struct {
	PaneIDs []int `json:"pane_ids"`
}

func (h *RuntimePoolHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req poolRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(req.PaneIDs) == 0 {
		http.Error(w, "pane_ids required", http.StatusBadRequest)
		return
	}

	p := h.rt.Pool()
	if p == nil {
		http.Error(w, "pool not configured", http.StatusServiceUnavailable)
		return
	}

	added := p.Register(req.PaneIDs)
	h.logger.Info("runtime_pool: registered %d panes (requested %d)", added, len(req.PaneIDs))
	writeJSON(w, http.StatusOK, map[string]any{
		"registered": added,
		"total":      p.Size(),
	})
}

func (h *RuntimePoolHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	p := h.rt.Pool()
	if p == nil {
		writeJSON(w, http.StatusOK, []pool.Slot{})
		return
	}
	writeJSON(w, http.StatusOK, p.Slots())
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
