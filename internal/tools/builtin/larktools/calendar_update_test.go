package larktools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	larkoauth "alex/internal/lark/oauth"
	"alex/internal/tools/builtin/shared"
	"alex/internal/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestCalendarUpdate_NoLarkClient(t *testing.T) {
	tool := NewLarkCalendarUpdate()
	call := ports.ToolCall{ID: "test-1", Name: "lark_calendar_update"}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when lark client is missing")
	}
}

func TestCalendarUpdate_InvalidClientType(t *testing.T) {
	tool := NewLarkCalendarUpdate()
	ctx := shared.WithLarkClient(context.Background(), "not-a-lark-client")
	call := ports.ToolCall{ID: "test-2", Name: "lark_calendar_update"}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid client type")
	}
}

func TestCalendarUpdate_MissingEventID(t *testing.T) {
	tool := NewLarkCalendarUpdate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-3", Name: "lark_calendar_update", Arguments: map[string]any{
		"summary": "Updated Title",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing event_id")
	}
}

func TestCalendarUpdate_NoFieldsToUpdate(t *testing.T) {
	tool := NewLarkCalendarUpdate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-4", Name: "lark_calendar_update", Arguments: map[string]any{
		"event_id": "evt_123",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when no fields to update are provided")
	}
}

func TestCalendarUpdate_MissingOAuthToken_ShowsAuthURL(t *testing.T) {
	tool := NewLarkCalendarUpdate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	oauthSvc := &fakeLarkOAuth{
		err:      &larkoauth.NeedUserAuthError{AuthURL: "http://localhost:8080/api/lark/oauth/start"},
		startURL: "http://localhost:8080/api/lark/oauth/start",
	}

	ctx := id.WithUserID(context.Background(), "ou_123")
	ctx = shared.WithLarkClient(ctx, larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-5", Name: "lark_calendar_update", Arguments: map[string]any{
		"event_id": "evt_123",
		"summary":  "Updated",
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

func TestCalendarUpdate_PrimaryCalendarIDResolves(t *testing.T) {
	var gotAuth string
	var gotPath string

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
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/calendar/v4/calendars/") && strings.Contains(r.URL.Path, "/events/"):
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{
				"event": map[string]interface{}{
					"event_id": "evt_123",
					"summary":  "Updated",
				},
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkCalendarUpdate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	oauthSvc := &fakeLarkOAuth{token: "user-token", startURL: "http://localhost:8080/api/lark/oauth/start"}
	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-6", Name: "lark_calendar_update", Arguments: map[string]any{
		"event_id":   "evt_123",
		"summary":    "Updated",
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
	if gotAuth != "Bearer user-token" {
		t.Fatalf("expected user token auth, got %q", gotAuth)
	}
	if !strings.Contains(gotPath, "/calendar/v4/calendars/cal-primary/events/evt_123") {
		t.Fatalf("unexpected path: %s", gotPath)
	}
}
