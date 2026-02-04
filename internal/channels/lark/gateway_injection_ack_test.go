package lark

import (
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/channels"
)

func TestGatewayInjectUserInput_AcksWithReactionOnSuccess(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(&stubExecutor{}, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	inputCh := make(chan agent.UserInput, 1)
	msg := &incomingMessage{
		chatID:    "oc_chat_1",
		messageID: "om_msg_1",
		senderID:  "ou_user_1",
		content:   "hello",
		isGroup:   false,
	}

	gw.injectUserInput(inputCh, "sess_1", msg)

	select {
	case <-inputCh:
	default:
		t.Fatal("expected injected user input to be enqueued")
	}

	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if len(rec.CallsByMethod("AddReaction")) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	reactions := rec.CallsByMethod("AddReaction")
	if len(reactions) != 1 {
		t.Fatalf("expected 1 AddReaction call, got %d", len(reactions))
	}
	if reactions[0].MsgID != msg.messageID {
		t.Fatalf("expected reaction on msg_id=%q, got %q", msg.messageID, reactions[0].MsgID)
	}
	if reactions[0].Emoji != "THINKING" {
		t.Fatalf("expected THINKING reaction, got %q", reactions[0].Emoji)
	}
}

func TestGatewayInjectUserInput_NoAckWhenChannelFull(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(&stubExecutor{}, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	// Unbuffered channel with no receiver: non-blocking send should fail.
	inputCh := make(chan agent.UserInput)
	msg := &incomingMessage{
		chatID:    "oc_chat_1",
		messageID: "om_msg_drop",
		senderID:  "ou_user_1",
		content:   "hello",
		isGroup:   false,
	}

	gw.injectUserInput(inputCh, "sess_1", msg)
	time.Sleep(50 * time.Millisecond)

	reactions := rec.CallsByMethod("AddReaction")
	if len(reactions) != 0 {
		t.Fatalf("expected no reactions when injection is dropped, got %d", len(reactions))
	}
}

