package larktools

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/shared"

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

	call := ports.ToolCall{ID: "test-3", Name: "lark_calendar_delete", Arguments: map[string]any{
		"calendar_id": "cal_123",
	}}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing event_id")
	}
}
