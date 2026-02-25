package larktools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestCalendarDelete_NoLarkClient(t *testing.T) {
	tool := NewLarkCalendarDelete()
	call := ports.ToolCall{ID: "test-1", Name: "lark_calendar_delete"}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when lark client is missing")
	}
}

func TestCalendarDelete_InvalidClientType(t *testing.T) {
	tool := NewLarkCalendarDelete()
	ctx := shared.WithLarkClient(context.Background(), "not-a-lark-client")
	call := ports.ToolCall{ID: "test-2", Name: "lark_calendar_delete"}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid client type")
	}
}

func TestCalendarDelete_MissingEventID(t *testing.T) {
	tool := NewLarkCalendarDelete()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-3", Name: "lark_calendar_delete", Arguments: map[string]any{}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing event_id")
	}
}

func TestCalendarDelete_MissingOAuthToken_RequiresTenantCalendarID(t *testing.T) {
	tool := NewLarkCalendarDelete()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	oauthSvc := &fakeLarkOAuth{
		err:      &larkoauth.NeedUserAuthError{AuthURL: "http://localhost:8080/api/lark/oauth/start"},
		startURL: "http://localhost:8080/api/lark/oauth/start",
	}

	ctx := id.WithUserID(context.Background(), "ou_123")
	ctx = shared.WithLarkClient(ctx, larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-4", Name: "lark_calendar_delete", Arguments: map[string]any{
		"event_id": "evt_123",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing OAuth token")
	}
	if !strings.Contains(result.Content, "tenant_calendar_id") {
		t.Fatalf("expected tenant_calendar_id guidance, got %q", result.Content)
	}
}

func TestCalendarDelete_PrimaryCalendarIDResolves(t *testing.T) {
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
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/calendar/v4/calendars/") && strings.Contains(r.URL.Path, "/events/"):
			gotAuth = r.Header.Get("Authorization")
			gotPath = r.URL.Path
			_, _ = w.Write(jsonResponse(0, "ok", map[string]interface{}{}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkCalendarDelete()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	oauthSvc := &fakeLarkOAuth{token: "user-token", startURL: "http://localhost:8080/api/lark/oauth/start"}
	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-5", Name: "lark_calendar_delete", Arguments: map[string]any{
		"event_id": "evt_123",
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
