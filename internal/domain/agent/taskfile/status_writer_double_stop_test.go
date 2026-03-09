package taskfile

import (
	"sync"
	"testing"
	"time"
)

// TestStatusWriter_ConcurrentStop verifies that calling Stop() concurrently
// from multiple goroutines does NOT panic (K-04/BL-04 regression guard).
func TestStatusWriter_ConcurrentStop(t *testing.T) {
	sw := newStatusWriter(t.TempDir()+"/status_writer_concurrent_test.yaml", nil)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			sw.Stop()
		}()
	}
	wg.Wait()
	// If we reach here without a panic, the guard works.
}

// TestStatusWriter_StopSafeWithPolling exercises a Stop call after polling
// starts, and a repeated Stop call from a second goroutine.
func TestStatusWriter_StopSafeWithPolling(t *testing.T) {
	statusPath := t.TempDir() + "/status_writer_polling.yaml"
	tf := &TaskFile{
		Version: "1",
		PlanID:  "poll-test",
		Tasks: []TaskSpec{
			{ID: "a", Prompt: "do a"},
		},
	}
	sw := newStatusWriter(statusPath, nil)
	if err := sw.InitFromTaskFile(tf); err != nil {
		t.Fatalf("InitFromTaskFile: %v", err)
	}

	sw.StartPolling(&mockDispatcher{}, []string{"a"}, 5*time.Millisecond)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond)
		sw.Stop()
	}()
	go func() {
		defer wg.Done()
		time.Sleep(20 * time.Millisecond)
		sw.Stop()
	}()
	wg.Wait()
}
