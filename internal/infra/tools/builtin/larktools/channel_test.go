package larktools

import (
	"context"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestChannel_Metadata(t *testing.T) {
	tool := NewLarkChannel()
	meta := tool.Metadata()
	if meta.Name != "channel" {
		t.Fatalf("expected name 'channel', got %q", meta.Name)
	}
	if meta.Category != "lark" {
		t.Fatalf("expected category 'lark', got %q", meta.Category)
	}
}

func TestChannel_Definition(t *testing.T) {
	tool := NewLarkChannel()
	def := tool.Definition()
	if def.Name != "channel" {
		t.Fatalf("expected name 'channel', got %q", def.Name)
	}
	if _, ok := def.Parameters.Properties["action"]; !ok {
		t.Fatal("missing 'action' parameter")
	}
	if len(def.Parameters.Required) != 1 || def.Parameters.Required[0] != "action" {
		t.Fatalf("expected required=[action], got %v", def.Parameters.Required)
	}
}

func TestChannel_NoLarkClient(t *testing.T) {
	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "t1", Name: "channel", Arguments: map[string]any{
		"action": "send_message",
	}}
	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when lark client is missing")
	}
}

func TestChannel_InvalidClientType(t *testing.T) {
	tool := NewLarkChannel()
	ctx := shared.WithLarkClient(context.Background(), "invalid")
	call := ports.ToolCall{ID: "t2", Name: "channel", Arguments: map[string]any{
		"action": "send_message",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for invalid client type")
	}
}

func TestChannel_MissingAction(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t3", Name: "channel", Arguments: map[string]any{}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing action")
	}
}

func TestChannel_UnsupportedAction(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	for _, action := range []string{"unknown", "remove", ""} {
		call := ports.ToolCall{ID: "t4", Name: "channel", Arguments: map[string]any{
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

func TestChannel_SendMessage_NoChatID(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t5", Name: "channel", Arguments: map[string]any{
		"action":  "send_message",
		"content": "hello",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing chat_id")
	}
}

func TestChannel_History_NoChatID(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t6", Name: "channel", Arguments: map[string]any{
		"action": "history",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing chat_id")
	}
}

func TestChannel_UploadFile_NoChatID(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t7", Name: "channel", Arguments: map[string]any{
		"action": "upload_file",
		"path":   "/tmp/test.txt",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing chat_id")
	}
}

func TestChannel_CreateEvent_MissingRequired(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t8", Name: "channel", Arguments: map[string]any{
		"action": "create_event",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing calendar create params")
	}
}

func TestChannel_QueryEvents_MissingRequired(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t9", Name: "channel", Arguments: map[string]any{
		"action": "query_events",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing time range")
	}
}

func TestChannel_UpdateEvent_MissingEventID(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t10", Name: "channel", Arguments: map[string]any{
		"action": "update_event",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing event_id")
	}
}

func TestChannel_DeleteEvent_MissingEventID(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t11", Name: "channel", Arguments: map[string]any{
		"action": "delete_event",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing event_id")
	}
}

func TestChannel_ListTasks_NoLarkClient(t *testing.T) {
	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "t12", Name: "channel", Arguments: map[string]any{
		"action": "list_tasks",
	}}
	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when lark client is missing")
	}
}

func TestChannel_CreateTask_MissingSummary(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t13", Name: "channel", Arguments: map[string]any{
		"action": "create_task",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing summary on create_task")
	}
}

func TestChannel_UpdateTask_MissingTaskID(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t14", Name: "channel", Arguments: map[string]any{
		"action":  "update_task",
		"summary": "Updated",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing task_id on update_task")
	}
}

func TestChannel_DeleteTask_MissingTaskID(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{ID: "t15", Name: "channel", Arguments: map[string]any{
		"action": "delete_task",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing task_id on delete_task")
	}
}

func TestChannel_ActionCaseInsensitive(t *testing.T) {
	tool := NewLarkChannel()
	larkClient := lark.NewClient("id", "secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)

	// "SEND_MESSAGE" should be normalized to "send_message" and dispatch correctly.
	// It will fail on missing chat_id, which proves dispatch succeeded.
	call := ports.ToolCall{ID: "t16", Name: "channel", Arguments: map[string]any{
		"action":  "SEND_MESSAGE",
		"content": "test",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected chat_id error, not unsupported action error")
	}
	if strings.Contains(result.Content, "unsupported channel action") {
		t.Fatal("action should be case-insensitive but got unsupported action error")
	}
}

func TestChannel_DescriptionMentionsActions(t *testing.T) {
	tool := NewLarkChannel()
	desc := tool.Definition().Description
	for _, keyword := range []string{"send_message", "upload_file", "history", "create_event", "query_events", "list_tasks"} {
		if !strings.Contains(desc, keyword) {
			t.Fatalf("expected description to mention %q", keyword)
		}
	}
}
