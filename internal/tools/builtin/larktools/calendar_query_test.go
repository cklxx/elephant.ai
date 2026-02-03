package larktools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"alex/internal/agent/ports"
	larkoauth "alex/internal/lark/oauth"
	"alex/internal/tools/builtin/shared"
	"alex/internal/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestCalendarQuery_NoLarkClient(t *testing.T) {
	tool := NewLarkCalendarQuery()
	call := ports.ToolCall{ID: "test-1", Name: "lark_calendar_query"}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when lark client is missing")
	}
}

func TestCalendarQuery_InvalidClientType(t *testing.T) {
	tool := NewLarkCalendarQuery()
	ctx := shared.WithLarkClient(context.Background(), "not-a-lark-client")
	call := ports.ToolCall{ID: "test-2", Name: "lark_calendar_query"}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid client type")
	}
}

func TestCalendarQuery_MissingOAuthToken_ShowsAuthURL(t *testing.T) {
	tool := NewLarkCalendarQuery()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	oauthSvc := &fakeLarkOAuth{
		err:      &larkoauth.NeedUserAuthError{AuthURL: "http://localhost:8080/api/lark/oauth/start"},
		startURL: "http://localhost:8080/api/lark/oauth/start",
	}
	ctx := id.WithUserID(context.Background(), "ou_123")
	ctx = shared.WithLarkClient(ctx, larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-3", Name: "lark_calendar_query", Arguments: map[string]any{
		"start_time": "1700000000",
		"end_time":   "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing OAuth token")
	}
	if !strings.Contains(result.Content, oauthSvc.startURL) {
		t.Fatalf("expected auth url in content, got %q", result.Content)
	}
}

func TestCalendarQuery_UsesTenantTokenWhenConfigured(t *testing.T) {
	var mu sync.Mutex
	var listAuth string
	var queryAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/auth/v3/tenant_access_token/internal"):
			_, _ = w.Write(tokenResponse("app-token", 7200))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/auth/v3/app_access_token/internal"):
			_, _ = w.Write(tokenResponse("app-token", 7200))
			return
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars"):
			mu.Lock()
			listAuth = r.Header.Get("Authorization")
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"calendar_list": []map[string]interface{}{
					{
						"calendar_id": "cal-primary",
						"type":        "primary",
						"role":        "owner",
					},
				},
				"has_more": false,
			}))
			return
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/calendar/v4/calendars/") && strings.HasSuffix(r.URL.Path, "/events"):
			mu.Lock()
			queryAuth = r.Header.Get("Authorization")
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"items":    []map[string]interface{}{},
				"has_more": false,
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkCalendarQuery()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	ctx = shared.WithLarkTenantToken(ctx, "tenant-token")

	call := ports.ToolCall{ID: "test-tenant", Name: "lark_calendar_query", Arguments: map[string]any{
		"start_time": "1700000000",
		"end_time":   "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	mu.Lock()
	defer mu.Unlock()
	if listAuth != "Bearer tenant-token" {
		t.Fatalf("expected tenant token for list calendars, got %q", listAuth)
	}
	if queryAuth != "Bearer tenant-token" {
		t.Fatalf("expected tenant token for list events, got %q", queryAuth)
	}
}

func TestCalendarQuery_PrimaryCalendarIDResolves(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars"):
			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"calendar_list": []map[string]interface{}{
					{
						"calendar_id": "cal-primary",
						"type":        "primary",
						"role":        "owner",
					},
				},
				"has_more": false,
			}))
			return
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/calendar/v4/calendars/") && strings.HasSuffix(r.URL.Path, "/events"):
			gotAuth = r.Header.Get("Authorization")
			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"items": []map[string]interface{}{
					{
						"event_id": "evt_1",
						"summary":  "Hello",
						"start_time": map[string]interface{}{
							"timestamp": "1700000000",
						},
						"end_time": map[string]interface{}{
							"timestamp": "1700003600",
						},
						"organizer_calendar_id": "cal-primary",
						"status":                "confirmed",
					},
				},
				"has_more": false,
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkCalendarQuery()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	oauthSvc := &fakeLarkOAuth{token: "user-token", startURL: "http://localhost:8080/api/lark/oauth/start"}
	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-4", Name: "lark_calendar_query", Arguments: map[string]any{
		"start_time": "1700000000",
		"end_time":   "1700003600",
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
	if gotAuth != "Bearer user-token" {
		t.Fatalf("expected user token auth, got %q", gotAuth)
	}
}

func TestCalendarQuery_MissingStartTime(t *testing.T) {
	tool := NewLarkCalendarQuery()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-4", Name: "lark_calendar_query", Arguments: map[string]any{
		"end_time": "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing start_time")
	}
}
