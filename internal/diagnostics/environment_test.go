package diagnostics

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestPublishAndLatestEnvironment(t *testing.T) {
	payload := EnvironmentPayload{
		Host:     map[string]string{"A": "1"},
		Sandbox:  map[string]string{"B": "2"},
		Captured: time.Now(),
	}

	PublishEnvironments(payload)
	latest, ok := LatestEnvironments()
	if !ok {
		t.Fatalf("expected latest payload")
	}
	if latest.Host["A"] != "1" || latest.Sandbox["B"] != "2" {
		t.Fatalf("unexpected payload contents: %+v", latest)
	}

	latest.Host["A"] = "mutated"
	again, ok := LatestEnvironments()
	if !ok {
		t.Fatalf("expected latest payload after mutation")
	}
	if again.Host["A"] != "1" {
		t.Fatalf("expected clone to protect stored payload")
	}
}

func TestSubscribeEnvironmentsReceivesUpdates(t *testing.T) {
	var count atomic.Int32
	unsubscribe := SubscribeEnvironments(func(EnvironmentPayload) {
		count.Add(1)
	})
	defer unsubscribe()

	PublishEnvironments(EnvironmentPayload{Captured: time.Now()})
	if count.Load() != 1 {
		t.Fatalf("expected listener to receive update, got %d", count.Load())
	}
}
