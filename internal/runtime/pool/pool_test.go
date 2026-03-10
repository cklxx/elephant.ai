package pool

import (
	"context"
	"sync"
	"testing"
	"time"
)

// --- New ---

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New returned nil")
	}
	if p.Size() != 0 {
		t.Errorf("Size = %d, want 0", p.Size())
	}
}

// --- Register ---

func TestRegister(t *testing.T) {
	p := New()
	n := p.Register([]int{10, 20, 30})
	if n != 3 {
		t.Errorf("Register returned %d, want 3", n)
	}
	if p.Size() != 3 {
		t.Errorf("Size = %d, want 3", p.Size())
	}
}

func TestRegister_Duplicates(t *testing.T) {
	p := New()
	p.Register([]int{1, 2})
	n := p.Register([]int{2, 3})
	if n != 1 {
		t.Errorf("Register duplicates returned %d, want 1", n)
	}
	if p.Size() != 3 {
		t.Errorf("Size = %d, want 3", p.Size())
	}
}

func TestRegister_Empty(t *testing.T) {
	p := New()
	n := p.Register(nil)
	if n != 0 {
		t.Errorf("Register nil returned %d, want 0", n)
	}
}

// --- Acquire + Release ---

func TestAcquireAndRelease(t *testing.T) {
	p := New()
	p.Register([]int{100})

	ctx := context.Background()
	paneID, err := p.Acquire(ctx, "session-1")
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if paneID != 100 {
		t.Errorf("paneID = %d, want 100", paneID)
	}

	// Verify slot is busy.
	slots := p.Slots()
	if len(slots) != 1 || slots[0].State != SlotBusy {
		t.Errorf("expected SlotBusy, got %+v", slots)
	}
	if slots[0].SessionID != "session-1" {
		t.Errorf("SessionID = %q, want session-1", slots[0].SessionID)
	}

	// Release and verify idle.
	p.Release(100)

	slots = p.Slots()
	if slots[0].State != SlotIdle {
		t.Errorf("expected SlotIdle after release, got %v", slots[0].State)
	}
	if slots[0].SessionID != "" {
		t.Errorf("SessionID should be empty after release, got %q", slots[0].SessionID)
	}
}

func TestAcquire_ContextCancelled(t *testing.T) {
	p := New()
	// No panes registered → Acquire will block until context cancelled.

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Acquire(ctx, "sess")
	if err == nil {
		t.Fatal("expected error on cancelled context")
	}
}

func TestAcquire_BlocksUntilRelease(t *testing.T) {
	p := New()
	p.Register([]int{1})

	ctx := context.Background()
	// Acquire the only pane.
	paneID, _ := p.Acquire(ctx, "s1")

	acquired := make(chan int, 1)
	go func() {
		id, err := p.Acquire(ctx, "s2")
		if err != nil {
			t.Errorf("second Acquire: %v", err)
		}
		acquired <- id
	}()

	// Give goroutine time to block.
	time.Sleep(20 * time.Millisecond)

	select {
	case <-acquired:
		t.Fatal("second Acquire should block")
	default:
	}

	// Release frees it up.
	p.Release(paneID)

	select {
	case id := <-acquired:
		if id != 1 {
			t.Errorf("second Acquire got %d, want 1", id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second Acquire did not unblock after Release")
	}
}

// --- Release unknown pane ---

func TestRelease_UnknownPane(t *testing.T) {
	p := New()
	p.Register([]int{1})
	// Should not panic.
	p.Release(999)
}

// --- Slots ---

func TestSlots_SortedByPaneID(t *testing.T) {
	p := New()
	p.Register([]int{30, 10, 20})
	slots := p.Slots()
	if len(slots) != 3 {
		t.Fatalf("len = %d, want 3", len(slots))
	}
	for i, want := range []int{10, 20, 30} {
		if slots[i].PaneID != want {
			t.Errorf("slots[%d].PaneID = %d, want %d", i, slots[i].PaneID, want)
		}
	}
}

func TestSlots_ReturnsSnapshot(t *testing.T) {
	p := New()
	p.Register([]int{1})
	slots := p.Slots()
	// Mutating the snapshot should not affect pool.
	slots[0].State = SlotBusy
	fresh := p.Slots()
	if fresh[0].State != SlotIdle {
		t.Error("Slots should return a copy, mutation leaked")
	}
}

// --- Concurrent ---

func TestConcurrent_AcquireRelease(t *testing.T) {
	p := New()
	p.Register([]int{1, 2, 3, 4, 5})

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id, err := p.Acquire(ctx, "sess")
			if err != nil {
				t.Errorf("Acquire %d: %v", i, err)
				return
			}
			// Simulate work.
			time.Sleep(time.Millisecond)
			p.Release(id)
		}(i)
	}
	wg.Wait()

	// All panes should be idle.
	for _, s := range p.Slots() {
		if s.State != SlotIdle {
			t.Errorf("pane %d still busy after all goroutines done", s.PaneID)
		}
	}
}

func TestConcurrent_RegisterAndAcquire(t *testing.T) {
	p := New()

	var wg sync.WaitGroup
	ctx := context.Background()

	// Register in one goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.Register([]int{10, 20, 30})
	}()

	// Wait for registration to complete before acquiring.
	wg.Wait()

	wg.Add(3)
	for i := 0; i < 3; i++ {
		go func() {
			defer wg.Done()
			id, err := p.Acquire(ctx, "s")
			if err != nil {
				t.Errorf("Acquire: %v", err)
				return
			}
			p.Release(id)
		}()
	}
	wg.Wait()
}
