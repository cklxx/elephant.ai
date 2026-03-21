package lark

import (
	"sort"
	"time"
)

func (g *Gateway) cleanupRuntimeState() {
	now := g.currentTime()
	trimmedSlots := g.pruneActiveSlots(now)
	g.pruneActiveChatSlots()
	g.evictExpiredPromptCache()
	g.evictExpiredChatContexts()
	trimmedAISessions := 0
	if g.aiCoordinator != nil && g.cfg.AIChatSessionTTL > 0 {
		trimmedAISessions = g.aiCoordinator.CleanupExpiredSessions(g.cfg.AIChatSessionTTL)
	}
	if trimmedSlots > 0 || trimmedAISessions > 0 {
		g.logger.Info(
			"Lark state cleanup: removed_slots=%d removed_ai_chat_sessions=%d",
			trimmedSlots, trimmedAISessions,
		)
	}
}

func (g *Gateway) pruneActiveSlots(now time.Time) int {
	ttl := g.cfg.ActiveSlotTTL
	maxEntries := g.cfg.ActiveSlotMaxEntries

	type slotMeta struct {
		chatID      string
		lastTouched time.Time
	}
	var idle []slotMeta
	total := 0
	removed := 0

	g.activeSlots.Range(func(key, value any) bool {
		chatID, ok := key.(string)
		if !ok {
			g.activeSlots.Delete(key)
			removed++
			return true
		}
		slot, ok := value.(*sessionSlot)
		if !ok || slot == nil {
			g.activeSlots.Delete(chatID)
			removed++
			return true
		}
		total++

		slot.mu.Lock()
		phase := slot.phase
		lastTouched := slot.lastTouched
		slot.mu.Unlock()

		if phase == slotRunning {
			return true
		}
		if ttl > 0 && !lastTouched.IsZero() && now.Sub(lastTouched) > ttl {
			g.activeSlots.Delete(chatID)
			removed++
			return true
		}
		idle = append(idle, slotMeta{chatID: chatID, lastTouched: lastTouched})
		return true
	})

	current := total - removed
	if maxEntries <= 0 || current <= maxEntries {
		return removed
	}

	sort.Slice(idle, func(i, j int) bool {
		if idle[i].lastTouched.Equal(idle[j].lastTouched) {
			return idle[i].chatID < idle[j].chatID
		}
		return idle[i].lastTouched.Before(idle[j].lastTouched)
	})

	need := current - maxEntries
	for i := 0; i < len(idle) && need > 0; i++ {
		g.activeSlots.Delete(idle[i].chatID)
		removed++
		need--
	}
	return removed
}

func (g *Gateway) currentTime() time.Time {
	nowFn := g.now
	if nowFn == nil {
		nowFn = time.Now
	}
	return nowFn()
}

func (g *Gateway) pruneActiveChatSlots() {
	g.activeChatSlots.Range(func(k, v any) bool {
		m, ok := v.(*chatSlotMap)
		if !ok || m == nil {
			g.activeChatSlots.Delete(k)
			return true
		}
		m.removeIdle()
		m.mu.Lock()
		empty := len(m.slots) == 0
		m.mu.Unlock()
		if empty {
			g.activeChatSlots.Delete(k)
		}
		return true
	})
}
