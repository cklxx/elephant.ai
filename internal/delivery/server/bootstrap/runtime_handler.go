package bootstrap

import (
	"encoding/json"
	"net/http"
	"strings"

	"alex/internal/runtime"
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
type createRequest struct {
	Member       string `json:"member"`
	Goal         string `json:"goal"`
	WorkDir      string `json:"work_dir"`
	ParentPaneID int    `json:"parent_pane_id"`
}

func (h *RuntimeSessionHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	member := session.MemberType(req.Member)
	if member == "" {
		member = session.MemberClaudeCode
	}

	s, err := h.rt.CreateSession(member, req.Goal, req.WorkDir)
	if err != nil {
		h.logger.Warn("runtime_handler: create session: %v", err)
		http.Error(w, "create session: "+err.Error(), http.StatusInternalServerError)
		return
	}

	parentPaneID := req.ParentPaneID
	if parentPaneID == 0 {
		parentPaneID = -1 // default: no pane split
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
