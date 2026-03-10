package server

import (
	"hash/fnv"
	"time"
)

// isDuplicateEvent checks whether an identical event was seen within the dedupe window.
func (h *HooksBridge) isDuplicateEvent(chatID string, payload hookPayload) bool {
	if h == nil || h.dedupeWindow <= 0 {
		return false
	}

	now := h.now()
	fingerprint := hookEventFingerprint(chatID, payload)

	h.dedupeMu.Lock()
	defer h.dedupeMu.Unlock()

	if h.recentFingerprints == nil {
		h.recentFingerprints = make(map[uint64]time.Time)
	}

	h.evictExpiredFingerprints(now)

	if ts, ok := h.recentFingerprints[fingerprint]; ok && now.Sub(ts) <= h.dedupeWindow {
		return true
	}
	h.recentFingerprints[fingerprint] = now

	h.evictOldestIfOverCap(now)
	return false
}

// evictExpiredFingerprints removes fingerprints older than the dedupe window.
// Must be called with dedupeMu held.
func (h *HooksBridge) evictExpiredFingerprints(now time.Time) {
	cutoff := now.Add(-h.dedupeWindow)
	for key, ts := range h.recentFingerprints {
		if ts.Before(cutoff) {
			delete(h.recentFingerprints, key)
		}
	}
}

// evictOldestIfOverCap removes the single oldest fingerprint when the map
// exceeds maxRecentHookFingerprints. Must be called with dedupeMu held.
func (h *HooksBridge) evictOldestIfOverCap(now time.Time) {
	if len(h.recentFingerprints) <= maxRecentHookFingerprints {
		return
	}
	var oldestKey uint64
	oldest := now
	found := false
	for key, ts := range h.recentFingerprints {
		if !found || ts.Before(oldest) {
			oldest = ts
			oldestKey = key
			found = true
		}
	}
	if found {
		delete(h.recentFingerprints, oldestKey)
	}
}

// hookEventFingerprint computes an FNV-64a hash of the event's identity fields.
func hookEventFingerprint(chatID string, payload hookPayload) uint64 {
	hasher := fnv.New64a()
	write := func(s string) {
		_, _ = hasher.Write([]byte(s))
		_, _ = hasher.Write([]byte{0})
	}

	write(chatID)
	write(payload.Event)
	write(payload.SessionID)
	write(payload.ToolName)
	write(truncateHookText(payload.Output, 512))
	write(truncateHookText(payload.Error, 512))
	write(payload.StopReason)
	write(truncateHookText(payload.Answer, 512))
	write(truncateHookText(payload.Thinking, 512))
	if len(payload.ToolInput) > 0 {
		input := payload.ToolInput
		if len(input) > maxFingerprintInputBytes {
			input = input[:maxFingerprintInputBytes]
		}
		_, _ = hasher.Write(input)
	}

	return hasher.Sum64()
}
