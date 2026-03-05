package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type stubLarkMessenger struct {
	sendMessageID string
	sendCalls     int
	lastChatID    string
	lastMsgType   string
	lastContent   string
	sendErr       error

	replyMessageID string
	replyCalls     int
	lastReplyTo    string
	replyErr       error
}

func (s *stubLarkMessenger) SendMessage(_ context.Context, chatID, msgType, content string) (string, error) {
	s.sendCalls++
	s.lastChatID = chatID
	s.lastMsgType = msgType
	s.lastContent = content
	if s.sendErr != nil {
		return "", s.sendErr
	}
	if s.sendMessageID == "" {
		return "om_synthetic_stub", nil
	}
	return s.sendMessageID, nil
}

func (s *stubLarkMessenger) ReplyMessage(_ context.Context, replyToID, msgType, content string) (string, error) {
	s.replyCalls++
	s.lastReplyTo = replyToID
	s.lastMsgType = msgType
	s.lastContent = content
	if s.replyErr != nil {
		return "", s.replyErr
	}
	if s.replyMessageID == "" {
		return "om_reply_stub", nil
	}
	return s.replyMessageID, nil
}

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
	if result.Content != "lark_send_message: no chat_id available in context." {
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

func TestSendMessage_SyntheticInjectUsesContextMessenger(t *testing.T) {
	tool := NewLarkSendMessage()
	messenger := &stubLarkMessenger{sendMessageID: "om_synth_1"}
	ctx := context.Background()
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")
	ctx = shared.WithLarkMessageID(ctx, "inject_oc_chat123_1")
	ctx = shared.WithLarkMessenger(ctx, messenger)

	call := ports.ToolCall{
		ID:        "call-synth",
		Name:      "lark_send_message",
		Arguments: map[string]any{"content": "hello synthetic"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected nil result error, got %v", result.Error)
	}
	if messenger.sendCalls != 1 {
		t.Fatalf("expected 1 synthetic messenger call, got %d", messenger.sendCalls)
	}
	if messenger.lastChatID != "oc_chat123" {
		t.Fatalf("unexpected chat id: %s", messenger.lastChatID)
	}
	if messenger.lastMsgType != "text" {
		t.Fatalf("unexpected msg type: %s", messenger.lastMsgType)
	}
	if messenger.lastContent != `{"text":"hello synthetic"}` {
		t.Fatalf("unexpected payload: %s", messenger.lastContent)
	}
	if result.Metadata["message_id"] != "om_synth_1" {
		t.Fatalf("unexpected metadata message_id: %#v", result.Metadata["message_id"])
	}
	if result.Metadata["synthetic_inject"] != true {
		t.Fatalf("expected synthetic_inject metadata true, got %#v", result.Metadata["synthetic_inject"])
	}
}

func TestSendMessage_SyntheticInjectMessengerError(t *testing.T) {
	tool := NewLarkSendMessage()
	messenger := &stubLarkMessenger{sendErr: fmt.Errorf("send failed")}
	ctx := context.Background()
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")
	ctx = shared.WithLarkMessageID(ctx, "inject_oc_chat123_1")
	ctx = shared.WithLarkMessenger(ctx, messenger)

	call := ports.ToolCall{
		ID:        "call-synth-err",
		Name:      "lark_send_message",
		Arguments: map[string]any{"content": "hello synthetic"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected result error for synthetic messenger failure")
	}
	if !strings.Contains(result.Content, "messenger send failed") {
		t.Fatalf("unexpected content: %s", result.Content)
	}
}

func TestSendMessage_NonSyntheticPrefersContextMessenger(t *testing.T) {
	tool := NewLarkSendMessage()
	messenger := &stubLarkMessenger{replyMessageID: "om_ctx_reply_1"}
	ctx := context.Background()
	ctx = shared.WithLarkChatID(ctx, "oc_chat123")
	ctx = shared.WithLarkMessageID(ctx, "om_real_123")
	ctx = shared.WithLarkMessenger(ctx, messenger)
	ctx = shared.WithLarkClient(ctx, "invalid-client-should-not-be-used")

	call := ports.ToolCall{
		ID:        "call-nonsynth",
		Name:      "lark_send_message",
		Arguments: map[string]any{"content": "hello messenger"},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected nil result error, got %v", result.Error)
	}
	if messenger.replyCalls != 1 {
		t.Fatalf("expected 1 messenger reply call, got %d", messenger.replyCalls)
	}
	if messenger.sendCalls != 0 {
		t.Fatalf("expected 0 messenger send call for reply mode, got %d", messenger.sendCalls)
	}
	if messenger.lastReplyTo != "om_real_123" {
		t.Fatalf("unexpected reply target: %s", messenger.lastReplyTo)
	}
	if result.Metadata["message_id"] != "om_ctx_reply_1" {
		t.Fatalf("unexpected metadata message_id: %#v", result.Metadata["message_id"])
	}
	if _, ok := result.Metadata["synthetic_inject"]; ok {
		t.Fatalf("did not expect synthetic_inject metadata for non-synthetic send")
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

func TestIsSyntheticInjectMessageID(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{name: "synthetic inject id", id: "inject_oc_chat_1", want: true},
		{name: "trimmed synthetic id", id: "  inject_auto_oc_chat_1  ", want: true},
		{name: "real lark id", id: "om_real_123", want: false},
		{name: "blank", id: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSyntheticInjectMessageID(tt.id); got != tt.want {
				t.Fatalf("isSyntheticInjectMessageID(%q)=%v want=%v", tt.id, got, tt.want)
			}
		})
	}
}
