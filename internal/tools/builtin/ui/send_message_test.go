package ui

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestSendMessage_NoLarkClient(t *testing.T) {
	tool := NewSendMessage()
	ctx := context.Background()
	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "send_message",
		Arguments: map[string]any{"content": "hello"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when lark client is missing")
	}
	if result.Content != "send_message is only available inside a Lark chat context." {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestSendMessage_InvalidClientType(t *testing.T) {
	tool := NewSendMessage()
	ctx := shared.WithLarkClient(context.Background(), "not-a-lark-client")
	call := ports.ToolCall{
		ID:        "call-2",
		Name:      "send_message",
		Arguments: map[string]any{"content": "hello"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result for invalid client type")
	}
}

func TestSendMessage_NoChatID(t *testing.T) {
	tool := NewSendMessage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{
		ID:        "call-3",
		Name:      "send_message",
		Arguments: map[string]any{"content": "hello"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when chat_id is missing")
	}
	if result.Content != "send_message: no chat_id available in context." {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestSendMessage_MissingContent(t *testing.T) {
	tool := NewSendMessage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")
	call := ports.ToolCall{
		ID:        "call-4",
		Name:      "send_message",
		Arguments: map[string]any{},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when content is missing")
	}
}

func TestSendMessage_EmptyContent(t *testing.T) {
	tool := NewSendMessage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")
	call := ports.ToolCall{
		ID:        "call-5",
		Name:      "send_message",
		Arguments: map[string]any{"content": "   "},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when content is blank")
	}
}

func TestSendMessage_Metadata(t *testing.T) {
	tool := NewSendMessage()
	meta := tool.Metadata()
	if meta.Name != "send_message" {
		t.Errorf("unexpected name: %s", meta.Name)
	}
	if meta.Category == "" {
		t.Errorf("expected non-empty category")
	}
}

func TestSendMessage_Definition(t *testing.T) {
	tool := NewSendMessage()
	def := tool.Definition()
	if def.Name != "send_message" {
		t.Errorf("unexpected name: %s", def.Name)
	}
	if _, ok := def.Parameters.Properties["content"]; !ok {
		t.Error("missing content parameter")
	}
	if _, ok := def.Parameters.Properties["reply_to_message_id"]; !ok {
		t.Error("missing reply_to_message_id parameter")
	}
	if len(def.Parameters.Required) != 1 || def.Parameters.Required[0] != "content" {
		t.Errorf("unexpected required params: %v", def.Parameters.Required)
	}
}

