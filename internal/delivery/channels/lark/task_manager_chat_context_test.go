package lark

import (
	"context"
	"strings"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestEnrichWithChatContextInjectsStructuredChunkBeforeTask(t *testing.T) {
	rec := NewRecordingMessenger()
	rec.ListMessagesResult = []*larkim.Message{
		makeTextMessage("m2", "1706500060000", "app", "bot_1", "assistant reply"),
		makeTextMessage("m1", "1706500000000", "user", "ou_1", "user asks"),
	}
	gw := &Gateway{
		messenger: rec,
		cfg: Config{
			AutoChatContextSize: 20,
		},
	}

	got := gw.enrichWithChatContext(context.Background(), "execute task", &incomingMessage{
		chatID:    "oc_1",
		messageID: "m3",
	}, false)

	if !strings.HasPrefix(got, larkHistoryChunkHeader+"\n") {
		t.Fatalf("expected task to start with lark history chunk header, got %q", got)
	}
	if strings.Contains(got, "[近期对话]") {
		t.Fatalf("expected legacy chinese history marker to be removed, got %q", got)
	}
	if !strings.Contains(got, "1 | role=user | sender=user(ou_1)") {
		t.Fatalf("expected structured user history line, got %q", got)
	}
	if !strings.Contains(got, "2 | role=assistant | sender=bot(bot_1)") {
		t.Fatalf("expected structured assistant history line, got %q", got)
	}
	if !strings.HasSuffix(got, "\n\nexecute task") {
		t.Fatalf("expected original task to remain after history chunk, got %q", got)
	}
}

func TestEnrichWithChatContextAppendsChunkForPlanReview(t *testing.T) {
	rec := NewRecordingMessenger()
	rec.ListMessagesResult = []*larkim.Message{
		makeTextMessage("m1", "1706500000000", "user", "ou_1", "plan review question"),
	}
	gw := &Gateway{
		messenger: rec,
		cfg: Config{
			AutoChatContextSize: 20,
		},
	}

	got := gw.enrichWithChatContext(context.Background(), "plan feedback block", &incomingMessage{
		chatID:    "oc_1",
		messageID: "m2",
	}, true)

	if !strings.HasPrefix(got, "plan feedback block\n\n"+larkHistoryChunkHeader+"\n") {
		t.Fatalf("expected plan feedback task to append history chunk, got %q", got)
	}
}
