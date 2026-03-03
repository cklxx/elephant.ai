package larktools

import (
	"context"
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
