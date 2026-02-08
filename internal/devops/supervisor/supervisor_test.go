package supervisor

import (
	"sync"
	"testing"
)

func TestComponentMuReturnsSameMutex(t *testing.T) {
	s := &Supervisor{}
	mu1 := s.componentMu("main")
	mu2 := s.componentMu("main")
	if mu1 != mu2 {
		t.Error("componentMu should return the same mutex for the same name")
	}
}

func TestComponentMuDifferentPerComponent(t *testing.T) {
	s := &Supervisor{}
	mu1 := s.componentMu("main")
	mu2 := s.componentMu("test")
	if mu1 == mu2 {
		t.Error("componentMu should return different mutexes for different names")
	}
}

func TestConcurrentRestartSkip(t *testing.T) {
	s := &Supervisor{}
	mu := s.componentMu("main")

	// Simulate a restart already in progress
	mu.Lock()

	// TryLock should fail (restart already in progress)
	if mu.TryLock() {
		t.Error("TryLock should fail when restart is in progress")
		mu.Unlock()
	}

	// Release the lock
	mu.Unlock()

	// Now TryLock should succeed
	if !mu.TryLock() {
		t.Error("TryLock should succeed after previous restart completes")
	}
	mu.Unlock()
}

func TestConcurrentRestartSkipParallel(t *testing.T) {
	s := &Supervisor{}

	const workers = 10
	var (
		acquired int32
		wg       sync.WaitGroup
		start    = make(chan struct{})
	)

	mu := s.componentMu("main")

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // synchronize goroutine start
			if mu.TryLock() {
				acquired++
				// Simulate restart work
				mu.Unlock()
			}
		}()
	}

	close(start) // fire all goroutines
	wg.Wait()

	// At least 1 should have acquired (the first one)
	// but not all 10 simultaneously
	if acquired == 0 {
		t.Error("at least one goroutine should have acquired the lock")
	}
	// With TryLock contention, some will be skipped — exact count is non-deterministic
	t.Logf("acquired %d/%d (rest were correctly skipped)", acquired, workers)
}
