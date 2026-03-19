package lark

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// workerSnapshot captures the current state of a running worker for use by
// the conversation process. The snapshot is read-only and assembled from the
// sessionSlot under its mutex, so the conversation process never needs to
// hold the slot lock beyond a single read.
type workerSnapshot struct {
	Phase          slotPhase
	TaskID         string   // "#1", "#2", etc.; empty for legacy single-worker path
	TaskDesc       string
	Elapsed        time.Duration
	Signals        []string // recent user messages injected into the worker
	RecentProgress []string // recent tool progress entries from the worker
	Lang           string   // "zh" or "en", populated from msg.content
	ResultPreview  string   // truncated answer from completed task; empty when running
}

// workerSnapshotList holds snapshots of all workers in a chat (multi-worker mode).
type workerSnapshotList struct {
	Snapshots []workerSnapshot
	Lang      string
}

// detectLang returns "en" if the text is predominantly ASCII, "zh" otherwise.
// Uses a fast unicode scan — no LLM call.
func detectLang(text string) string {
	if text == "" {
		return "zh"
	}
	cjk := 0
	latin := 0
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hangul, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
			cjk++
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			latin++
		}
	}
	if latin > cjk {
		return "en"
	}
	return "zh"
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
		TaskID:   slot.taskID,
		TaskDesc: slot.taskDesc,
	}
	if slot.phase == slotRunning {
		ref := slot.taskStartTime
		if ref.IsZero() {
			ref = slot.lastTouched
		}
		snap.Elapsed = g.currentTime().Sub(ref)
	}
	if len(slot.recentProgress) > 0 {
		snap.RecentProgress = make([]string, len(slot.recentProgress))
		copy(snap.RecentProgress, slot.recentProgress)
	}
	return snap
}

// snapshotAllWorkers returns snapshots of all active workers for a chat
// from the activeChatSlots registry (conversation-process mode).
func (g *Gateway) snapshotAllWorkers(chatID string, lang string) workerSnapshotList {
	raw, ok := g.activeChatSlots.Load(chatID)
	if !ok {
		return workerSnapshotList{Lang: lang}
	}
	m, ok := raw.(*chatSlotMap)
	if !ok || m == nil {
		return workerSnapshotList{Lang: lang}
	}

	now := g.currentTime()
	var snaps []workerSnapshot
	m.forEachSlot(func(taskID string, s *sessionSlot) {
		s.mu.Lock()
		defer s.mu.Unlock()
		// Include idle slots only if they have a result preview (for cross-task references).
		if s.phase == slotIdle && s.lastResultPreview == "" {
			return
		}
		snap := workerSnapshot{
			Phase:         s.phase,
			TaskID:        taskID,
			TaskDesc:      s.taskDesc,
			Lang:          lang,
			ResultPreview: s.lastResultPreview,
		}
		if s.phase == slotRunning {
			ref := s.taskStartTime
			if ref.IsZero() {
				ref = s.lastTouched
			}
			snap.Elapsed = now.Sub(ref)
		}
		if len(s.recentProgress) > 0 {
			snap.RecentProgress = make([]string, len(s.recentProgress))
			copy(snap.RecentProgress, s.recentProgress)
		}
		snaps = append(snaps, snap)
	})
	return workerSnapshotList{Snapshots: snaps, Lang: lang}
}

// IsIdle reports whether no worker task is running.
func (s workerSnapshot) IsIdle() bool { return s.Phase == slotIdle }

// IsRunning reports whether a worker task is currently active.
func (s workerSnapshot) IsRunning() bool { return s.Phase == slotRunning }

// StatusSummary returns a short human-readable description of the worker state
// suitable for inclusion in an LLM prompt. Pass lang="" to default to "zh".
func (s workerSnapshot) StatusSummary(lang string) string {
	if lang == "" {
		lang = "zh"
	}
	taskLabel := s.TaskID
	if taskLabel == "" {
		taskLabel = "task"
	}
	switch s.Phase {
	case slotRunning:
		desc := s.TaskDesc
		if len([]rune(desc)) > 80 {
			desc = string([]rune(desc)[:80]) + "…"
		}
		var summary string
		elapsed := s.Elapsed.Truncate(time.Second).String()
		if lang == "en" {
			summary = fmt.Sprintf("%s running (%s): %s", taskLabel, elapsed, desc)
		} else {
			summary = fmt.Sprintf("%s 执行中（已运行 %s）：%s", taskLabel, elapsed, desc)
		}
		if len(s.RecentProgress) > 0 {
			if lang == "en" {
				summary += "\nRecent progress:"
			} else {
				summary += "\n最近进展："
			}
			for _, p := range s.RecentProgress {
				summary += "\n- " + p
			}
		}
		return summary
	case slotAwaitingInput:
		if lang == "en" {
			return fmt.Sprintf("%s waiting for user input", taskLabel)
		}
		return fmt.Sprintf("%s 等待用户输入中", taskLabel)
	default:
		// Idle with result preview — show completed task output for cross-task references.
		if s.ResultPreview != "" {
			if lang == "en" {
				return fmt.Sprintf("%s done: %s\nResult: %s", taskLabel, s.TaskDesc, s.ResultPreview)
			}
			return fmt.Sprintf("%s 已完成：%s\n结果：%s", taskLabel, s.TaskDesc, s.ResultPreview)
		}
		return "idle"
	}
}

// StatusSummary returns a combined summary of all workers for LLM consumption.
func (l workerSnapshotList) StatusSummary() string {
	if len(l.Snapshots) == 0 {
		return "idle"
	}
	lang := l.Lang
	if lang == "" {
		lang = "zh"
	}

	var active, completed int
	var parts []string
	for _, s := range l.Snapshots {
		parts = append(parts, s.StatusSummary(lang))
		if s.Phase == slotIdle {
			completed++
		} else {
			active++
		}
	}

	var header string
	switch {
	case active > 0 && completed > 0:
		if lang == "en" {
			header = fmt.Sprintf("%d task(s) active, %d completed:", active, completed)
		} else {
			header = fmt.Sprintf("%d 个任务执行中，%d 个已完成：", active, completed)
		}
	case active > 0:
		if lang == "en" {
			header = fmt.Sprintf("%d task(s) active:", active)
		} else {
			header = fmt.Sprintf("%d 个任务执行中：", active)
		}
	default:
		if lang == "en" {
			header = fmt.Sprintf("%d completed task(s):", completed)
		} else {
			header = fmt.Sprintf("%d 个任务已完成：", completed)
		}
	}
	return header + "\n" + strings.Join(parts, "\n")
}
