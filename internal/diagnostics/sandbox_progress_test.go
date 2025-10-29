package diagnostics

import (
	"sync"
	"testing"
	"time"
)

func TestPublishAndSubscribeSandboxProgress(t *testing.T) {
	t.Cleanup(ResetSandboxProgressForTests)

	payload := SandboxProgressPayload{
		Status:     SandboxProgressRunning,
		Stage:      "health_check",
		Message:    "Contacting sandbox",
		Step:       1,
		TotalSteps: 2,
		Updated:    time.Now(),
	}

	var wg sync.WaitGroup
	wg.Add(1)

	var received SandboxProgressPayload
	unsubscribe := SubscribeSandboxProgress(func(p SandboxProgressPayload) {
		received = p
		wg.Done()
	})
	defer unsubscribe()

	PublishSandboxProgress(payload)
	wg.Wait()

	if received.Stage != payload.Stage {
		t.Fatalf("expected stage %q, got %q", payload.Stage, received.Stage)
	}
	if received.Status != payload.Status {
		t.Fatalf("expected status %q, got %q", payload.Status, received.Status)
	}
	if received.Step != payload.Step || received.TotalSteps != payload.TotalSteps {
		t.Fatalf("unexpected step info: %#v", received)
	}
}

func TestLatestSandboxProgress(t *testing.T) {
	t.Cleanup(ResetSandboxProgressForTests)

	if _, ok := LatestSandboxProgress(); ok {
		t.Fatalf("expected no progress before publish")
	}

	now := time.Now()
	payload := SandboxProgressPayload{
		Status:  SandboxProgressReady,
		Stage:   "complete",
		Message: "Sandbox ready",
		Updated: now,
	}
	PublishSandboxProgress(payload)

	latest, ok := LatestSandboxProgress()
	if !ok {
		t.Fatalf("expected progress to be present")
	}
	if latest.Status != SandboxProgressReady {
		t.Fatalf("expected ready status, got %q", latest.Status)
	}
	if !latest.Updated.Equal(payload.Updated) {
		t.Fatalf("expected timestamps to match")
	}
}
