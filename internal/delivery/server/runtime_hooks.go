package server

import (
	"io"
	"net/http"
	"strings"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/shared/logging"
)

// RuntimeBus is the event bus the RuntimeHooksHandler publishes to.
// It is satisfied by *hooks.InProcessBus and by test doubles.
type RuntimeBus interface {
	Publish(sessionID string, ev hooks.Event)
}

// RuntimeHooksHandler handles POST /api/hooks/runtime requests.
// Claude Code hooks (PostToolUse, Stop) posted by notify_runtime.sh are
// decoded using the shared decodeHookPayload helper and translated into
// structured events on the runtime event bus.
//
// This handler is intentionally simple: it publishes one event per request
// with no aggregation or deduplication (unlike HooksBridge which batches
// tool events for Lark).
type RuntimeHooksHandler struct {
	bus    RuntimeBus
	logger logging.Logger
}

// NewRuntimeHooksHandler creates a RuntimeHooksHandler.
func NewRuntimeHooksHandler(bus RuntimeBus, logger logging.Logger) *RuntimeHooksHandler {
	return &RuntimeHooksHandler{bus: bus, logger: logging.OrNop(logger)}
}

// ServeHTTP handles POST /api/hooks/runtime?session_id=<id>.
func (h *RuntimeHooksHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := strings.TrimSpace(r.URL.Query().Get("session_id"))
	if sessionID == "" {
		http.Error(w, "missing session_id query parameter", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	payload, err := decodeHookPayload(body)
	if err != nil {
		h.logger.Warn("Runtime hooks: invalid json payload: %v", err)
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	ev := h.translate(sessionID, payload)
	if ev.Type == "" {
		// Unknown event type — acknowledge but ignore.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.bus.Publish(sessionID, ev)
	w.WriteHeader(http.StatusOK)
}

// translate converts a decoded hook payload into a runtime Event.
func (h *RuntimeHooksHandler) translate(sessionID string, p hookPayload) hooks.Event {
	now := time.Now()

	switch p.Event {
	case "PostToolUse", "PreToolUse":
		return hooks.Event{
			Type:      hooks.EventHeartbeat,
			SessionID: sessionID,
			At:        now,
			Payload:   map[string]any{"tool_name": p.ToolName},
		}
	case "Stop":
		if p.Error != "" || strings.EqualFold(p.StopReason, "error") {
			return hooks.Event{
				Type:      hooks.EventFailed,
				SessionID: sessionID,
				At:        now,
				Payload:   map[string]any{"error": p.Error, "stop_reason": p.StopReason},
			}
		}
		return hooks.Event{
			Type:      hooks.EventCompleted,
			SessionID: sessionID,
			At:        now,
			Payload:   map[string]any{"answer": p.Answer, "stop_reason": p.StopReason},
		}
	default:
		return hooks.Event{}
	}
}
