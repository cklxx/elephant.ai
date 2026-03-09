// Package pool provides a fixed-size pane pool for the Kaku runtime.
// Instead of splitting a new pane for every CC session, sessions acquire
// an idle pane from the pool and release it on completion for reuse.
package pool

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// SlotState represents the current state of a pool slot.
type SlotState string

const (
	SlotIdle SlotState = "idle"
	SlotBusy SlotState = "busy"
)

// Slot tracks one pane in the pool.
type Slot struct {
	PaneID    int       `json:"pane_id"`
	State     SlotState `json:"state"`
	SessionID string    `json:"session_id,omitempty"`
}

// Pool manages a fixed set of pre-allocated Kaku panes.
type Pool struct {
	mu    sync.Mutex
	slots map[int]*Slot
	avail chan int // buffered channel of idle pane IDs
}

// New creates an empty Pool. Call Register to add pane IDs.
func New() *Pool {
	return &Pool{
		slots: make(map[int]*Slot),
		avail: make(chan int, 64),
	}
}

// Register adds pane IDs to the pool and marks them idle.
// Returns the number of newly registered panes (already-known IDs are skipped).
func (p *Pool) Register(paneIDs []int) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	added := 0
	for _, id := range paneIDs {
		if _, exists := p.slots[id]; exists {
			continue
		}
		p.slots[id] = &Slot{PaneID: id, State: SlotIdle}
		p.avail <- id
		added++
	}
	return added
}

// Size returns the total number of slots in the pool.
func (p *Pool) Size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.slots)
}

// Acquire blocks until an idle pane is available, marks it busy for sessionID,
// and returns the pane ID. Returns an error if ctx is cancelled while waiting.
func (p *Pool) Acquire(ctx context.Context, sessionID string) (int, error) {
	select {
	case <-ctx.Done():
		return 0, fmt.Errorf("pool: acquire cancelled: %w", ctx.Err())
	case paneID := <-p.avail:
		p.mu.Lock()
		slot := p.slots[paneID]
		slot.State = SlotBusy
		slot.SessionID = sessionID
		p.mu.Unlock()
		return paneID, nil
	}
}

// Release returns a pane to the pool, marking it idle.
// No-op if the pane ID is not in the pool.
func (p *Pool) Release(paneID int) {
	p.mu.Lock()
	slot, ok := p.slots[paneID]
	if !ok {
		p.mu.Unlock()
		return
	}
	slot.State = SlotIdle
	slot.SessionID = ""
	p.mu.Unlock()

	p.avail <- paneID
}

// Slots returns a snapshot of all slots, sorted by pane ID.
func (p *Pool) Slots() []Slot {
	p.mu.Lock()
	defer p.mu.Unlock()

	out := make([]Slot, 0, len(p.slots))
	for _, s := range p.slots {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].PaneID < out[j].PaneID })
	return out
}
