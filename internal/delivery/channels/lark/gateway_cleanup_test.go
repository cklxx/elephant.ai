package lark

import (
	"testing"
	"time"
)

func TestGatewayPruneActiveSlots_RemovesExpiredAndCaps(t *testing.T) {
	now := time.Now()
	gw := &Gateway{
		cfg: Config{
			ActiveSlotTTL:        time.Minute,
			ActiveSlotMaxEntries: 1,
		},
		now: func() time.Time { return now },
	}

	gw.activeSlots.Store("running", &sessionSlot{phase: slotRunning, lastTouched: now.Add(-10 * time.Hour)})
	gw.activeSlots.Store("expired", &sessionSlot{phase: slotIdle, lastTouched: now.Add(-2 * time.Hour)})
	gw.activeSlots.Store("idle-a", &sessionSlot{phase: slotIdle, lastTouched: now.Add(-2 * time.Second)})
	gw.activeSlots.Store("idle-b", &sessionSlot{phase: slotIdle, lastTouched: now.Add(-1 * time.Second)})

	removed := gw.pruneActiveSlots(now)
	if removed < 2 {
		t.Fatalf("expected at least two slot removals, got %d", removed)
	}
	if _, ok := gw.activeSlots.Load("running"); !ok {
		t.Fatal("expected running slot to be retained")
	}
	if _, ok := gw.activeSlots.Load("expired"); ok {
		t.Fatal("expected expired idle slot to be removed")
	}
}

func TestGatewayPrunePendingInputRelays_RemovesExpiredAndCaps(t *testing.T) {
	now := time.Now()
	gw := &Gateway{
		cfg: Config{
			PendingInputRelayMaxChats:   1,
			PendingInputRelayMaxPerChat: 1,
		},
		now: func() time.Time { return now },
	}

	q1 := &pendingRelayQueue{}
	q1.Push(&pendingInputRelay{taskID: "expired", requestID: "r-exp", createdAt: 1, expiresAt: now.Add(-time.Second).UnixNano()})
	q1.Push(&pendingInputRelay{taskID: "a", requestID: "r-a", createdAt: 2, expiresAt: now.Add(time.Minute).UnixNano()})
	q1.Push(&pendingInputRelay{taskID: "b", requestID: "r-b", createdAt: 3, expiresAt: now.Add(time.Minute).UnixNano()})

	q2 := &pendingRelayQueue{}
	q2.Push(&pendingInputRelay{taskID: "c", requestID: "r-c", createdAt: 4, expiresAt: now.Add(time.Minute).UnixNano()})

	gw.pendingInputRelays.Store("chat-1", q1)
	gw.pendingInputRelays.Store("chat-2", q2)

	removed := gw.prunePendingInputRelays(now)
	if removed < 2 {
		t.Fatalf("expected at least two relay removals, got %d", removed)
	}

	chatCount := 0
	gw.pendingInputRelays.Range(func(_, value any) bool {
		chatCount++
		queue := value.(*pendingRelayQueue)
		if queue.Len() > 1 {
			t.Fatalf("expected per-chat queue capped to 1, got %d", queue.Len())
		}
		return true
	})
	if chatCount != 1 {
		t.Fatalf("expected pending relay chats capped to 1, got %d", chatCount)
	}
}
