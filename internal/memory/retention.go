package memory

import (
	"strings"
	"time"
)

// RetentionPolicy defines how long memories should be retained.
type RetentionPolicy struct {
	DefaultTTL    time.Duration
	TypeTTL       map[string]time.Duration
	PruneOnRecall bool
}

func (p RetentionPolicy) HasRules() bool {
	return p.DefaultTTL > 0 || len(p.TypeTTL) > 0
}

func (p RetentionPolicy) ttlForEntry(entry Entry) time.Duration {
	entryType := entryType(entry)
	if entryType == "" {
		entryType = "manual"
	}
	if p.TypeTTL != nil {
		if ttl, ok := p.TypeTTL[entryType]; ok {
			return ttl
		}
	}
	return p.DefaultTTL
}

func (p RetentionPolicy) IsExpired(entry Entry, now time.Time) bool {
	ttl := p.ttlForEntry(entry)
	if ttl <= 0 {
		return false
	}
	if entry.CreatedAt.IsZero() {
		return false
	}
	return now.Sub(entry.CreatedAt) > ttl
}

func entryType(entry Entry) string {
	if entry.Slots == nil {
		return ""
	}
	value := strings.TrimSpace(entry.Slots["type"])
	return strings.ToLower(value)
}
