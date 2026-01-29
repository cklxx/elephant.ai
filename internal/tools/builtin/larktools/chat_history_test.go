package larktools

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestExecute_NoLarkClient(t *testing.T) {
	tool := NewLarkChatHistory()
	ctx := context.Background()
	call := ports.ToolCall{ID: "test-1", Name: "lark_chat_history"}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when lark client is missing")
	}
	if result.Content != "lark_chat_history is only available inside a Lark chat context." {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestExecute_InvalidClientType(t *testing.T) {
	tool := NewLarkChatHistory()
	ctx := shared.WithLarkClient(context.Background(), "not-a-lark-client")
	call := ports.ToolCall{ID: "test-2", Name: "lark_chat_history"}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result for invalid client type")
	}
}

func TestExecute_NoChatID(t *testing.T) {
	tool := NewLarkChatHistory()

	larkClient := lark.NewClient("test_app_id", "test_app_secret")
	ctx := shared.WithLarkClient(context.Background(), larkClient)
	// No chat ID set in context.
	call := ports.ToolCall{ID: "test-3", Name: "lark_chat_history"}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error in result when chat_id is missing")
	}
	if result.Content != "lark_chat_history: no chat_id available in context." {
		t.Errorf("unexpected content: %s", result.Content)
	}
}

func TestClampPageSize(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		want int
	}{
		{"nil args", nil, defaultPageSize},
		{"missing key", map[string]any{}, defaultPageSize},
		{"zero", map[string]any{"page_size": 0}, defaultPageSize},
		{"negative", map[string]any{"page_size": -5}, defaultPageSize},
		{"normal", map[string]any{"page_size": 10}, 10},
		{"at max", map[string]any{"page_size": 50}, 50},
		{"over max", map[string]any{"page_size": 100}, maxPageSize},
		{"float64", map[string]any{"page_size": float64(30)}, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := clampPageSize(tt.args)
			if got != tt.want {
				t.Errorf("clampPageSize() = %d, want %d", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

func TestExtractMessageContent(t *testing.T) {
	tests := []struct {
		name    string
		msgType string
		body    *larkim.MessageBody
		want    string
	}{
		{"nil body", "text", nil, "[empty]"},
		{"nil content", "text", &larkim.MessageBody{}, "[empty]"},
		{"text message", "text", &larkim.MessageBody{Content: strPtr(`{"text":"hello world"}`)}, "hello world"},
		{"text with at-mention", "text", &larkim.MessageBody{Content: strPtr(`{"text":"@_user_1 hi there"}`)}, "@_user_1 hi there"},
		{"empty text", "text", &larkim.MessageBody{Content: strPtr(`{"text":""}`)}, "[empty]"},
		{"invalid json text", "text", &larkim.MessageBody{Content: strPtr("plain text")}, "plain text"},
		{"image", "image", &larkim.MessageBody{Content: strPtr(`{"image_key":"img_xxx"}`)}, "[image]"},
		{"file", "file", &larkim.MessageBody{Content: strPtr(`{"file_key":"file_xxx"}`)}, "[file]"},
		{"audio", "audio", &larkim.MessageBody{Content: strPtr(`{}`)}, "[audio]"},
		{"media", "media", &larkim.MessageBody{Content: strPtr(`{}`)}, "[media]"},
		{"sticker", "sticker", &larkim.MessageBody{Content: strPtr(`{}`)}, "[sticker]"},
		{"interactive", "interactive", &larkim.MessageBody{Content: strPtr(`{}`)}, "[interactive card]"},
		{"share_chat", "share_chat", &larkim.MessageBody{Content: strPtr(`{}`)}, "[shared chat]"},
		{"share_user", "share_user", &larkim.MessageBody{Content: strPtr(`{}`)}, "[shared user]"},
		{"system", "system", &larkim.MessageBody{Content: strPtr(`{}`)}, "[system message]"},
		{"post", "post", &larkim.MessageBody{Content: strPtr(`{}`)}, "[rich text message]"},
		{"unknown type", "custom_type", &larkim.MessageBody{Content: strPtr(`{}`)}, "[custom_type]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMessageContent(tt.msgType, tt.body)
			if got != tt.want {
				t.Errorf("extractMessageContent(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestFormatMessages(t *testing.T) {
	// Items arrive in descending order (newest first); formatMessages should reverse them.
	items := []*larkim.Message{
		{
			CreateTime: strPtr("1704067200000"), // 2024-01-01 00:00:00 UTC
			MsgType:    strPtr("text"),
			Sender: &larkim.Sender{
				Id:         strPtr("ou_second"),
				SenderType: strPtr("user"),
			},
			Body: &larkim.MessageBody{Content: strPtr(`{"text":"second message"}`)},
		},
		{
			CreateTime: strPtr("1704063600000"), // 2023-12-31 23:00:00 UTC (earlier)
			MsgType:    strPtr("text"),
			Sender: &larkim.Sender{
				Id:         strPtr("ou_first"),
				SenderType: strPtr("user"),
			},
			Body: &larkim.MessageBody{Content: strPtr(`{"text":"first message"}`)},
		},
	}

	result := formatMessages(items)
	lines := splitLines(result)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), result)
	}

	// First line should be the earlier message (reversed from desc order).
	if !contains(lines[0], "first message") {
		t.Errorf("first line should contain 'first message': %s", lines[0])
	}
	if !contains(lines[0], "user(ou_first)") {
		t.Errorf("first line should contain sender: %s", lines[0])
	}
	if !contains(lines[1], "second message") {
		t.Errorf("second line should contain 'second message': %s", lines[1])
	}
}

func TestFormatMessages_WithBotSender(t *testing.T) {
	items := []*larkim.Message{
		{
			CreateTime: strPtr("1704067200000"),
			MsgType:    strPtr("text"),
			Sender: &larkim.Sender{
				Id:         strPtr("cli_bot123"),
				SenderType: strPtr("app"),
			},
			Body: &larkim.MessageBody{Content: strPtr(`{"text":"bot reply"}`)},
		},
	}

	result := formatMessages(items)
	if !contains(result, "bot(cli_bot123)") {
		t.Errorf("expected bot sender format: %s", result)
	}
}

func TestFormatMessages_NilItems(t *testing.T) {
	items := []*larkim.Message{nil, nil}
	result := formatMessages(items)
	if result != "" {
		t.Errorf("expected empty string for nil items, got: %q", result)
	}
}

func TestFormatMessages_Empty(t *testing.T) {
	result := formatMessages(nil)
	if result != "" {
		t.Errorf("expected empty string, got: %q", result)
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		name string
		ms   string
		want string
	}{
		{"empty", "", "unknown"},
		{"non-numeric", "abc", "abc"},
		{"valid ms", "1704067200000", "2024-01-01"}, // Just check date prefix
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatTimestamp(tt.ms)
			if tt.name == "valid ms" {
				if !contains(got, tt.want) {
					t.Errorf("formatTimestamp(%q) = %q, want prefix %q", tt.ms, got, tt.want)
				}
			} else {
				if got != tt.want {
					t.Errorf("formatTimestamp(%q) = %q, want %q", tt.ms, got, tt.want)
				}
			}
		})
	}
}

func TestFormatSender(t *testing.T) {
	tests := []struct {
		name   string
		sender *larkim.Sender
		want   string
	}{
		{"nil", nil, "unknown"},
		{"user", &larkim.Sender{Id: strPtr("ou_123"), SenderType: strPtr("user")}, "user(ou_123)"},
		{"app", &larkim.Sender{Id: strPtr("cli_456"), SenderType: strPtr("app")}, "bot(cli_456)"},
		{"anonymous", &larkim.Sender{Id: strPtr(""), SenderType: strPtr("anonymous")}, "anonymous"},
		{"other with id", &larkim.Sender{Id: strPtr("id_789"), SenderType: strPtr("unknown")}, "unknown(id_789)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSender(tt.sender)
			if got != tt.want {
				t.Errorf("formatSender() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetadata(t *testing.T) {
	tool := NewLarkChatHistory()
	meta := tool.Metadata()
	if meta.Name != "lark_chat_history" {
		t.Errorf("unexpected name: %s", meta.Name)
	}
	if meta.Category != "lark" {
		t.Errorf("unexpected category: %s", meta.Category)
	}
}

func TestDefinition(t *testing.T) {
	tool := NewLarkChatHistory()
	def := tool.Definition()
	if def.Name != "lark_chat_history" {
		t.Errorf("unexpected name: %s", def.Name)
	}
	if _, ok := def.Parameters.Properties["page_size"]; !ok {
		t.Error("missing page_size parameter")
	}
	if _, ok := def.Parameters.Properties["start_time"]; !ok {
		t.Error("missing start_time parameter")
	}
	if _, ok := def.Parameters.Properties["end_time"]; !ok {
		t.Error("missing end_time parameter")
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	for _, line := range splitByNewline(s) {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func splitByNewline(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
