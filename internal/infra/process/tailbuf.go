package process

import "sync"

const defaultStderrTail = 8 * 1024

// tailBuffer is a thread-safe circular buffer that retains the last N bytes
// written to it. It implements io.Writer.
type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

// newTailBuffer creates a tailBuffer capped at max bytes.
func newTailBuffer(max int) *tailBuffer {
	if max <= 0 {
		max = defaultStderrTail
	}
	return &tailBuffer{max: max}
}

func (t *tailBuffer) Write(p []byte) (int, error) {
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
func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.buf) == 0 {
		return ""
	}
	cp := make([]byte, len(t.buf))
	copy(cp, t.buf)
	return string(cp)
}
