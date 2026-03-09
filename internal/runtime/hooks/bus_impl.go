package hooks

import (
	"sync"
	"sync/atomic"
)

const busChannelBuffer = 64

// InProcessBus is the in-process implementation of Bus.
// It uses buffered channels and a fan-out model: each published event is sent
// to all matching per-session subscribers and all wildcard (SubscribeAll) subscribers.
type InProcessBus struct {
	mu      sync.RWMutex
	subs    map[string]map[uint64]chan Event // sessionID → subID → ch
	wilds   map[uint64]chan Event            // wildcard subs (SubscribeAll)
	nextID  atomic.Uint64
}

// NewInProcessBus creates a ready-to-use in-process event bus.
func NewInProcessBus() *InProcessBus {
	return &InProcessBus{
		subs:  make(map[string]map[uint64]chan Event),
		wilds: make(map[uint64]chan Event),
	}
}

// Publish sends ev to every subscriber of sessionID and every wildcard subscriber.
// Delivery is non-blocking: if a subscriber's buffer is full, the event is dropped
// for that subscriber only.
func (b *InProcessBus) Publish(sessionID string, ev Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Fan-out to per-session subscribers.
	for _, ch := range b.subs[sessionID] {
		select {
		case ch <- ev:
		default:
		}
	}

	// Fan-out to wildcard subscribers.
	for _, ch := range b.wilds {
		select {
		case ch <- ev:
		default:
		}
	}
}

// Subscribe returns a buffered channel that receives events for sessionID.
// The caller must invoke the returned cancel function to unsubscribe.
func (b *InProcessBus) Subscribe(sessionID string) (<-chan Event, func()) {
	id := b.nextID.Add(1)
	ch := make(chan Event, busChannelBuffer)

	b.mu.Lock()
	if b.subs[sessionID] == nil {
		b.subs[sessionID] = make(map[uint64]chan Event)
	}
	b.subs[sessionID][id] = ch
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		if m := b.subs[sessionID]; m != nil {
			delete(m, id)
			if len(m) == 0 {
				delete(b.subs, sessionID)
			}
		}
		b.mu.Unlock()
	}
	return ch, cancel
}

// SubscribeAll returns a buffered channel that receives events for every session.
// The caller must invoke the returned cancel function to unsubscribe.
func (b *InProcessBus) SubscribeAll() (<-chan Event, func()) {
	id := b.nextID.Add(1)
	ch := make(chan Event, busChannelBuffer)

	b.mu.Lock()
	b.wilds[id] = ch
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.wilds, id)
		b.mu.Unlock()
	}
	return ch, cancel
}
