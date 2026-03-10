//go:build integration

package pool

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestPool_HandoffBetweenWaitingSessions(t *testing.T) {
	p := New()
	if added := p.Register([]int{11, 12}); added != 2 {
		t.Fatalf("Register added %d panes, want 2", added)
	}

	firstPane, err := p.Acquire(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Acquire session-1: %v", err)
	}
	secondPane, err := p.Acquire(context.Background(), "session-2")
	if err != nil {
		t.Fatalf("Acquire session-2: %v", err)
	}

	type result struct {
		sessionID string
		paneID    int
		err       error
	}
	results := make(chan result, 2)

	for _, sessionID := range []string{"session-3", "session-4"} {
		go func(sessionID string) {
			paneID, err := p.Acquire(context.Background(), sessionID)
			results <- result{sessionID: sessionID, paneID: paneID, err: err}
		}(sessionID)
	}

	time.Sleep(50 * time.Millisecond)
	p.Release(firstPane)
	p.Release(secondPane)

	got := []result{<-results, <-results}
	for _, res := range got {
		if res.err != nil {
			t.Fatalf("Acquire %s: %v", res.sessionID, res.err)
		}
	}

	acquiredPaneIDs := []int{got[0].paneID, got[1].paneID}
	slices.Sort(acquiredPaneIDs)
	if !slices.Equal(acquiredPaneIDs, []int{11, 12}) {
		t.Fatalf("acquired pane IDs = %v, want [11 12]", acquiredPaneIDs)
	}

	slots := p.Slots()
	if len(slots) != 2 {
		t.Fatalf("Slots len = %d, want 2", len(slots))
	}

	busySessions := []string{slots[0].SessionID, slots[1].SessionID}
	slices.Sort(busySessions)
	if !slices.Equal(busySessions, []string{"session-3", "session-4"}) {
		t.Fatalf("busy sessions = %v, want session-3/session-4", busySessions)
	}
	for _, slot := range slots {
		if slot.State != SlotBusy {
			t.Fatalf("slot %+v should be busy after handoff", slot)
		}
	}
}

func TestPool_CancelledWaiterDoesNotStealReleasedPane(t *testing.T) {
	p := New()
	p.Register([]int{21})

	heldPane, err := p.Acquire(context.Background(), "session-1")
	if err != nil {
		t.Fatalf("Acquire session-1: %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(context.Background())
	waitErr := make(chan error, 1)
	go func() {
		_, err := p.Acquire(cancelledCtx, "session-2")
		waitErr <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-waitErr:
		if err == nil {
			t.Fatal("cancelled waiter should return an error")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for cancelled waiter")
	}

	p.Release(heldPane)

	recycledPane, err := p.Acquire(context.Background(), "session-3")
	if err != nil {
		t.Fatalf("Acquire session-3: %v", err)
	}
	if recycledPane != 21 {
		t.Fatalf("recycled pane = %d, want 21", recycledPane)
	}

	slots := p.Slots()
	if len(slots) != 1 {
		t.Fatalf("Slots len = %d, want 1", len(slots))
	}
	if slots[0].SessionID != "session-3" || slots[0].State != SlotBusy {
		t.Fatalf("slot = %+v, want session-3 busy", slots[0])
	}
}
