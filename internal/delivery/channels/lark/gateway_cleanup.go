package lark

import (
	"context"
	"sort"
	"time"
)

func (g *Gateway) setCleanupCancel(cancel context.CancelFunc) {
	g.cleanupMu.Lock()
	if g.cleanupCancel != nil {
		g.cleanupCancel()
	}
	g.cleanupCancel = cancel
	g.cleanupMu.Unlock()
}

func (g *Gateway) stopStateCleanupLoop() {
	g.cleanupMu.Lock()
	cancel := g.cleanupCancel
	g.cleanupCancel = nil
	g.cleanupMu.Unlock()
	if cancel != nil {
		cancel()
	}
	g.cleanupWG.Wait()
}

func (g *Gateway) startStateCleanupLoop(ctx context.Context) {
	interval := g.cfg.StateCleanupInterval
	if interval <= 0 {
		return
	}
	g.cleanupWG.Add(1)
	go func() {
		defer g.cleanupWG.Done()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				g.cleanupRuntimeState()
			}
		}
	}()
}

func (g *Gateway) cleanupRuntimeState() {
	now := g.currentTime()
	trimmedSlots := g.pruneActiveSlots(now)
	trimmedRelays := g.prunePendingInputRelays(now)

	trimmedAISessions := 0
	if g.aiCoordinator != nil && g.cfg.AIChatSessionTTL > 0 {
		trimmedAISessions = g.aiCoordinator.CleanupExpiredSessions(g.cfg.AIChatSessionTTL)
	}
	if trimmedSlots > 0 || trimmedRelays > 0 || trimmedAISessions > 0 {
		g.logger.Info(
			"Lark state cleanup: removed_slots=%d removed_relays=%d removed_ai_chat_sessions=%d",
			trimmedSlots, trimmedRelays, trimmedAISessions,
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

func (g *Gateway) prunePendingInputRelays(now time.Time) int {
	maxChats := g.cfg.PendingInputRelayMaxChats
	maxPerChat := g.cfg.PendingInputRelayMaxPerChat

	type queueMeta struct {
		chatID   string
		oldestAt int64
	}
	totalChats := 0
	removed := 0
	var metas []queueMeta

	g.pendingInputRelays.Range(func(key, value any) bool {
		chatID, ok := key.(string)
		if !ok {
			g.pendingInputRelays.Delete(key)
			removed++
			return true
		}
		queue, ok := value.(*pendingRelayQueue)
		if !ok || queue == nil {
			g.pendingInputRelays.Delete(chatID)
			removed++
			return true
		}

		removed += queue.PruneExpired(now)
		if maxPerChat > 0 {
			removed += queue.TrimToMax(maxPerChat)
		}
		if queue.Len() == 0 {
			g.pendingInputRelays.Delete(chatID)
			removed++
			return true
		}

		totalChats++
		metas = append(metas, queueMeta{
			chatID:   chatID,
			oldestAt: queue.OldestCreatedAtUnixNano(),
		})
		return true
	})

	if maxChats <= 0 || totalChats <= maxChats {
		return removed
	}
	sort.Slice(metas, func(i, j int) bool {
		if metas[i].oldestAt == metas[j].oldestAt {
			return metas[i].chatID < metas[j].chatID
		}
		return metas[i].oldestAt < metas[j].oldestAt
	})
	need := totalChats - maxChats
	for i := 0; i < len(metas) && need > 0; i++ {
		g.pendingInputRelays.Delete(metas[i].chatID)
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
