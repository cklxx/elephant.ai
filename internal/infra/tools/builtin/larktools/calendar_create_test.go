package larktools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"alex/internal/domain/agent/ports"
	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils/id"

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

type fakeLarkOAuth struct {
	token    string
	err      error
	startURL string

	gotOpenID string
}

func (f *fakeLarkOAuth) UserAccessToken(ctx context.Context, openID string) (string, error) {
	f.gotOpenID = openID
	if f.err != nil {
		return "", f.err
	}
	return f.token, nil
}

func (f *fakeLarkOAuth) StartURL() string {
	return f.startURL
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
		"start_time": "1700000000",
		"end_time":   "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing summary")
	}
}

func TestCalendarCreate_MissingOAuthToken_RequiresTenantCalendarID(t *testing.T) {
	tool := NewLarkCalendarCreate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	oauthSvc := &fakeLarkOAuth{
		err:      &larkoauth.NeedUserAuthError{AuthURL: "http://localhost:8080/api/lark/oauth/start"},
		startURL: "http://localhost:8080/api/lark/oauth/start",
	}

	ctx := id.WithUserID(context.Background(), "ou_123")
	ctx = shared.WithLarkClient(ctx, larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-4", Name: "lark_calendar_create", Arguments: map[string]any{
		"summary":    "Test",
		"start_time": "1700000000",
		"end_time":   "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when OAuth token is missing")
	}
	if !strings.Contains(result.Content, "tenant_calendar_id") {
		t.Fatalf("expected tenant_calendar_id guidance, got %q", result.Content)
	}
}

func TestCalendarCreate_TenantAutoSharedCalendar(t *testing.T) {
	var mu sync.Mutex
	var gotAuth string
	var tokenCalls int

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/auth/v3/tenant_access_token/internal"):
			mu.Lock()
			tokenCalls++
			mu.Unlock()
			_, _ = w.Write(tokenResponse("tenant-token", 7200))
			return
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/calendar/v4/calendars"):
			t.Fatalf("unexpected calendar list request: %s %s", r.Method, r.URL.Path)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/calendar/v4/calendars/") && strings.Contains(r.URL.Path, "/events"):
			if !strings.Contains(r.URL.Path, "/calendar/v4/calendars/cal-shared/") {
				t.Fatalf("unexpected calendar_id in path: %s", r.URL.Path)
			}
			mu.Lock()
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
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	ctx = shared.WithLarkTenantCalendarID(ctx, "cal-shared")

	call := ports.ToolCall{ID: "test-tenant-auto", Name: "lark_calendar_create", Arguments: map[string]any{
		"summary":    "Test",
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
	if tokenCalls == 0 {
		t.Fatalf("expected tenant token auto-refresh to be invoked")
	}
	if gotAuth != "Bearer tenant-token" {
		t.Fatalf("expected tenant token auth, got %q", gotAuth)
	}
}

func TestCalendarCreate_TenantModeMissingCalendarID(t *testing.T) {
	tool := NewLarkCalendarCreate()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-tenant-missing", Name: "lark_calendar_create", Arguments: map[string]any{
		"summary":    "Test",
		"start_time": "1700000000",
		"end_time":   "1700003600",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when tenant calendar_id is missing")
	}
	if !strings.Contains(result.Content, "tenant_calendar_id") {
		t.Fatalf("expected tenant_calendar_id guidance, got %q", result.Content)
	}
}

func TestCalendarCreate_PrimaryCalendarIDResolves(t *testing.T) {
	var mu sync.Mutex
	var gotCalendarID string
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
	oauthSvc := &fakeLarkOAuth{token: "user-token", startURL: "http://localhost:8080/api/lark/oauth/start"}
	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-4", Name: "lark_calendar_create", Arguments: map[string]any{
		"summary":    "Test",
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
	mu.Lock()
	defer mu.Unlock()
	if gotCalendarID != "cal-primary" {
		t.Fatalf("create used calendar_id=%q, want %q", gotCalendarID, "cal-primary")
	}
	if gotAuth != "Bearer user-token" {
		t.Fatalf("expected user token auth, got %q", gotAuth)
	}
	if oauthSvc.gotOpenID != "ou_123" {
		t.Fatalf("oauth service received open_id=%q, want %q", oauthSvc.gotOpenID, "ou_123")
	}
}
