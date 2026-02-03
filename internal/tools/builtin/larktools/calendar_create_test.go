package larktools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/shared"
	"alex/internal/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func jsonResponse(code int, msg string, data interface{}) []byte {
	resp := map[string]interface{}{
		"code": code,
		"msg":  msg,
	}
	if data != nil {
		resp["data"] = data
	}
	b, _ := json.Marshal(resp)
	return b
}

func tokenResponse(token string, expire int) []byte {
	resp := map[string]interface{}{
		"code":                0,
		"msg":                 "ok",
		"tenant_access_token": token,
		"app_access_token":    token,
		"expire":              expire,
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestCalendarCreate_NoLarkClient(t *testing.T) {
	tool := NewLarkCalendarCreate()
	call := ports.ToolCall{ID: "test-1", Name: "lark_calendar_create"}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when lark client is missing")
	}
}

func TestCalendarCreate_InvalidClientType(t *testing.T) {
	tool := NewLarkCalendarCreate()
	ctx := shared.WithLarkClient(context.Background(), "not-a-lark-client")
	call := ports.ToolCall{ID: "test-2", Name: "lark_calendar_create"}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid client type")
	}
}

func TestCalendarCreate_MissingSummary(t *testing.T) {
	tool := NewLarkCalendarCreate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-3", Name: "lark_calendar_create", Arguments: map[string]any{
		"calendar_id": "cal_123",
		"start_time":  "1700000000",
		"end_time":    "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing summary")
	}
}

func TestCalendarCreate_PrimaryCalendarIDResolves(t *testing.T) {
	var mu sync.Mutex
	var gotCalendarID string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle token requests so the SDK can authenticate.
		if strings.Contains(r.URL.Path, "tenant_access_token") || strings.Contains(r.URL.Path, "app_access_token") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(tokenResponse("test-token", 7200))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars/primarys"):
			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"calendars": []map[string]interface{}{
					{
						"user_id": "ou_123",
						"calendar": map[string]interface{}{
							"calendar_id": "cal-primary",
							"type":        "primary",
							"role":        "owner",
						},
					},
				},
			}))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/calendar/v4/calendars/") && strings.Contains(r.URL.Path, "/events"):
			parts := strings.Split(r.URL.Path, "/calendar/v4/calendars/")
			if len(parts) != 2 {
				t.Fatalf("unexpected create path: %s", r.URL.Path)
			}
			rest := parts[1]
			calID := strings.TrimSuffix(rest, "/events")
			mu.Lock()
			gotCalendarID = calID
			mu.Unlock()

			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"event": map[string]interface{}{
					"event_id": "evt_123",
				},
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkCalendarCreate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)

	call := ports.ToolCall{ID: "test-4", Name: "lark_calendar_create", Arguments: map[string]any{
		"calendar_id": "primary",
		"summary":     "Test",
		"start_time":  "1700000000",
		"end_time":    "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if result.Metadata["calendar_id"] != "cal-primary" {
		t.Fatalf("metadata calendar_id = %v, want %q", result.Metadata["calendar_id"], "cal-primary")
	}
	mu.Lock()
	defer mu.Unlock()
	if gotCalendarID != "cal-primary" {
		t.Fatalf("create used calendar_id=%q, want %q", gotCalendarID, "cal-primary")
	}
}

func TestCalendarCreate_PrimaryCalendarIDResolvesForOwner(t *testing.T) {
	var mu sync.Mutex
	var gotCalendarID string
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handle token requests so the SDK can authenticate.
		if strings.Contains(r.URL.Path, "tenant_access_token") || strings.Contains(r.URL.Path, "app_access_token") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(tokenResponse("test-token", 7200))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars/primarys"):
			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"calendars": []map[string]interface{}{
					{
						"user_id": "ou_target",
						"calendar": map[string]interface{}{
							"calendar_id": "cal-target",
							"type":        "primary",
							"role":        "owner",
						},
					},
				},
			}))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/calendar/v4/calendars/") && strings.Contains(r.URL.Path, "/events"):
			parts := strings.Split(r.URL.Path, "/calendar/v4/calendars/")
			if len(parts) != 2 {
				t.Fatalf("unexpected create path: %s", r.URL.Path)
			}
			rest := parts[1]
			calID := strings.TrimSuffix(rest, "/events")
			mu.Lock()
			gotCalendarID = calID
			gotAuth = r.Header.Get("Authorization")
			mu.Unlock()

			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"event": map[string]interface{}{
					"event_id": "evt_123",
				},
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkCalendarCreate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_sender"), larkClient)

	call := ports.ToolCall{ID: "test-5", Name: "lark_calendar_create", Arguments: map[string]any{
		"calendar_id":            "primary",
		"calendar_owner_id":      "ou_target",
		"calendar_owner_id_type": "open_id",
		"user_access_token":      "user-token",
		"summary":                "Test",
		"start_time":             "1700000000",
		"end_time":               "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if result.Metadata["calendar_id"] != "cal-target" {
		t.Fatalf("metadata calendar_id = %v, want %q", result.Metadata["calendar_id"], "cal-target")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotCalendarID != "cal-target" {
		t.Fatalf("create used calendar_id=%q, want %q", gotCalendarID, "cal-target")
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("expected tenant token auth, got %q", gotAuth)
	}
}
