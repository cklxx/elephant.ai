package lark

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

// TestEventDedup_ConcurrentStress_100Goroutines_SameEventID verifies that when
// 100 goroutines simultaneously call isDuplicate with the same eventID, exactly
// one goroutine sees it as non-duplicate (the winner), and all 99 others see it
// as duplicate. This exercises the sync.Map.LoadOrStore atomicity guarantee.
func TestEventDedup_ConcurrentStress_100Goroutines_SameEventID(t *testing.T) {
	const goroutines = 100
	const trials = 20

	for trial := 0; trial < trials; trial++ {
		d := newEventDedup(nil)
		eventID := fmt.Sprintf("evt-stress-%d", trial)
		msgID := fmt.Sprintf("msg-stress-%d", trial)

		var processed int64
		var wg sync.WaitGroup
		barrier := make(chan struct{})

		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-barrier
				if !d.isDuplicate(msgID, eventID) {
					atomic.AddInt64(&processed, 1)
				}
			}()
		}

		close(barrier)
		wg.Wait()

		if processed != 1 {
			t.Fatalf("trial %d: expected exactly 1 goroutine to process event, got %d", trial, processed)
		}
	}
}

// TestEventDedup_ConcurrentStress_MixedEventIDs fires 100 goroutines where
// 50 share event-A and 50 share event-B. Each group should have exactly 1 winner.
func TestEventDedup_ConcurrentStress_MixedEventIDs(t *testing.T) {
	const goroutinesPerGroup = 50
	const trials = 20

	for trial := 0; trial < trials; trial++ {
		d := newEventDedup(nil)

		var processedA, processedB int64
		var wg sync.WaitGroup
		barrier := make(chan struct{})

		for i := 0; i < goroutinesPerGroup; i++ {
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-barrier
				if !d.isDuplicate("msg-a", "evt-a") {
					atomic.AddInt64(&processedA, 1)
				}
			}()
			go func() {
				defer wg.Done()
				<-barrier
				if !d.isDuplicate("msg-b", "evt-b") {
					atomic.AddInt64(&processedB, 1)
				}
			}()
		}

		close(barrier)
		wg.Wait()

		if processedA != 1 {
			t.Fatalf("trial %d: event-A processed by %d goroutines, want 1", trial, processedA)
		}
		if processedB != 1 {
			t.Fatalf("trial %d: event-B processed by %d goroutines, want 1", trial, processedB)
		}
	}
}

// TestEventDedup_ConcurrentSweepDuringInsert runs isDuplicate and sweep
// concurrently to verify there are no data races on the sync.Map.
func TestEventDedup_ConcurrentSweepDuringInsert(t *testing.T) {
	d := newEventDedup(nil)
	const goroutines = 100

	var wg sync.WaitGroup
	barrier := make(chan struct{})

	// Half the goroutines insert new entries.
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			d.isDuplicate(fmt.Sprintf("msg-%d", i), fmt.Sprintf("evt-%d", i))
		}(i)
	}

	// Half the goroutines sweep concurrently.
	for i := 0; i < goroutines/2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-barrier
			d.sweep()
		}()
	}

	close(barrier)
	wg.Wait()
}
