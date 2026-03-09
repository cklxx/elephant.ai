package lark

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestEventDedup_DuplicateSkipped(t *testing.T) {
	d := newEventDedup(nil)

	if d.isDuplicate("msg1", "evt1") {
		t.Fatal("first call should not be duplicate")
	}
	if !d.isDuplicate("msg1", "evt1") {
		t.Fatal("second call with same IDs should be duplicate")
	}
}

func TestEventDedup_DuplicateByMessageIDOnly(t *testing.T) {
	d := newEventDedup(nil)

	if d.isDuplicate("msg1", "evt1") {
		t.Fatal("first call should not be duplicate")
	}
	// Same message_id, different event_id — still duplicate.
	if !d.isDuplicate("msg1", "evt2") {
		t.Fatal("same message_id with different event_id should be duplicate")
	}
}

func TestEventDedup_DuplicateByEventIDOnly(t *testing.T) {
	d := newEventDedup(nil)

	if d.isDuplicate("msg1", "evt1") {
		t.Fatal("first call should not be duplicate")
	}
	// Same event_id, different message_id — still duplicate.
	if !d.isDuplicate("msg2", "evt1") {
		t.Fatal("same event_id with different message_id should be duplicate")
	}
}

func TestEventDedup_TTLExpiry(t *testing.T) {
	now := time.Now()
	d := newEventDedup(nil)
	d.now = func() time.Time { return now }

	if d.isDuplicate("msg1", "evt1") {
		t.Fatal("first call should not be duplicate")
	}

	// Advance time past TTL.
	d.now = func() time.Time { return now.Add(eventDedupTTL + time.Second) }

	if d.isDuplicate("msg1", "evt1") {
		t.Fatal("after TTL expiry, same IDs should be allowed again")
	}
}

func TestEventDedup_Sweep(t *testing.T) {
	now := time.Now()
	d := newEventDedup(nil)
	d.now = func() time.Time { return now }

	d.isDuplicate("msg1", "evt1")
	d.isDuplicate("msg2", "evt2")

	// Advance past TTL and sweep.
	d.now = func() time.Time { return now.Add(eventDedupTTL + time.Second) }
	d.sweep()

	// Entries should be cleaned up — both IDs re-processable.
	if d.isDuplicate("msg1", "evt1") {
		t.Fatal("msg1/evt1 should be processable after sweep")
	}
	if d.isDuplicate("msg2", "evt2") {
		t.Fatal("msg2/evt2 should be processable after sweep")
	}
}

func TestEventDedup_EmptyIDs(t *testing.T) {
	d := newEventDedup(nil)

	if d.isDuplicate("", "") {
		t.Fatal("empty IDs should never be duplicate")
	}
	if d.isDuplicate("", "") {
		t.Fatal("empty IDs should never be duplicate even on repeat")
	}
}

func TestEventDedup_ConcurrencySafety(t *testing.T) {
	d := newEventDedup(nil)

	var wg sync.WaitGroup
	const goroutines = 100

	// Half the goroutines use the same ID, half use unique IDs.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				d.isDuplicate("shared-msg", "shared-evt")
			} else {
				d.isDuplicate("msg-"+string(rune('A'+i)), "evt-"+string(rune('A'+i)))
			}
		}(i)
	}
	wg.Wait()

	// Verify shared ID is now a duplicate.
	if !d.isDuplicate("shared-msg", "shared-evt") {
		t.Fatal("shared ID should be duplicate after concurrent writes")
	}
}

func TestEventDedup_CleanupGoroutine(t *testing.T) {
	now := time.Now()
	d := newEventDedup(nil)
	d.now = func() time.Time { return now }
	d.ttl = 10 * time.Millisecond

	d.isDuplicate("msg1", "evt1")

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	d.startCleanup(ctx, &wg)

	// Wait for at least one cleanup cycle.
	time.Sleep(50 * time.Millisecond)
	d.now = func() time.Time { return now.Add(20 * time.Millisecond) }
	time.Sleep(100 * time.Millisecond)

	cancel()
	wg.Wait()

	// After cleanup, the entry should be gone (or expired).
	d.now = func() time.Time { return now.Add(20 * time.Millisecond) }
	if d.isDuplicate("msg1", "evt1") {
		t.Fatal("entry should be processable after cleanup goroutine ran")
	}
}
