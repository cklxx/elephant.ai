package lark

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBatchCreateEvents_AllSuccess(t *testing.T) {
	cnt := &counter{}
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars") {
			if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"has_more":   false,
				"page_token": "",
				"calendar_list": []map[string]interface{}{
					{"calendar_id": "cal-primary", "type": "primary", "role": "owner"},
				},
			})); err != nil {
				t.Fatalf("write response: %v", err)
			}
			return
		}

		n := cnt.next()
		ev := calendarEventJSON(
			fmt.Sprintf("ev-%d", n),
			fmt.Sprintf("Event %d", n),
			base().Unix(),
			base().Add(1*time.Hour).Unix(),
		)
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{"event": ev})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	events := []CreateEventRequest{
		{Summary: "Event 1", StartTime: base(), EndTime: base().Add(1 * time.Hour)},
		{Summary: "Event 2", StartTime: base().Add(2 * time.Hour), EndTime: base().Add(3 * time.Hour)},
		{Summary: "Event 3", StartTime: base().Add(4 * time.Hour), EndTime: base().Add(5 * time.Hour)},
	}

	results, errs := client.Calendar().BatchCreateEvents(context.Background(), "primary", events)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if len(errs) != 3 {
		t.Fatalf("expected 3 error slots, got %d", len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("event[%d]: unexpected error: %v", i, err)
		}
	}
	for i, ev := range results {
		if ev.EventID == "" {
			t.Errorf("event[%d]: expected non-empty EventID", i)
		}
	}
}

func TestBatchCreateEvents_MixedSuccessFailure(t *testing.T) {
	cnt := &counter{}
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		n := cnt.next()
		w.Header().Set("Content-Type", "application/json")
		if n == 2 {
			// Simulate failure on the second event.
			if _, err := w.Write(jsonResponse(400100, "invalid event", nil)); err != nil {
				t.Fatalf("write response: %v", err)
			}
			return
		}
		ev := calendarEventJSON(
			fmt.Sprintf("ev-%d", n),
			fmt.Sprintf("Event %d", n),
			base().Unix(),
			base().Add(1*time.Hour).Unix(),
		)
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{"event": ev})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	events := []CreateEventRequest{
		{Summary: "Good Event 1", StartTime: base(), EndTime: base().Add(1 * time.Hour)},
		{Summary: "Bad Event", StartTime: base().Add(2 * time.Hour), EndTime: base().Add(3 * time.Hour)},
		{Summary: "Good Event 2", StartTime: base().Add(4 * time.Hour), EndTime: base().Add(5 * time.Hour)},
	}

	results, errs := client.Calendar().BatchCreateEvents(context.Background(), "cal-123", events)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// First event: success.
	if errs[0] != nil {
		t.Errorf("event[0]: unexpected error: %v", errs[0])
	}
	if results[0].EventID == "" {
		t.Error("event[0]: expected non-empty EventID")
	}

	// Second event: failure.
	if errs[1] == nil {
		t.Error("event[1]: expected error, got nil")
	}
	if results[1].EventID != "" {
		t.Errorf("event[1]: expected empty EventID on failure, got %q", results[1].EventID)
	}

	// Third event: success.
	if errs[2] != nil {
		t.Errorf("event[2]: unexpected error: %v", errs[2])
	}
	if results[2].EventID == "" {
		t.Error("event[2]: expected non-empty EventID")
	}
}

func TestBatchCreateEvents_Empty(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for empty batch")
	})
	defer srv.Close()

	results, errs := client.Calendar().BatchCreateEvents(context.Background(), "primary", nil)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestBatchDeleteEvents_AllSuccess(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars") {
			if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"has_more":   false,
				"page_token": "",
				"calendar_list": []map[string]interface{}{
					{"calendar_id": "cal-primary", "type": "primary", "role": "owner"},
				},
			})); err != nil {
				t.Fatalf("write response: %v", err)
			}
			return
		}
		if _, err := w.Write(jsonResponse(0, "ok", nil)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	errs := client.Calendar().BatchDeleteEvents(context.Background(), "primary", []string{"ev-1", "ev-2", "ev-3"})
	if len(errs) != 3 {
		t.Fatalf("expected 3 error slots, got %d", len(errs))
	}
	for i, err := range errs {
		if err != nil {
			t.Errorf("event[%d]: unexpected error: %v", i, err)
		}
	}
}

func TestBatchDeleteEvents_MixedSuccessFailure(t *testing.T) {
	cnt := &counter{}
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars") {
			if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"has_more":   false,
				"page_token": "",
				"calendar_list": []map[string]interface{}{
					{"calendar_id": "cal-primary", "type": "primary", "role": "owner"},
				},
			})); err != nil {
				t.Fatalf("write response: %v", err)
			}
			return
		}

		n := cnt.next()
		if n == 2 {
			if _, err := w.Write(jsonResponse(404001, "event not found", nil)); err != nil {
				t.Fatalf("write response: %v", err)
			}
			return
		}
		if _, err := w.Write(jsonResponse(0, "ok", nil)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	errs := client.Calendar().BatchDeleteEvents(context.Background(), "primary", []string{"ev-1", "ev-not-found", "ev-3"})

	if errs[0] != nil {
		t.Errorf("event[0]: unexpected error: %v", errs[0])
	}
	if errs[1] == nil {
		t.Error("event[1]: expected error for not-found event, got nil")
	}
	if errs[2] != nil {
		t.Errorf("event[2]: unexpected error: %v", errs[2])
	}
}

func TestBatchDeleteEvents_Empty(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("server should not be called for empty batch")
	})
	defer srv.Close()

	errs := client.Calendar().BatchDeleteEvents(context.Background(), "primary", nil)
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestBatchCreateEvents_AllFailure(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars") {
			if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"has_more":   false,
				"page_token": "",
				"calendar_list": []map[string]interface{}{
					{"calendar_id": "cal-primary", "type": "primary", "role": "owner"},
				},
			})); err != nil {
				t.Fatalf("write response: %v", err)
			}
			return
		}
		if _, err := w.Write(jsonResponse(500000, "internal error", nil)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	events := []CreateEventRequest{
		{Summary: "Fail 1", StartTime: base(), EndTime: base().Add(1 * time.Hour)},
		{Summary: "Fail 2", StartTime: base().Add(1 * time.Hour), EndTime: base().Add(2 * time.Hour)},
	}

	results, errs := client.Calendar().BatchCreateEvents(context.Background(), "primary", events)

	for i, err := range errs {
		if err == nil {
			t.Errorf("event[%d]: expected error, got nil", i)
		}
	}
	for i, ev := range results {
		if ev.EventID != "" {
			t.Errorf("event[%d]: expected zero-value on failure, got EventID=%q", i, ev.EventID)
		}
	}
}
