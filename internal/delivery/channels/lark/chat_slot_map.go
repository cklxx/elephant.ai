package lark

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// chatSlotMap manages N concurrent sessionSlots for a single chat in
// conversation-process mode. Task IDs are monotonically incrementing (#1, #2…)
// per chat and reset when /new is called.
type chatSlotMap struct {
	mu     sync.Mutex
	slots  map[string]*sessionSlot // taskID → slot
	nextID int
}

// allocateSlotIfCapacity atomically checks capacity and claims a new slot.
// Returns (slot, taskID, activeCount, true) on success, or (nil, "", 0, false) if at capacity.
// activeCount includes the newly allocated slot.
// The returned slot's taskID field is already set; the caller must further
// initialize phase/inputCh/taskCancel before launching the goroutine.
func (m *chatSlotMap) allocateSlotIfCapacity(max int, now time.Time) (*sessionSlot, string, int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Count active (non-idle) slots.
	active := 0
	for _, s := range m.slots {
		s.mu.Lock()
		if s.phase != slotIdle {
			active++
		}
		s.mu.Unlock()
	}
	if active >= max {
		return nil, "", 0, false
	}

	m.nextID++
	taskID := fmt.Sprintf("#%d", m.nextID)
	slot := &sessionSlot{}
	slot.taskID = taskID
	slot.lastTouched = now
	m.slots[taskID] = slot
	return slot, taskID, active + 1, true
}

// stopAll cancels all running workers. Pass intentional=true for user-initiated
// stops (suppresses the cancellation error reply).
func (m *chatSlotMap) stopAll(intentional bool) {
	m.mu.Lock()
	var toCancel []struct {
		token  uint64
		cancel context.CancelFunc
		slot   *sessionSlot
	}
	for _, s := range m.slots {
		s.mu.Lock()
		if s.phase == slotRunning && s.taskCancel != nil {
			if intentional {
				s.intentionalCancelToken = s.taskToken
			}
			toCancel = append(toCancel, struct {
				token  uint64
				cancel context.CancelFunc
				slot   *sessionSlot
			}{s.taskToken, s.taskCancel, s})
		}
		s.mu.Unlock()
	}
	m.mu.Unlock()

	for _, entry := range toCancel {
		entry.cancel()
	}
}

// stopByTaskID cancels the worker with the given task ID.
// Returns true if a running worker was found and cancelled.
func (m *chatSlotMap) stopByTaskID(taskID string) bool {
	m.mu.Lock()
	s, ok := m.slots[taskID]
	m.mu.Unlock()
	if !ok {
		return false
	}

	s.mu.Lock()
	if s.phase != slotRunning || s.taskCancel == nil {
		s.mu.Unlock()
		return false
	}
	s.intentionalCancelToken = s.taskToken
	cancel := s.taskCancel
	s.mu.Unlock()

	cancel()
	return true
}

// forEachSlot calls fn for each slot. The slotMap lock is NOT held during fn.
func (m *chatSlotMap) forEachSlot(fn func(taskID string, s *sessionSlot)) {
	m.mu.Lock()
	type entry struct {
		id   string
		slot *sessionSlot
	}
	entries := make([]entry, 0, len(m.slots))
	for id, s := range m.slots {
		entries = append(entries, entry{id, s})
	}
	m.mu.Unlock()

	for _, e := range entries {
		fn(e.id, e.slot)
	}
}

// removeIdle deletes idle slots from the map to reclaim memory.
func (m *chatSlotMap) removeIdle() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.slots {
		s.mu.Lock()
		idle := s.phase == slotIdle
		s.mu.Unlock()
		if idle {
			delete(m.slots, id)
		}
	}
}
