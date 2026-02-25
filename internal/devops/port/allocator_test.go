package port

import (
	"sync"
	"testing"
)

func TestAllocatorReservePreferred(t *testing.T) {
	a := NewAllocator()

	port, err := a.Reserve("test-svc", 18999)
	if err != nil {
		t.Fatalf("Reserve error: %v", err)
	}
	if port != 18999 {
		t.Errorf("port = %d, want 18999", port)
	}

	// Same service, same port = idempotent
	port2, err := a.Reserve("test-svc", 18999)
	if err != nil {
		t.Fatalf("Reserve idempotent error: %v", err)
	}
	if port2 != 18999 {
		t.Errorf("port = %d, want 18999", port2)
	}

	// Different service, same port = error
	_, err = a.Reserve("other-svc", 18999)
	if err == nil {
		t.Error("expected error for conflicting reservation")
	}
}

func TestAllocatorReserveRandom(t *testing.T) {
	a := NewAllocator()

	port, err := a.Reserve("test-svc", 0)
	if err != nil {
		t.Fatalf("Reserve error: %v", err)
	}
	if port < 20000 || port > 45000 {
		t.Errorf("random port %d out of range [20000, 45000]", port)
	}
}

func TestAllocatorRelease(t *testing.T) {
	a := NewAllocator()

	if _, err := a.Reserve("test-svc", 18998); err != nil {
		t.Fatalf("Reserve error: %v", err)
	}
	a.Release("test-svc")

	if !a.IsAvailable(18998) {
		t.Error("port should be available after release")
	}
}

func TestAllocatorConcurrentReserve(t *testing.T) {
	a := NewAllocator()
	var wg sync.WaitGroup
	results := make(chan int, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			port, err := a.Reserve("", 0) // random port
			if err != nil {
				return
			}
			results <- port
		}(i)
	}

	wg.Wait()
	close(results)

	ports := make(map[int]bool)
	for port := range results {
		if ports[port] {
			t.Errorf("duplicate port %d assigned", port)
		}
		ports[port] = true
	}
}
