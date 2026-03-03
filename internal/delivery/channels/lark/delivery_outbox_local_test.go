package lark

import (
	"context"
	"testing"
	"time"
)

func TestDeliveryOutboxLocalStore_DedupClaimRetryAndSent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewDeliveryOutboxMemoryStore()

	firstBatch, err := store.Enqueue(ctx, []DeliveryIntent{{
		ChatID:         "chat-1",
		RunID:          "run-1",
		EventType:      "result_final",
		Sequence:       1,
		IdempotencyKey: "k1",
		MsgType:        "text",
		Content:        `{"text":"ok"}`,
	}})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if len(firstBatch) != 1 {
		t.Fatalf("expected 1 intent, got %d", len(firstBatch))
	}
	if firstBatch[0].Status != DeliveryIntentPending {
		t.Fatalf("expected pending status, got %s", firstBatch[0].Status)
	}

	dupBatch, err := store.Enqueue(ctx, []DeliveryIntent{{
		ChatID:         "chat-1",
		RunID:          "run-1",
		EventType:      "result_final",
		Sequence:       1,
		IdempotencyKey: "k1",
		MsgType:        "text",
		Content:        `{"text":"ok"}`,
	}})
	if err != nil {
		t.Fatalf("Enqueue duplicate error = %v", err)
	}
	if len(dupBatch) != 1 {
		t.Fatalf("expected duplicate lookup to return 1 intent, got %d", len(dupBatch))
	}
	if dupBatch[0].IntentID != firstBatch[0].IntentID {
		t.Fatalf("expected duplicate to reuse intent id %q, got %q", firstBatch[0].IntentID, dupBatch[0].IntentID)
	}

	now := time.Now()
	claimed, err := store.ClaimPending(ctx, 10, now)
	if err != nil {
		t.Fatalf("ClaimPending() error = %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected 1 claimed intent, got %d", len(claimed))
	}
	if claimed[0].Status != DeliveryIntentSending {
		t.Fatalf("expected sending status, got %s", claimed[0].Status)
	}
	if claimed[0].AttemptCount != 1 {
		t.Fatalf("expected attempt count 1, got %d", claimed[0].AttemptCount)
	}

	next := now.Add(50 * time.Millisecond)
	if err := store.MarkRetry(ctx, claimed[0].IntentID, next, "temporary"); err != nil {
		t.Fatalf("MarkRetry() error = %v", err)
	}

	noClaim, err := store.ClaimPending(ctx, 10, now)
	if err != nil {
		t.Fatalf("ClaimPending before retry time error = %v", err)
	}
	if len(noClaim) != 0 {
		t.Fatalf("expected no claim before next_attempt_at, got %d", len(noClaim))
	}

	retryClaim, err := store.ClaimPending(ctx, 10, next.Add(10*time.Millisecond))
	if err != nil {
		t.Fatalf("ClaimPending retry window error = %v", err)
	}
	if len(retryClaim) != 1 {
		t.Fatalf("expected retry claim count 1, got %d", len(retryClaim))
	}
	if retryClaim[0].AttemptCount != 2 {
		t.Fatalf("expected attempt count 2 after retry, got %d", retryClaim[0].AttemptCount)
	}

	sentAt := next.Add(20 * time.Millisecond)
	if err := store.MarkSent(ctx, retryClaim[0].IntentID, sentAt); err != nil {
		t.Fatalf("MarkSent() error = %v", err)
	}

	stored, ok, err := store.GetByIdempotencyKey(ctx, "k1")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected intent to exist")
	}
	if stored.Status != DeliveryIntentSent {
		t.Fatalf("expected sent status, got %s", stored.Status)
	}
}

func TestDeliveryOutboxLocalStore_FilePersistsAcrossReload(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	dir := t.TempDir()

	store, err := NewDeliveryOutboxFileStore(dir)
	if err != nil {
		t.Fatalf("NewDeliveryOutboxFileStore() error = %v", err)
	}
	batch, err := store.Enqueue(ctx, []DeliveryIntent{{
		ChatID:         "chat-1",
		RunID:          "run-1",
		EventType:      "result_final",
		Sequence:       1,
		IdempotencyKey: "persist-key",
		MsgType:        "text",
		Content:        `{"text":"persist"}`,
	}})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("expected 1 inserted intent, got %d", len(batch))
	}

	reloaded, err := NewDeliveryOutboxFileStore(dir)
	if err != nil {
		t.Fatalf("reload NewDeliveryOutboxFileStore() error = %v", err)
	}
	got, ok, err := reloaded.GetByIdempotencyKey(ctx, "persist-key")
	if err != nil {
		t.Fatalf("GetByIdempotencyKey() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected persisted intent to exist")
	}
	if got.IntentID == "" {
		t.Fatalf("expected intent_id to persist")
	}
}

func TestDeliveryOutboxLocalStore_ReplayDeadIntent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	store := NewDeliveryOutboxMemoryStore()

	batch, err := store.Enqueue(ctx, []DeliveryIntent{{
		ChatID:         "chat-2",
		RunID:          "run-2",
		EventType:      "result_failed",
		Sequence:       1,
		IdempotencyKey: "dead-key",
		MsgType:        "text",
		Content:        `{"text":"failed"}`,
	}})
	if err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}
	if len(batch) != 1 {
		t.Fatalf("expected 1 inserted intent, got %d", len(batch))
	}

	claimed, err := store.ClaimPending(ctx, 1, time.Now())
	if err != nil {
		t.Fatalf("ClaimPending() error = %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected one claim, got %d", len(claimed))
	}
	if err := store.MarkDead(ctx, claimed[0].IntentID, "permanent"); err != nil {
		t.Fatalf("MarkDead() error = %v", err)
	}

	replayed, err := store.Replay(ctx, ReplayFilter{IntentIDs: []string{claimed[0].IntentID}, Limit: 1})
	if err != nil {
		t.Fatalf("Replay() error = %v", err)
	}
	if replayed != 1 {
		t.Fatalf("expected replay count 1, got %d", replayed)
	}

	reclaimed, err := store.ClaimPending(ctx, 1, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("ClaimPending() after replay error = %v", err)
	}
	if len(reclaimed) != 1 {
		t.Fatalf("expected one reclaimed intent, got %d", len(reclaimed))
	}
}
