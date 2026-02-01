package lark

import (
	"context"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestFormatChatMessages_Chronological(t *testing.T) {
	// Simulate API response: descending order (newest first).
	ts1 := "1706500000000" // older
	ts2 := "1706500060000" // newer
	senderType1 := "user"
	senderID1 := "ou_user1"
	senderType2 := "app"
	senderID2 := "cli_bot"
	msgType := "text"
	body1Content := `{"text":"hello"}`
	body2Content := `{"text":"world"}`

	items := []*larkim.Message{
		{
			CreateTime: &ts2,
			Sender:     &larkim.Sender{SenderType: &senderType2, Id: &senderID2},
			MsgType:    &msgType,
			Body:       &larkim.MessageBody{Content: &body2Content},
		},
		{
			CreateTime: &ts1,
			Sender:     &larkim.Sender{SenderType: &senderType1, Id: &senderID1},
			MsgType:    &msgType,
			Body:       &larkim.MessageBody{Content: &body1Content},
		},
	}

	result := formatChatMessages(items)

	// After reversal, older message should come first.
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	// Check order: first line should contain "hello", second "world".
	lines := splitLines(result)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), result)
	}
	if !containsSubstring(lines[0], "hello") {
		t.Fatalf("expected first line to contain 'hello', got %q", lines[0])
	}
	if !containsSubstring(lines[1], "world") {
		t.Fatalf("expected second line to contain 'world', got %q", lines[1])
	}
	if !containsSubstring(lines[0], "user(ou_user1)") {
		t.Fatalf("expected first line to contain 'user(ou_user1)', got %q", lines[0])
	}
	if !containsSubstring(lines[1], "bot(cli_bot)") {
		t.Fatalf("expected second line to contain 'bot(cli_bot)', got %q", lines[1])
	}
}

func TestFormatChatMessages_Empty(t *testing.T) {
	result := formatChatMessages(nil)
	if result != "" {
		t.Fatalf("expected empty result for nil items, got %q", result)
	}

	result = formatChatMessages([]*larkim.Message{})
	if result != "" {
		t.Fatalf("expected empty result for empty items, got %q", result)
	}
}

func TestFormatChatMessages_NilItem(t *testing.T) {
	items := []*larkim.Message{nil}
	result := formatChatMessages(items)
	if result != "" {
		t.Fatalf("expected empty result for nil item, got %q", result)
	}
}

func TestFormatChatTimestamp(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantSub  string
	}{
		{"valid timestamp", "1706500000000", "2024-01-29"},
		{"empty", "", "unknown"},
		{"non-numeric", "abc", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatChatTimestamp(tt.input)
			if !containsSubstring(got, tt.wantSub) {
				t.Fatalf("formatChatTimestamp(%q) = %q, want substring %q", tt.input, got, tt.wantSub)
			}
		})
	}
}

func TestFormatChatSender(t *testing.T) {
	tests := []struct {
		name   string
		sender *larkim.Sender
		want   string
	}{
		{"nil sender", nil, "unknown"},
		{"user sender", makeSender("user", "ou_123"), "user(ou_123)"},
		{"app sender", makeSender("app", "cli_bot"), "bot(cli_bot)"},
		{"other sender", makeSender("webhook", "wh_1"), "webhook(wh_1)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatChatSender(tt.sender)
			if got != tt.want {
				t.Fatalf("formatChatSender() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractChatMessageContent(t *testing.T) {
	tests := []struct {
		name    string
		msgType string
		body    *larkim.MessageBody
		want    string
	}{
		{"nil body", "text", nil, "[empty]"},
		{"text message", "text", makeBody(`{"text":"hello"}`), "hello"},
		{"image message", "image", makeBody(`{"image_key":"key"}`), "[image]"},
		{"file message", "file", makeBody(`{}`), "[file]"},
		{"post message", "post", makeBody(`{}`), "[rich text message]"},
		{"unknown type", "custom", makeBody(`{}`), "[custom]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractChatMessageContent(tt.msgType, tt.body)
			if got != tt.want {
				t.Fatalf("extractChatMessageContent(%q) = %q, want %q", tt.msgType, got, tt.want)
			}
		})
	}
}

func TestExtractChatTextContent(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"valid", `{"text":"hello"}`, "hello"},
		{"empty text", `{"text":""}`, "[empty]"},
		{"invalid json", "not json", "not json"},
		{"whitespace only", `{"text":"  "}`, "[empty]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractChatTextContent(tt.raw)
			if got != tt.want {
				t.Fatalf("extractChatTextContent(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestFetchRecentChatMessages_NilMessenger(t *testing.T) {
	gw := &Gateway{}
	_, err := gw.fetchRecentChatMessages(context.Background(), "oc_123", 20)
	if err == nil {
		t.Fatal("expected error for nil messenger")
	}
}

func TestFetchRecentChatMessages_EmptyChatID(t *testing.T) {
	gw := &Gateway{messenger: NewRecordingMessenger()}
	_, err := gw.fetchRecentChatMessages(context.Background(), "", 20)
	if err == nil {
		t.Fatal("expected error for empty chat_id")
	}
}

// --- test helpers ---

func makeSender(senderType, id string) *larkim.Sender {
	return &larkim.Sender{SenderType: &senderType, Id: &id}
}

func makeBody(content string) *larkim.MessageBody {
	return &larkim.MessageBody{Content: &content}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && (sub == "" || findSubstring(s, sub))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
