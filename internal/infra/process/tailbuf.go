package process

import "sync"

// DefaultStderrTail is the default tail buffer size (8 KB).
const DefaultStderrTail = 8 * 1024

// TailBuffer is a thread-safe circular buffer that retains the last N bytes
// written to it. It implements io.Writer.
type TailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

// NewTailBuffer creates a TailBuffer capped at max bytes.
// If max <= 0, DefaultStderrTail is used.
func NewTailBuffer(max int) *TailBuffer {
	if max <= 0 {
		max = DefaultStderrTail
	}
	return &TailBuffer{max: max}
}

func (t *TailBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(p) >= t.max {
		t.buf = append(t.buf[:0], p[len(p)-t.max:]...)
		return len(p), nil
	}

	if len(t.buf)+len(p) > t.max {
		excess := len(t.buf) + len(p) - t.max
		t.buf = t.buf[excess:]
	}
	t.buf = append(t.buf, p...)
	return len(p), nil
}

// String returns a copy of the buffered content.
func (t *TailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.buf) == 0 {
		return ""
	}
	cp := make([]byte, len(t.buf))
	copy(cp, t.buf)
	return string(cp)
}
