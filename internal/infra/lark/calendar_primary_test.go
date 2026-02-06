package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestPrimaryCalendarIDs_Basic(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/open-apis/calendar/v4/calendars/primarys" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("user_id_type"); got != "open_id" {
			t.Fatalf("user_id_type=%q, want %q", got, "open_id")
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"calendars": []map[string]interface{}{
				{
					"user_id": "ou_1",
					"calendar": map[string]interface{}{
						"calendar_id": "cal_1",
						"type":        "primary",
						"role":        "owner",
					},
				},
				{
					"user_id": "ou_2",
					"calendar": map[string]interface{}{
						"calendar_id": "cal_2",
						"type":        "primary",
						"role":        "owner",
					},
				},
			},
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	got, err := client.Calendar().PrimaryCalendarIDs(context.Background(), "open_id", []string{"ou_1", "ou_2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["ou_1"] != "cal_1" {
		t.Fatalf("ou_1=%q, want %q", got["ou_1"], "cal_1")
	}
	if got["ou_2"] != "cal_2" {
		t.Fatalf("ou_2=%q, want %q", got["ou_2"], "cal_2")
	}
}

func TestPrimaryCalendarID_NotFound(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"calendars": []map[string]interface{}{},
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	_, err := client.Calendar().PrimaryCalendarID(context.Background(), "open_id", "ou_missing")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}
