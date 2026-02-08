package supervisor

import (
	"os"
	"path/filepath"
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

func TestReadLoopState(t *testing.T) {
	dir := t.TempDir()

	s := &Supervisor{
		tmpDir:   dir,
		mainRoot: dir, // getMainSHA will fail gracefully → "unknown"
	}

	// Write loop state JSON
	stateJSON := `{
  "cycle_phase": "fast_gate",
  "cycle_result": "running",
  "last_error": "test restart failed"
}`
	os.WriteFile(filepath.Join(dir, "lark-loop.state.json"), []byte(stateJSON), 0o644)

	// Write last processed SHA
	os.WriteFile(filepath.Join(dir, "lark-loop.last"), []byte("6f608251\n"), 0o644)

	// Write last validated SHA
	os.WriteFile(filepath.Join(dir, "lark-loop.last-validated"), []byte("5a5b5c5d\n"), 0o644)

	s.readLoopState()

	if s.loopState.CyclePhase != "fast_gate" {
		t.Errorf("CyclePhase = %q, want fast_gate", s.loopState.CyclePhase)
	}
	if s.loopState.CycleResult != "running" {
		t.Errorf("CycleResult = %q, want running", s.loopState.CycleResult)
	}
	if s.loopState.LastError != "test restart failed" {
		t.Errorf("LastError = %q, want 'test restart failed'", s.loopState.LastError)
	}
	if s.loopState.LastProcessedSHA != "6f608251" {
		t.Errorf("LastProcessedSHA = %q, want 6f608251", s.loopState.LastProcessedSHA)
	}
	if s.loopState.LastValidatedSHA != "5a5b5c5d" {
		t.Errorf("LastValidatedSHA = %q, want 5a5b5c5d", s.loopState.LastValidatedSHA)
	}
	// MainSHA will be "unknown" since dir is not a git repo
	if s.loopState.MainSHA != "unknown" {
		t.Errorf("MainSHA = %q, want unknown", s.loopState.MainSHA)
	}
}

func TestReadLoopStateMissingFiles(t *testing.T) {
	dir := t.TempDir()

	s := &Supervisor{
		tmpDir:   dir,
		mainRoot: dir,
	}

	// No files written — should gracefully degrade
	s.readLoopState()

	if s.loopState.CyclePhase != "" {
		t.Errorf("CyclePhase = %q, want empty", s.loopState.CyclePhase)
	}
	if s.loopState.CycleResult != "" {
		t.Errorf("CycleResult = %q, want empty", s.loopState.CycleResult)
	}
	if s.loopState.LastError != "" {
		t.Errorf("LastError = %q, want empty", s.loopState.LastError)
	}
	if s.loopState.LastProcessedSHA != "" {
		t.Errorf("LastProcessedSHA = %q, want empty", s.loopState.LastProcessedSHA)
	}
	if s.loopState.LastValidatedSHA != "" {
		t.Errorf("LastValidatedSHA = %q, want empty", s.loopState.LastValidatedSHA)
	}
	if s.loopState.MainSHA != "unknown" {
		t.Errorf("MainSHA = %q, want unknown", s.loopState.MainSHA)
	}
}
