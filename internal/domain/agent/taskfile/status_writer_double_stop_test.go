package taskfile_test

import (
	"sync"
	"testing"

	"alex/internal/domain/agent/taskfile"
)

// TestStatusWriter_ConcurrentStop verifies that calling Stop() concurrently
// from multiple goroutines does NOT panic (K-04/BL-04 regression guard).
func TestStatusWriter_ConcurrentStop(t *testing.T) {
	sw := taskfile.NewStatusWriter("/tmp/status_writer_concurrent_test.yaml", nil)

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
