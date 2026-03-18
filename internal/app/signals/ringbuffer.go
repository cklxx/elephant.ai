package signals

import "sync"

// RingBuffer is a fixed-size, thread-safe ring buffer for SignalEvents.
// When full, the oldest event is overwritten.
type RingBuffer struct {
	mu    sync.Mutex
	items []SignalEvent
	head  int
	count int
}

// NewRingBuffer creates a RingBuffer with the given capacity.
func NewRingBuffer(size int) *RingBuffer {
	if size <= 0 {
		size = 1
	}
	return &RingBuffer{items: make([]SignalEvent, size)}
}

// Push adds an event. If full, the oldest event is dropped.
func (rb *RingBuffer) Push(event SignalEvent) {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	idx := (rb.head + rb.count) % len(rb.items)
	rb.items[idx] = event
	if rb.count == len(rb.items) {
		rb.head = (rb.head + 1) % len(rb.items)
	} else {
		rb.count++
	}
}

// Drain returns all buffered events (oldest first) and resets the buffer.
func (rb *RingBuffer) Drain() []SignalEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.count == 0 {
		return nil
	}
	out := make([]SignalEvent, rb.count)
	for i := range rb.count {
		out[i] = rb.items[(rb.head+i)%len(rb.items)]
	}
	rb.head = 0
	rb.count = 0
	return out
}

// Len returns the number of buffered events.
func (rb *RingBuffer) Len() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

// Snapshot returns a copy of all buffered events without draining.
func (rb *RingBuffer) Snapshot() []SignalEvent {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.count == 0 {
		return nil
	}
	out := make([]SignalEvent, rb.count)
	for i := range rb.count {
		out[i] = rb.items[(rb.head+i)%len(rb.items)]
	}
	return out
}
