package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestResolveCalendarID_Passthrough(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected API call: %s", r.URL.Path)
	})
	defer srv.Close()

	got, err := client.Calendar().ResolveCalendarID(context.Background(), "cal-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cal-123" {
		t.Fatalf("calendar id = %q, want %q", got, "cal-123")
	}
}

func TestResolveCalendarID_PrimaryPicksTypePrimary(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"has_more":   false,
			"page_token": "",
			"calendar_list": []map[string]interface{}{
				{"calendar_id": "cal-primary", "type": "primary", "role": "owner"},
				{"calendar_id": "cal-other", "type": "shared", "role": "owner"},
			},
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	got, err := client.Calendar().ResolveCalendarID(context.Background(), "primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cal-primary" {
		t.Fatalf("calendar id = %q, want %q", got, "cal-primary")
	}
}

func TestResolveCalendarID_PrimaryFallbackOwner(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"has_more":   false,
			"page_token": "",
			"calendar_list": []map[string]interface{}{
				{"calendar_id": "cal-reader", "type": "shared", "role": "reader"},
				{"calendar_id": "cal-owner", "type": "shared", "role": "owner"},
			},
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	got, err := client.Calendar().ResolveCalendarID(context.Background(), "primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cal-owner" {
		t.Fatalf("calendar id = %q, want %q", got, "cal-owner")
	}
}

func TestResolveCalendarID_PrimaryFallbackFirst(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"has_more":   false,
			"page_token": "",
			"calendar_list": []map[string]interface{}{
				{"calendar_id": "cal-1", "type": "shared", "role": "reader"},
				{"calendar_id": "cal-2", "type": "shared", "role": "reader"},
			},
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	got, err := client.Calendar().ResolveCalendarID(context.Background(), "primary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "cal-1" {
		t.Fatalf("calendar id = %q, want %q", got, "cal-1")
	}
}

func TestResolveCalendarID_PrimaryNoCalendars(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"has_more":      false,
			"page_token":    "",
			"calendar_list": []map[string]interface{}{},
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	_, err := client.Calendar().ResolveCalendarID(context.Background(), "primary")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
