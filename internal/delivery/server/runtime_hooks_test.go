package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/shared/logging"
)

var nopLog = logging.OrNop(nil)

type captureBus struct {
	events []hooks.Event
}

func (b *captureBus) Publish(sessionID string, ev hooks.Event) {
	b.events = append(b.events, ev)
}

func TestRuntimeHooksHandler_PostToolUse(t *testing.T) {
	bus := &captureBus{}
	h := NewRuntimeHooksHandler(bus, nopLog)

	body := `{"hook_event_name":"PostToolUse","tool_name":"Bash"}`
	req := httptest.NewRequest(http.MethodPost, "/api/hooks/runtime?session_id=rs-test1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(bus.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(bus.events))
	}
	ev := bus.events[0]
	if ev.Type != hooks.EventHeartbeat {
		t.Fatalf("expected heartbeat, got %s", ev.Type)
	}
	if ev.SessionID != "rs-test1" {
		t.Fatalf("expected session rs-test1, got %s", ev.SessionID)
	}
}

func TestRuntimeHooksHandler_StopSuccess(t *testing.T) {
	bus := &captureBus{}
	h := NewRuntimeHooksHandler(bus, nopLog)

	body := `{"hook_event_name":"Stop","stop_reason":"end_turn","answer":"task done"}`
	req := httptest.NewRequest(http.MethodPost, "/api/hooks/runtime?session_id=rs-test2", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(bus.events) != 1 || bus.events[0].Type != hooks.EventCompleted {
		t.Fatalf("expected completed event, got %+v", bus.events)
	}
}

func TestRuntimeHooksHandler_StopError(t *testing.T) {
	bus := &captureBus{}
	h := NewRuntimeHooksHandler(bus, nopLog)

	body := `{"hook_event_name":"Stop","error":"something went wrong","stop_reason":"error"}`
	req := httptest.NewRequest(http.MethodPost, "/api/hooks/runtime?session_id=rs-test3", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(bus.events) != 1 || bus.events[0].Type != hooks.EventFailed {
		t.Fatalf("expected failed event, got %+v", bus.events)
	}
}

func TestRuntimeHooksHandler_MissingSessionID(t *testing.T) {
	bus := &captureBus{}
	h := NewRuntimeHooksHandler(bus, nopLog)

	req := httptest.NewRequest(http.MethodPost, "/api/hooks/runtime", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRuntimeHooksHandler_WrongMethod(t *testing.T) {
	bus := &captureBus{}
	h := NewRuntimeHooksHandler(bus, nopLog)

	req := httptest.NewRequest(http.MethodGet, "/api/hooks/runtime?session_id=x", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// Ensure the handler sets At to approximately now.
func TestRuntimeHooksHandler_EventTimestamp(t *testing.T) {
	bus := &captureBus{}
	h := NewRuntimeHooksHandler(bus, nopLog)

	before := time.Now()
	body := `{"hook_event_name":"PostToolUse"}`
	req := httptest.NewRequest(http.MethodPost, "/api/hooks/runtime?session_id=ts-test", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	after := time.Now()

	if len(bus.events) != 1 {
		t.Fatal("expected one event")
	}
	ev := bus.events[0]
	if ev.At.Before(before) || ev.At.After(after) {
		t.Fatalf("event timestamp %v not in [%v, %v]", ev.At, before, after)
	}
}
