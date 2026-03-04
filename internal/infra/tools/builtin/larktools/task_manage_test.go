package larktools

import (
	"context"
	"io"
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

func TestTaskManage_NoLarkClient(t *testing.T) {
	tool := NewLarkTaskManage()
	call := ports.ToolCall{ID: "test-1", Name: "lark_task_manage"}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when lark client is missing")
	}
}

func TestTaskManage_InvalidAction(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-2", Name: "lark_task_manage", Arguments: map[string]any{
		"action": "unknown",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for unsupported action")
	}
}

func TestTaskManage_CreateMissingSummary(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-3", Name: "lark_task_manage", Arguments: map[string]any{
		"action": "create",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing summary")
	}
}

func TestTaskManage_CreateInvalidDueDate(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-4", Name: "lark_task_manage", Arguments: map[string]any{
		"action":   "create",
		"summary":  "Test",
		"due_date": "2024-13-99",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid due_date")
	}
}

func TestTaskManage_UpdateMissingTaskID(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-5", Name: "lark_task_manage", Arguments: map[string]any{
		"action":  "update",
		"summary": "Updated title",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing task_id on update")
	}
}

func TestTaskManage_UpdateNoFields(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-6", Name: "lark_task_manage", Arguments: map[string]any{
		"action":  "update",
		"task_id": "some-task-guid",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when no update fields are provided")
	}
}

func TestTaskManage_UpdateInvalidDueTime(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-7", Name: "lark_task_manage", Arguments: map[string]any{
		"action":   "update",
		"task_id":  "some-task-guid",
		"due_time": "not-a-number",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid due_time")
	}
}

func TestTaskManage_DeleteMissingTaskID(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	call := ports.ToolCall{ID: "test-8", Name: "lark_task_manage", Arguments: map[string]any{
		"action": "delete",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing task_id on delete")
	}
}

func TestTaskManage_InvalidActionStillWorks(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	for _, action := range []string{"remove", "patch", "archive", ""} {
		call := ports.ToolCall{ID: "test-9", Name: "lark_task_manage", Arguments: map[string]any{
			"action": action,
		}}

		result, err := tool.Execute(ctx, call)
		if err != nil {
			t.Fatalf("unexpected error for action=%q: %v", action, err)
		}
		if result.Error == nil {
			t.Fatalf("expected error for unsupported action=%q", action)
		}
	}
}

func TestTaskManage_ListAutoUsesOAuthToken(t *testing.T) {
	var mu sync.Mutex
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/task/v2/tasks"):
			mu.Lock()
			gotAuth = r.Header.Get("Authorization")
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"items":    []map[string]any{},
				"has_more": false,
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	oauthSvc := &fakeLarkOAuth{token: "user-token", startURL: "http://localhost:8080/api/lark/oauth/start"}
	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-list-oauth", Name: "lark_task_manage", Arguments: map[string]any{
		"action": "list",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if oauthSvc.gotOpenID != "ou_123" {
		t.Fatalf("oauth service received open_id=%q, want %q", oauthSvc.gotOpenID, "ou_123")
	}
	mu.Lock()
	defer mu.Unlock()
	if gotAuth != "Bearer user-token" {
		t.Fatalf("expected user token auth, got %q", gotAuth)
	}
}

func TestTaskManage_ListMissingOAuthTokenRequestsAuthorization(t *testing.T) {
	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	oauthSvc := &fakeLarkOAuth{
		err:      &larkoauth.NeedUserAuthError{AuthURL: "http://localhost:8080/api/lark/oauth/start"},
		startURL: "http://localhost:8080/api/lark/oauth/start",
	}

	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-list-oauth-missing", Name: "lark_task_manage", Arguments: map[string]any{
		"action": "list",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when OAuth token is missing")
	}
	if !strings.Contains(result.Content, "Please authorize Lark task access first:") {
		t.Fatalf("expected authorization guidance, got %q", result.Content)
	}
}

func TestTaskManage_CreateAutoUsesOAuthToken(t *testing.T) {
	var mu sync.Mutex
	var gotAuth string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/task/v2/tasks"):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			mu.Lock()
			gotAuth = r.Header.Get("Authorization")
			gotBody = string(body)
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"task": map[string]any{
					"guid": "task-guid-1",
				},
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	oauthSvc := &fakeLarkOAuth{token: "user-token", startURL: "http://localhost:8080/api/lark/oauth/start"}
	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-create-oauth", Name: "lark_task_manage", Arguments: map[string]any{
		"action":      "create",
		"summary":     "Task title",
		"description": "Task body",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if oauthSvc.gotOpenID != "ou_123" {
		t.Fatalf("oauth service received open_id=%q, want %q", oauthSvc.gotOpenID, "ou_123")
	}

	mu.Lock()
	defer mu.Unlock()
	if gotAuth != "Bearer user-token" {
		t.Fatalf("expected user token auth, got %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"summary":"Task title"`) {
		t.Fatalf("expected request body to include summary, got %q", gotBody)
	}
	if !strings.Contains(gotBody, `"description":"Task body"`) {
		t.Fatalf("expected request body to include description, got %q", gotBody)
	}
}

func TestTaskManage_CreateMissingOAuthTokenFallsBackToTenantAndAddsSender(t *testing.T) {
	var mu sync.Mutex
	var gotAuth string
	var gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/auth/v3/tenant_access_token/internal"):
			_, _ = w.Write(tokenResponse("tenant-token", 7200))
			return
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/task/v2/tasks"):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			mu.Lock()
			gotAuth = r.Header.Get("Authorization")
			gotBody = string(body)
			mu.Unlock()
			_, _ = w.Write(jsonResponse(0, "ok", map[string]any{
				"task": map[string]any{
					"guid": "task-guid-tenant",
				},
			}))
			return
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	tool := NewLarkTaskManage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret", lark.WithOpenBaseUrl(srv.URL))
	oauthSvc := &fakeLarkOAuth{
		err:      &larkoauth.NeedUserAuthError{AuthURL: "http://localhost:8080/api/lark/oauth/start"},
		startURL: "http://localhost:8080/api/lark/oauth/start",
	}

	ctx := shared.WithLarkClient(id.WithUserID(context.Background(), "ou_123"), larkClient)
	ctx = shared.WithLarkOAuth(ctx, oauthSvc)

	call := ports.ToolCall{ID: "test-create-oauth-missing", Name: "lark_task_manage", Arguments: map[string]any{
		"action":  "create",
		"summary": "Task title",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected tenant fallback success, got error: %v", result.Error)
	}

	mu.Lock()
	defer mu.Unlock()
	if !strings.HasPrefix(gotAuth, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(gotAuth, "Bearer ")) == "" {
		t.Fatalf("expected non-empty bearer auth when using tenant fallback, got %q", gotAuth)
	}
	if !strings.Contains(gotBody, `"id":"ou_123"`) {
		t.Fatalf("expected sender to be auto-added as member, got body %q", gotBody)
	}
	if !strings.Contains(gotBody, `"role":"follower"`) {
		t.Fatalf("expected sender role=follower, got body %q", gotBody)
	}
	if got := result.Metadata["auth_mode"]; got != "tenant" {
		t.Fatalf("expected auth_mode=tenant, got %v", got)
	}
	if got := result.Metadata["sender_added_as_member"]; got != true {
		t.Fatalf("expected sender_added_as_member=true, got %v", got)
	}
}

func TestNormalizeCreateTaskTextFields_SplitSummaryAndBody(t *testing.T) {
	summary, description := normalizeCreateTaskTextFields("标题\n第一行内容\n第二行内容", "", "")
	if summary != "标题" {
		t.Fatalf("expected summary to be first line, got %q", summary)
	}
	if description != "第一行内容\n第二行内容" {
		t.Fatalf("expected description to contain remaining lines, got %q", description)
	}
}

func TestNormalizeCreateTaskTextFields_ContentAlias(t *testing.T) {
	summary, description := normalizeCreateTaskTextFields("短标题", "", "这是正文")
	if summary != "短标题" {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if description != "这是正文" {
		t.Fatalf("expected description from content alias, got %q", description)
	}
}

func TestNormalizeUpdateTaskTextFields_ContentAlias(t *testing.T) {
	summary, description := normalizeUpdateTaskTextFields("", "", "仅更新正文")
	if summary != "" {
		t.Fatalf("expected empty summary, got %q", summary)
	}
	if description != "仅更新正文" {
		t.Fatalf("expected description from content alias, got %q", description)
	}
}

func TestNormalizeUpdateTaskTextFields_SplitSummaryAndBody(t *testing.T) {
	summary, description := normalizeUpdateTaskTextFields("标题\n正文", "", "")
	if summary != "标题" {
		t.Fatalf("expected summary to be first line, got %q", summary)
	}
	if description != "正文" {
		t.Fatalf("expected description from remaining lines, got %q", description)
	}
}

func TestNormalizeCreateTaskTextFields_ContentOnlyLongSingleLine(t *testing.T) {
	content := "This is a long single-line task detail that should remain in description instead of being title only."
	summary, description := normalizeCreateTaskTextFields("", "", content)
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if summary == content {
		t.Fatalf("expected compact summary, got full content: %q", summary)
	}
	if !strings.HasSuffix(summary, "...") {
		t.Fatalf("expected compact summary to end with ellipsis, got %q", summary)
	}
	if description != content {
		t.Fatalf("expected description to keep full content, got %q", description)
	}
}

func TestNormalizeCreateTaskTextFields_LongSummaryFallsBackToDescription(t *testing.T) {
	rawSummary := "This summary is too long to be a title by itself and should be preserved as description when auto-normalized."
	summary, description := normalizeCreateTaskTextFields(rawSummary, "", "")
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if summary == rawSummary {
		t.Fatalf("expected compact summary, got full summary: %q", summary)
	}
	if !strings.HasSuffix(summary, "...") {
		t.Fatalf("expected compact summary to end with ellipsis, got %q", summary)
	}
	if description != rawSummary {
		t.Fatalf("expected full summary to be preserved in description, got %q", description)
	}
}
