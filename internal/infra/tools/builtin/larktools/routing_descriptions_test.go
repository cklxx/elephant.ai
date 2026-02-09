package larktools

import (
	"strings"
	"testing"
)

func TestLarkToolDescriptionsExpressRoutingBoundaries(t *testing.T) {
	t.Parallel()

	chatDesc := NewLarkChatHistory().Definition().Description
	if !strings.Contains(chatDesc, "Lark chat thread") || !strings.Contains(chatDesc, "not for repository files/topology") {
		t.Fatalf("expected lark_chat_history description to mention context retrieval, got %q", chatDesc)
	}

	sendDesc := NewLarkSendMessage().Definition().Description
	if !strings.Contains(sendDesc, "text/status") || !strings.Contains(sendDesc, "lark_upload_file") {
		t.Fatalf("expected lark_send_message description to mention text-only routing, got %q", sendDesc)
	}

	uploadDesc := NewLarkUploadFile().Definition().Description
	if !strings.Contains(uploadDesc, "file delivery/attachment transfer") || !strings.Contains(uploadDesc, "lark_send_message") {
		t.Fatalf("expected lark_upload_file description to mention explicit file-delivery routing, got %q", uploadDesc)
	}

	calendarQueryDesc := NewLarkCalendarQuery().Definition().Description
	if !strings.Contains(calendarQueryDesc, "calendar schedule/event retrieval") || !strings.Contains(calendarQueryDesc, "not deterministic computation/recalculation") {
		t.Fatalf("expected lark_calendar_query description to mention calendar-only routing, got %q", calendarQueryDesc)
	}
}
