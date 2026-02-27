package lark

import (
	"context"
	"testing"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
)

func TestSetupListeners_DoesNotAttachSlowProgressSummary(t *testing.T) {
	recorder := NewRecordingMessenger()
	enabled := true
	disabled := false

	gw := &Gateway{
		cfg: Config{
			SlowProgressSummaryEnabled: &enabled,
			SlowProgressSummaryDelay:   20 * time.Millisecond,
			BackgroundProgressEnabled:  &disabled,
		},
		messenger: recorder,
		logger:    logging.NewComponentLogger("test"),
	}
	gw.activeSlots.Store("oc_chat", &sessionSlot{phase: slotRunning})

	msg := &incomingMessage{
		chatID:    "oc_chat",
		messageID: "om_parent",
		isGroup:   true,
	}

	listener, cleanup, _ := gw.setupListeners(context.Background(), msg, &awaitQuestionTracker{})
	defer cleanup()

	listener.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventToolStarted,
		NodeKind:  "tool",
		NodeID:    "call-1",
		Payload: map[string]any{
			"tool_name": "read_file",
		},
	})

	time.Sleep(120 * time.Millisecond)
	if replies := recorder.CallsByMethod("ReplyMessage"); len(replies) != 0 {
		t.Fatalf("expected no slow progress summary replies, got %d", len(replies))
	}
}
