package larktools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func TestSendMessage_NoLarkClient(t *testing.T) {
	tool := NewLarkSendMessage()
	ctx := context.Background()
	call := ports.ToolCall{
		ID:        "call-1",
		Name:      "lark_send_message",
		Arguments: map[string]any{"content": "hello"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when lark client is missing")
	}
	if result.Content != "lark_send_message is only available inside a Lark chat context." {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestSendMessage_UnsupportedParameter(t *testing.T) {
	tool := NewLarkSendMessage()
	ctx := context.Background()
	call := ports.ToolCall{
		ID:   "call-unsupported",
		Name: "lark_send_message",
		Arguments: map[string]any{
			"content":             "hello",
			"reply_to_message_id": "om_mock",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result for unsupported parameter")
	}
	if result.Content != "unsupported parameter: reply_to_message_id" {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestSendMessage_InvalidClientType(t *testing.T) {
	tool := NewLarkSendMessage()
	ctx := shared.WithLarkClient(context.Background(), "not-a-lark-client")
	call := ports.ToolCall{
		ID:        "call-2",
		Name:      "lark_send_message",
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
	tool := NewLarkSendMessage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	call := ports.ToolCall{
		ID:        "call-3",
		Name:      "lark_send_message",
		Arguments: map[string]any{"content": "hello"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when chat_id is missing")
	}
	if result.Content != "lark_send_message: no chat_id available in context." {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestSendMessage_MissingContent(t *testing.T) {
	tool := NewLarkSendMessage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")
	call := ports.ToolCall{
		ID:        "call-4",
		Name:      "lark_send_message",
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
	tool := NewLarkSendMessage()
	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")
	call := ports.ToolCall{
		ID:        "call-5",
		Name:      "lark_send_message",
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
	tool := NewLarkSendMessage()
	meta := tool.Metadata()
	if meta.Name != "lark_send_message" {
		t.Errorf("unexpected name: %s", meta.Name)
	}
	if meta.Category != "lark" {
		t.Errorf("unexpected category: %s", meta.Category)
	}
}

func TestSendMessage_Definition(t *testing.T) {
	tool := NewLarkSendMessage()
	def := tool.Definition()
	if def.Name != "lark_send_message" {
		t.Errorf("unexpected name: %s", def.Name)
	}
	if _, ok := def.Parameters.Properties["content"]; !ok {
		t.Error("missing content parameter")
	}
	if _, ok := def.Parameters.Properties["reply_to_message_id"]; ok {
		t.Error("unexpected reply_to_message_id parameter")
	}
	if len(def.Parameters.Required) != 1 || def.Parameters.Required[0] != "content" {
		t.Errorf("unexpected required params: %v", def.Parameters.Required)
	}
}

func TestTextPayload(t *testing.T) {
	got := textPayload("hello world")
	if got != `{"text":"hello world"}` {
		t.Errorf("unexpected payload: %s", got)
	}
}

func TestTextPayload_RendersOutgoingMention(t *testing.T) {
	got := textPayload("hi @Bob(ou_123)")
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("failed to parse payload json: %v", err)
	}
	if !strings.Contains(parsed.Text, `<at user_id="ou_123">Bob</at>`) {
		t.Errorf("expected outgoing mention to render, got %s", parsed.Text)
	}
}

func TestTextPayload_SpecialChars(t *testing.T) {
	got := textPayload(`say "hello" & <bye>`)
	// JSON encodes angle brackets and ampersands within strings, but quotes are escaped.
	if got == "" {
		t.Error("expected non-empty payload")
	}
}
