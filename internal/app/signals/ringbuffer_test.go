package signals

import (
	"sync"
	"testing"
)

func TestRingBufferBasic(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		pushIDs  []string
		wantIDs  []string
		wantLen  int
	}{
		{
			name:    "under capacity",
			size:    5,
			pushIDs: []string{"a", "b"},
			wantIDs: []string{"a", "b"},
			wantLen: 2,
		},
		{
			name:    "at capacity",
			size:    3,
			pushIDs: []string{"a", "b", "c"},
			wantIDs: []string{"a", "b", "c"},
			wantLen: 3,
		},
		{
			name:    "overflow drops oldest",
			size:    3,
			pushIDs: []string{"a", "b", "c", "d", "e"},
			wantIDs: []string{"c", "d", "e"},
			wantLen: 3,
		},
		{
			name:    "zero size treated as 1",
			size:    0,
			pushIDs: []string{"a", "b"},
			wantIDs: []string{"b"},
			wantLen: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rb := NewRingBuffer(tt.size)
			for _, id := range tt.pushIDs {
				rb.Push(SignalEvent{ID: id})
			}
			if rb.Len() != tt.wantLen {
				t.Fatalf("Len() = %d, want %d", rb.Len(), tt.wantLen)
			}
			got := rb.Drain()
			if len(got) != len(tt.wantIDs) {
				t.Fatalf("Drain() len = %d, want %d", len(got), len(tt.wantIDs))
			}
			for i, want := range tt.wantIDs {
				if got[i].ID != want {
					t.Errorf("Drain()[%d].ID = %q, want %q", i, got[i].ID, want)
				}
			}
			if rb.Len() != 0 {
				t.Error("Len() after Drain() should be 0")
			}
		})
	}
}

func TestRingBufferSnapshot(t *testing.T) {
	rb := NewRingBuffer(3)
	rb.Push(SignalEvent{ID: "a"})
	rb.Push(SignalEvent{ID: "b"})
	snap := rb.Snapshot()
	if len(snap) != 2 {
		t.Fatalf("Snapshot() len = %d, want 2", len(snap))
	}
	if rb.Len() != 2 {
		t.Error("Snapshot should not drain")
	}
}

func TestRingBufferEmptyDrain(t *testing.T) {
	rb := NewRingBuffer(5)
	if got := rb.Drain(); got != nil {
		t.Errorf("empty Drain() = %v, want nil", got)
	}
}

func TestRingBufferConcurrent(t *testing.T) {
	rb := NewRingBuffer(100)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(base int) {
			defer wg.Done()
			for j := range 50 {
				rb.Push(SignalEvent{ID: string(rune('A' + base*50 + j))})
			}
		}(i)
	}
	wg.Wait()
	if rb.Len() != 100 {
		t.Errorf("after concurrent writes, Len() = %d, want 100", rb.Len())
	}
}
