package filestore

import (
	"sort"
	"time"
)

// EvictByTTL removes entries older than maxAge from items.
// ageFn extracts the timestamp to compare against.
// Returns the number of evicted entries.
func EvictByTTL[K comparable, V any](items map[K]V, now time.Time, maxAge time.Duration, ageFn func(V) time.Time) int {
	cutoff := now.Add(-maxAge)
	evicted := 0
	for k, v := range items {
		if ageFn(v).Before(cutoff) {
			delete(items, k)
			evicted++
		}
	}
	return evicted
}

// EvictByCap removes the oldest entries when len(items) exceeds maxCap.
// ageFn extracts the sort timestamp (oldest first = earliest evicted).
// Returns the number of evicted entries.
func EvictByCap[K comparable, V any](items map[K]V, maxCap int, ageFn func(V) time.Time) int {
	if len(items) <= maxCap {
		return 0
	}

	type entry struct {
		key K
		ts  time.Time
	}
	entries := make([]entry, 0, len(items))
	for k, v := range items {
		entries = append(entries, entry{key: k, ts: ageFn(v)})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ts.Before(entries[j].ts)
	})

	toRemove := len(items) - maxCap
	for i := 0; i < toRemove; i++ {
		delete(items, entries[i].key)
	}
	return toRemove
}
