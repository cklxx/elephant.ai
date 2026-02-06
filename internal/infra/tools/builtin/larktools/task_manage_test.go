package larktools

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/shared"

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
