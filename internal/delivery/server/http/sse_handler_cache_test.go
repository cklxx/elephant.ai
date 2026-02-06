package http

import "testing"

func TestRunSeqLRUEvictsOldest(t *testing.T) {
	cache := newRunSeqLRU(2)
	cache.Set("run-a", 1)
	cache.Set("run-b", 2)
	cache.Set("run-c", 3)

	if _, ok := cache.Get("run-a"); ok {
		t.Fatal("expected run-a to be evicted")
	}
	if val, ok := cache.Get("run-b"); !ok || val != 2 {
		t.Fatalf("expected run-b to remain, got %v %v", val, ok)
	}
	if val, ok := cache.Get("run-c"); !ok || val != 3 {
		t.Fatalf("expected run-c to remain, got %v %v", val, ok)
	}
}
