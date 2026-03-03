package lark

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"alex/internal/shared/logging"
)

func TestDispatchTerminalIntent_OutboxModeQueuesThenWorkerSends(t *testing.T) {
	t.Parallel()

	store := NewDeliveryOutboxMemoryStore()
	rec := NewRecordingMessenger()
	now := time.Now().UTC()

	gw := &Gateway{
		cfg: Config{
			DeliveryMode: string(DeliveryModeOutbox),
			DeliveryWorker: DeliveryWorkerConfig{
				Enabled:      true,
				BatchSize:    10,
				MaxAttempts:  3,
				BaseBackoff:  5 * time.Millisecond,
				MaxBackoff:   100 * time.Millisecond,
				JitterRatio:  0,
				PollInterval: 10 * time.Millisecond,
			},
		},
		messenger:           rec,
		logger:              logging.Nop(),
		deliveryOutboxStore: store,
		now:                 func() time.Time { return now },
	}

	gw.dispatchTerminalIntent(context.Background(), DeliveryIntent{
		ChatID:         "chat-1",
		RunID:          "run-1",
		EventType:      "result_final",
		Sequence:       1,
		IdempotencyKey: "intent-k1",
		MsgType:        "text",
		Content:        `{"text":"hello"}`,
	})
	now = now.Add(time.Second)

	if calls := rec.Calls(); len(calls) != 0 {
		t.Fatalf("expected no immediate message in outbox mode, got %d calls", len(calls))
	}

	stored, ok, err := store.GetByIdempotencyKey(context.Background(), "intent-k1")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected intent to be enqueued")
	}
	if stored.Status != DeliveryIntentPending {
		t.Fatalf("expected pending status, got %s", stored.Status)
	}

	if processed := gw.processDeliveryOutbox(context.Background()); processed != 1 {
		t.Fatalf("expected 1 processed intent, got %d", processed)
	}
	if sends := rec.CallsByMethod(MethodSendMessage); len(sends) != 1 {
		t.Fatalf("expected 1 send call, got %d", len(sends))
	}

	stored, ok, err = store.GetByIdempotencyKey(context.Background(), "intent-k1")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey() post-send error = %v", err)
	}
	if !ok {
		t.Fatalf("expected intent to still exist")
	}
	if stored.Status != DeliveryIntentSent {
		t.Fatalf("expected sent status, got %s", stored.Status)
	}
}

func TestProcessDeliveryOutbox_RetryThenSuccess(t *testing.T) {
	t.Parallel()

	store := NewDeliveryOutboxMemoryStore()
	rec := NewRecordingMessenger()
	now := time.Now().UTC()

	gw := &Gateway{
		cfg: Config{
			DeliveryMode: string(DeliveryModeOutbox),
			DeliveryWorker: DeliveryWorkerConfig{
				Enabled:      true,
				BatchSize:    10,
				MaxAttempts:  3,
				BaseBackoff:  10 * time.Millisecond,
				MaxBackoff:   100 * time.Millisecond,
				JitterRatio:  0,
				PollInterval: 10 * time.Millisecond,
			},
		},
		messenger:           rec,
		logger:              logging.Nop(),
		deliveryOutboxStore: store,
		now:                 func() time.Time { return now },
	}

	gw.dispatchTerminalIntent(context.Background(), DeliveryIntent{
		ChatID:         "chat-1",
		RunID:          "run-2",
		EventType:      "result_final",
		Sequence:       1,
		IdempotencyKey: "intent-k2",
		MsgType:        "text",
		Content:        `{"text":"hello"}`,
	})
	now = now.Add(time.Second)

	rec.NextError = errors.New("503 service unavailable")
	if processed := gw.processDeliveryOutbox(context.Background()); processed != 1 {
		t.Fatalf("expected first cycle to process 1 intent, got %d", processed)
	}

	stored, ok, err := store.GetByIdempotencyKey(context.Background(), "intent-k2")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected intent to exist")
	}
	if stored.Status != DeliveryIntentRetrying {
		t.Fatalf("expected retrying status, got %s", stored.Status)
	}
	if stored.AttemptCount != 1 {
		t.Fatalf("expected attempt count 1, got %d", stored.AttemptCount)
	}

	if processed := gw.processDeliveryOutbox(context.Background()); processed != 0 {
		t.Fatalf("expected no processing before retry window, got %d", processed)
	}

	now = now.Add(20 * time.Millisecond)
	if processed := gw.processDeliveryOutbox(context.Background()); processed != 1 {
		t.Fatalf("expected second cycle to process 1 intent, got %d", processed)
	}

	stored, ok, err = store.GetByIdempotencyKey(context.Background(), "intent-k2")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey() post-retry error = %v", err)
	}
	if !ok {
		t.Fatalf("expected intent to exist")
	}
	if stored.Status != DeliveryIntentSent {
		t.Fatalf("expected sent status after retry, got %s", stored.Status)
	}
	if stored.AttemptCount != 2 {
		t.Fatalf("expected attempt count 2 after retry, got %d", stored.AttemptCount)
	}
}

func TestProcessDeliveryOutbox_NonRetryableBecomesDead(t *testing.T) {
	t.Parallel()

	store := NewDeliveryOutboxMemoryStore()
	rec := NewRecordingMessenger()
	now := time.Now().UTC()

	gw := &Gateway{
		cfg: Config{
			DeliveryMode: string(DeliveryModeOutbox),
			DeliveryWorker: DeliveryWorkerConfig{
				Enabled:      true,
				BatchSize:    10,
				MaxAttempts:  3,
				BaseBackoff:  10 * time.Millisecond,
				MaxBackoff:   100 * time.Millisecond,
				JitterRatio:  0,
				PollInterval: 10 * time.Millisecond,
			},
		},
		messenger:           rec,
		logger:              logging.Nop(),
		deliveryOutboxStore: store,
		now:                 func() time.Time { return now },
	}

	gw.dispatchTerminalIntent(context.Background(), DeliveryIntent{
		ChatID:         "chat-1",
		RunID:          "run-3",
		EventType:      "result_final",
		Sequence:       1,
		IdempotencyKey: "intent-k3",
		MsgType:        "text",
		Content:        `{"text":"hello"}`,
	})
	now = now.Add(time.Second)

	rec.NextError = errors.New("400 bad request")
	if processed := gw.processDeliveryOutbox(context.Background()); processed != 1 {
		t.Fatalf("expected one processed intent, got %d", processed)
	}

	stored, ok, err := store.GetByIdempotencyKey(context.Background(), "intent-k3")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected intent to exist")
	}
	if stored.Status != DeliveryIntentDead {
		t.Fatalf("expected dead status, got %s", stored.Status)
	}
}

func TestDeliverIntent_PostInvalidPayloadFallbacksToText(t *testing.T) {
	t.Parallel()

	rec := NewRecordingMessenger()
	rec.NextError = errors.New("lark reply message error: code=230001 msg=invalid message content, ext=message_content_text_tag's text field can't be nil")
	gw := &Gateway{
		messenger: rec,
		logger:    logging.Nop(),
	}

	intent := DeliveryIntent{
		ChatID:           "chat-1",
		ReplyToMessageID: "om-parent",
		MsgType:          "post",
		Content:          buildPostContent("## 标题\n\n正文"),
	}
	if err := gw.deliverIntent(context.Background(), intent); err != nil {
		t.Fatalf("deliverIntent() error = %v", err)
	}

	replies := rec.CallsByMethod(MethodReplyMessage)
	if len(replies) != 2 {
		t.Fatalf("expected 2 reply attempts (post then text fallback), got %d", len(replies))
	}
	if replies[0].MsgType != "post" {
		t.Fatalf("first reply should be post, got %s", replies[0].MsgType)
	}
	if replies[1].MsgType != "text" {
		t.Fatalf("second reply should be text fallback, got %s", replies[1].MsgType)
	}
	if !strings.Contains(replies[1].Content, `"text"`) {
		t.Fatalf("fallback content should be text payload JSON, got %q", replies[1].Content)
	}
}
