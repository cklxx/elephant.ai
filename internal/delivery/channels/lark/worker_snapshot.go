package lark

import "time"

// workerSnapshot captures the current state of a running worker for use by
// the conversation process. The snapshot is read-only and assembled from the
// sessionSlot under its mutex, so the conversation process never needs to
// hold the slot lock beyond a single read.
type workerSnapshot struct {
	Phase          slotPhase
	TaskDesc       string
	Elapsed        time.Duration
	Signals        []string // recent user messages injected into the worker
	RecentProgress []string // recent tool progress entries from the worker
}

// snapshotWorker reads the current sessionSlot state and returns a
// point-in-time workerSnapshot. Must NOT be called while holding slot.mu.
func (g *Gateway) snapshotWorker(chatID string) workerSnapshot {
	raw, ok := g.activeSlots.Load(chatID)
	if !ok {
		return workerSnapshot{Phase: slotIdle}
	}
	slot, ok := raw.(*sessionSlot)
	if !ok || slot == nil {
		return workerSnapshot{Phase: slotIdle}
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()

	snap := workerSnapshot{
		Phase:    slot.phase,
		TaskDesc: slot.taskDesc,
	}
	if slot.phase == slotRunning {
		snap.Elapsed = g.currentTime().Sub(slot.lastTouched)
	}
	if len(slot.recentProgress) > 0 {
		snap.RecentProgress = make([]string, len(slot.recentProgress))
		copy(snap.RecentProgress, slot.recentProgress)
	}
	return snap
}

// IsIdle reports whether no worker task is running.
func (s workerSnapshot) IsIdle() bool { return s.Phase == slotIdle }

// IsRunning reports whether a worker task is currently active.
func (s workerSnapshot) IsRunning() bool { return s.Phase == slotRunning }

// StatusSummary returns a short human-readable description of the worker state
// suitable for inclusion in an LLM prompt.
func (s workerSnapshot) StatusSummary() string {
	switch s.Phase {
	case slotRunning:
		desc := s.TaskDesc
		if len([]rune(desc)) > 80 {
			desc = string([]rune(desc)[:80]) + "…"
		}
		summary := "有任务在执行中, 任务描述: " + desc + ", 已运行: " + s.Elapsed.Truncate(time.Second).String()
		if len(s.RecentProgress) > 0 {
			summary += "\n最近进展："
			for _, p := range s.RecentProgress {
				summary += "\n- " + p
			}
		}
		return summary
	case slotAwaitingInput:
		return "任务等待用户输入中"
	default:
		return "idle"
	}
}
