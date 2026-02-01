package larktools

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/shared"

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
