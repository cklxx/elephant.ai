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

func TestPublishEnvironmentsPreservesValues(t *testing.T) {
	payload := EnvironmentPayload{
		Host: map[string]string{
			"API_KEY": "sk-secret-value",
			"PATH":    "/usr/bin",
		},
		Sandbox: map[string]string{
			"token": "my-token",
			"LANG":  "en_US.UTF-8",
		},
		Captured: time.Now(),
	}

	ch := make(chan EnvironmentPayload, 1)
	unsubscribe := SubscribeEnvironments(func(p EnvironmentPayload) {
		ch <- p
	})
	defer unsubscribe()

	PublishEnvironments(payload)

	select {
	case received := <-ch:
		if received.Host["API_KEY"] != "sk-secret-value" {
			t.Fatalf("expected host API_KEY to be preserved, got %q", received.Host["API_KEY"])
		}
		if received.Sandbox["token"] != "my-token" {
			t.Fatalf("expected sandbox token to be preserved, got %q", received.Sandbox["token"])
		}
		if received.Host["PATH"] != "/usr/bin" {
			t.Fatalf("expected non-sensitive host value to remain, got %q", received.Host["PATH"])
		}
		if received.Sandbox["LANG"] != "en_US.UTF-8" {
			t.Fatalf("expected non-sensitive sandbox value to remain, got %q", received.Sandbox["LANG"])
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for environment payload")
	}

	latest, ok := LatestEnvironments()
	if !ok {
		t.Fatal("expected payload to be stored")
	}

	if latest.Host["API_KEY"] != "sk-secret-value" {
		t.Fatalf("expected stored host API_KEY to be preserved, got %q", latest.Host["API_KEY"])
	}
	if latest.Sandbox["token"] != "my-token" {
		t.Fatalf("expected stored sandbox token to be preserved, got %q", latest.Sandbox["token"])
	}
}
