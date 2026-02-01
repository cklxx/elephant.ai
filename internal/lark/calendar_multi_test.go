package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestListCalendars_SinglePage(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		hasMore := false
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"has_more":   hasMore,
			"page_token": "",
			"calendar_list": []map[string]interface{}{
				{
					"calendar_id": "cal-primary",
					"summary":     "My Calendar",
					"description": "Primary calendar",
					"type":        "primary",
					"role":        "owner",
				},
				{
					"calendar_id": "cal-shared",
					"summary":     "Team Calendar",
					"description": "Shared team calendar",
					"type":        "shared",
					"role":        "reader",
				},
			},
		})); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
	defer srv.Close()

	calendars, err := client.Calendar().ListCalendars(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calendars) != 2 {
		t.Fatalf("expected 2 calendars, got %d", len(calendars))
	}

	tests := []struct {
		idx         int
		wantID      string
		wantSummary string
		wantType    string
		wantRole    string
	}{
		{0, "cal-primary", "My Calendar", "primary", "owner"},
		{1, "cal-shared", "Team Calendar", "shared", "reader"},
	}
	for _, tt := range tests {
		cal := calendars[tt.idx]
		if cal.ID != tt.wantID {
			t.Errorf("cal[%d].ID = %q, want %q", tt.idx, cal.ID, tt.wantID)
		}
		if cal.Summary != tt.wantSummary {
			t.Errorf("cal[%d].Summary = %q, want %q", tt.idx, cal.Summary, tt.wantSummary)
		}
		if cal.Type != tt.wantType {
			t.Errorf("cal[%d].Type = %q, want %q", tt.idx, cal.Type, tt.wantType)
		}
		if cal.Role != tt.wantRole {
			t.Errorf("cal[%d].Role = %q, want %q", tt.idx, cal.Role, tt.wantRole)
		}
	}
}

func TestListCalendars_MultiPage(t *testing.T) {
	cnt := &counter{}
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		n := cnt.next()
		w.Header().Set("Content-Type", "application/json")

		switch n {
		case 1:
			// First page: return 1 calendar with has_more=true.
			hasMore := true
			if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"has_more":   hasMore,
				"page_token": "page-2",
				"calendar_list": []map[string]interface{}{
					{
						"calendar_id": "cal-1",
						"summary":     "Calendar One",
						"type":        "primary",
					},
				},
			})); err != nil {
				t.Errorf("write response: %v", err)
			}
		default:
			// Second page: return 1 calendar with has_more=false.
			hasMore := false
			if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"has_more":   hasMore,
				"page_token": "",
				"calendar_list": []map[string]interface{}{
					{
						"calendar_id": "cal-2",
						"summary":     "Calendar Two",
						"type":        "shared",
					},
				},
			})); err != nil {
				t.Errorf("write response: %v", err)
			}
		}
	})
	defer srv.Close()

	calendars, err := client.Calendar().ListCalendars(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calendars) != 2 {
		t.Fatalf("expected 2 calendars across 2 pages, got %d", len(calendars))
	}
	if calendars[0].ID != "cal-1" {
		t.Errorf("first calendar ID = %q, want %q", calendars[0].ID, "cal-1")
	}
	if calendars[1].ID != "cal-2" {
		t.Errorf("second calendar ID = %q, want %q", calendars[1].ID, "cal-2")
	}
}

func TestListCalendars_Empty(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		hasMore := false
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"has_more":      hasMore,
			"page_token":    "",
			"calendar_list": []map[string]interface{}{},
		})); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
	defer srv.Close()

	calendars, err := client.Calendar().ListCalendars(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calendars) != 0 {
		t.Fatalf("expected 0 calendars, got %d", len(calendars))
	}
}

func TestListCalendars_APIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(403001, "no permission", nil)); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
	defer srv.Close()

	_, err := client.Calendar().ListCalendars(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 403001 {
		t.Errorf("error code = %d, want 403001", apiErr.Code)
	}
}

func TestListCalendarsPage_Basic(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		hasMore := true
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"has_more":   hasMore,
			"page_token": "next-page",
			"calendar_list": []map[string]interface{}{
				{
					"calendar_id": "cal-1",
					"summary":     "Calendar One",
					"description": "First",
					"type":        "primary",
					"role":        "owner",
				},
			},
		})); err != nil {
			t.Errorf("write response: %v", err)
		}
	})
	defer srv.Close()

	resp, err := client.Calendar().ListCalendarsPage(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.HasMore {
		t.Error("expected HasMore=true")
	}
	if resp.PageToken != "next-page" {
		t.Errorf("PageToken = %q, want %q", resp.PageToken, "next-page")
	}
	if len(resp.Calendars) != 1 {
		t.Fatalf("expected 1 calendar, got %d", len(resp.Calendars))
	}
	if resp.Calendars[0].ID != "cal-1" {
		t.Errorf("calendar ID = %q, want %q", resp.Calendars[0].ID, "cal-1")
	}
}

func TestParseCalendarInfo_Nil(t *testing.T) {
	info := parseCalendarInfo(nil)
	if info.ID != "" || info.Summary != "" || info.Type != "" {
		t.Errorf("expected zero CalendarInfo from nil, got %+v", info)
	}
}
