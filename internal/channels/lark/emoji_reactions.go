package lark

import (
	"math/rand"
	"strings"
	"sync"
	"time"
)

var defaultEmojiPool = []string{
	"WAVE",
	"SMILE",
	"THINKING",
	"MUSCLE",
	"HEART",
	"THUMBSUP",
	"OK",
	"THANKS",
	"APPLAUSE",
	"LGTM",
	"DONE",
	"Coffee",
	"Fire",
	"JIAYI",
}

func parseEmojiPool(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', ';', '|', ' ', '\n', '\t', '\r':
			return true
		default:
			return false
		}
	})
	if len(parts) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(parts))
	pool := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		pool = append(pool, trimmed)
	}
	return pool
}

func resolveEmojiPool(raw string) []string {
	pool := parseEmojiPool(raw)
	if len(pool) < 2 {
		return append([]string(nil), defaultEmojiPool...)
	}
	return append([]string(nil), pool...)
}

type emojiPicker struct {
	mu   sync.Mutex
	rand *rand.Rand
	pool []string
}

func newEmojiPicker(seed int64, pool []string) *emojiPicker {
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	if len(pool) == 0 {
		pool = defaultEmojiPool
	}
	return &emojiPicker{
		rand: rand.New(rand.NewSource(seed)),
		pool: append([]string(nil), pool...),
	}
}

func (p *emojiPicker) pickStartEnd() (string, string) {
	if p == nil {
		return "", ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.pool) == 0 {
		return "", ""
	}
	if len(p.pool) == 1 {
		return p.pool[0], p.pool[0]
	}
	startIdx := p.rand.Intn(len(p.pool))
	endIdx := p.rand.Intn(len(p.pool) - 1)
	if endIdx >= startIdx {
		endIdx++
	}
	return p.pool[startIdx], p.pool[endIdx]
}

func (g *Gateway) pickReactionEmojis() (string, string) {
	if g == nil {
		return "", ""
	}
	picker := g.emojiPicker
	if picker == nil {
		picker = newEmojiPicker(time.Now().UnixNano(), resolveEmojiPool(g.cfg.ReactEmoji))
	}
	return picker.pickStartEnd()
}
