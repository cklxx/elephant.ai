package lark

import (
	"context"
	"sync"
	"time"

	"alex/internal/shared/logging"
)

const (
	eventDedupTTL          = 5 * time.Minute
	eventDedupCleanupEvery = 1 * time.Minute
)

// eventDedup is a TTL-based in-memory deduplicator for Lark webhook events.
// It keys on both message_id and event_id to catch exact message replays as
// well as duplicate webhook deliveries for the same logical event.
type eventDedup struct {
	entries sync.Map // key → expireAt (time.Time)
	now     func() time.Time
	ttl     time.Duration
	logger  logging.Logger
}

func newEventDedup(logger logging.Logger) *eventDedup {
	return &eventDedup{
		now:    time.Now,
		ttl:    eventDedupTTL,
		logger: logging.OrNop(logger),
	}
}

// startCleanup launches a background goroutine that periodically removes
// expired entries. Cancel the context to stop it.
func (d *eventDedup) startCleanup(ctx context.Context, wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(eventDedupCleanupEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				d.sweep()
			}
		}
	}()
}

// isDuplicate returns true if the (messageID, eventID) pair has been seen
// within the TTL window. When it returns false it atomically records the pair
// so that subsequent calls with the same IDs are detected as duplicates.
//
// The check-and-mark is atomic per key via sync.Map.LoadOrStore, preventing
// the TOCTOU race where two concurrent calls both pass a Load check before
// either calls Store.
func (d *eventDedup) isDuplicate(messageID, eventID string) bool {
	if messageID == "" && eventID == "" {
		return false
	}

	now := d.now()
	expireAt := now.Add(d.ttl)

	// Check event_id first (primary dedup key from Feishu).
	// LoadOrStore atomically checks and inserts if absent.
	if eventID != "" {
		if v, loaded := d.entries.LoadOrStore(eventID, expireAt); loaded {
			if now.Before(v.(time.Time)) {
				return true
			}
			// Expired entry — overwrite with fresh expiry.
			d.entries.Store(eventID, expireAt)
		}
	}

	// Check message_id (covers cases where event_id changes on replay).
	if messageID != "" {
		if v, loaded := d.entries.LoadOrStore(messageID, expireAt); loaded {
			if now.Before(v.(time.Time)) {
				return true
			}
			d.entries.Store(messageID, expireAt)
		}
	}

	return false
}

// sweep removes all expired entries.
func (d *eventDedup) sweep() {
	now := d.now()
	var removed int
	d.entries.Range(func(key, value any) bool {
		if now.After(value.(time.Time)) {
			d.entries.Delete(key)
			removed++
		}
		return true
	})
	if removed > 0 {
		d.logger.Debug("eventDedup sweep: removed %d expired entries", removed)
	}
}
