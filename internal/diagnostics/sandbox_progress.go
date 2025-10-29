package diagnostics

import (
	"sync"
	"time"
)

type SandboxProgressStatus string

const (
	SandboxProgressPending SandboxProgressStatus = "pending"
	SandboxProgressRunning SandboxProgressStatus = "running"
	SandboxProgressReady   SandboxProgressStatus = "ready"
	SandboxProgressError   SandboxProgressStatus = "error"
)

type SandboxProgressPayload struct {
	Status     SandboxProgressStatus
	Stage      string
	Message    string
	Step       int
	TotalSteps int
	Error      string
	Updated    time.Time
}

type SandboxProgressListener func(SandboxProgressPayload)

var (
	sandboxProgressMu       sync.RWMutex
	latestSandboxProgress   SandboxProgressPayload
	sandboxProgressPresent  bool
	sandboxProgressListener = map[int]SandboxProgressListener{}
	sandboxProgressSeq      int
)

// PublishSandboxProgress stores the latest sandbox progress and notifies subscribers.
func PublishSandboxProgress(payload SandboxProgressPayload) {
	clone := payload // struct copy is sufficient

	sandboxProgressMu.Lock()
	latestSandboxProgress = clone
	sandboxProgressPresent = true

	callbacks := make([]SandboxProgressListener, 0, len(sandboxProgressListener))
	for _, listener := range sandboxProgressListener {
		callbacks = append(callbacks, listener)
	}
	sandboxProgressMu.Unlock()

	for _, listener := range callbacks {
		if listener != nil {
			listener(clone)
		}
	}
}

// SubscribeSandboxProgress registers a listener for sandbox progress updates.
// The returned function unsubscribes the listener when invoked.
func SubscribeSandboxProgress(listener SandboxProgressListener) func() {
	sandboxProgressMu.Lock()
	defer sandboxProgressMu.Unlock()

	sandboxProgressSeq++
	id := sandboxProgressSeq
	sandboxProgressListener[id] = listener

	// Immediately replay the latest progress snapshot so new subscribers have state.
	if sandboxProgressPresent && listener != nil {
		go listener(latestSandboxProgress)
	}

	return func() {
		sandboxProgressMu.Lock()
		defer sandboxProgressMu.Unlock()
		delete(sandboxProgressListener, id)
	}
}

// LatestSandboxProgress returns the most recent sandbox progress payload if one exists.
func LatestSandboxProgress() (SandboxProgressPayload, bool) {
	sandboxProgressMu.RLock()
	defer sandboxProgressMu.RUnlock()

	if !sandboxProgressPresent {
		return SandboxProgressPayload{}, false
	}
	return latestSandboxProgress, true
}

// ResetSandboxProgressForTests clears sandbox progress state. Intended for use in tests only.
func ResetSandboxProgressForTests() {
	sandboxProgressMu.Lock()
	defer sandboxProgressMu.Unlock()

	latestSandboxProgress = SandboxProgressPayload{}
	sandboxProgressPresent = false
	sandboxProgressListener = map[int]SandboxProgressListener{}
	sandboxProgressSeq = 0
}
