package channel

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// DebounceConfig controls message debouncing for chatty channels.
type DebounceConfig struct {
	Window       time.Duration // Initial debounce window
	ActiveWindow time.Duration // Window during active typing
	MaxWait      time.Duration // Maximum time before forced flush
}

// DefaultDebounceConfig returns sensible defaults.
func DefaultDebounceConfig() DebounceConfig {
	return DebounceConfig{
		Window:       300 * time.Millisecond,
		ActiveWindow: 500 * time.Millisecond,
		MaxWait:      2 * time.Second,
	}
}

// Manager manages multiple channels and handles per-session debounce buffering.
type Manager struct {
	mu       sync.RWMutex
	channels map[string]Channel
	debounce DebounceConfig
	buffers  map[string]*debounceBuffer // keyed by channel:sessionID
}

// debounceBuffer accumulates messages for a session on a debounced channel.
type debounceBuffer struct {
	mu       sync.Mutex
	messages []Outbound
	timer    *time.Timer
	lastSend time.Time
	firstMsg time.Time
}

// NewManager creates a Manager with the given debounce configuration.
func NewManager(cfg DebounceConfig) *Manager {
	return &Manager{
		channels: make(map[string]Channel),
		debounce: cfg,
		buffers:  make(map[string]*debounceBuffer),
	}
}

// Register adds a channel to the manager.
func (m *Manager) Register(ch Channel) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.channels[ch.Name()] = ch
}

// Unregister removes a channel.
func (m *Manager) Unregister(name string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, name)
}

// Send delivers a message to the named channel, applying debounce if needed.
func (m *Manager) Send(ctx context.Context, channelName, sessionID string, msg Outbound) error {
	m.mu.RLock()
	ch, ok := m.channels[channelName]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("channel %q not registered", channelName)
	}

	if !ch.NeedsDebounce() {
		return ch.Send(ctx, sessionID, msg)
	}

	key := channelName + ":" + sessionID
	now := time.Now()

	m.mu.Lock()
	buf, exists := m.buffers[key]
	if !exists {
		buf = &debounceBuffer{}
		m.buffers[key] = buf
	}
	m.mu.Unlock()

	buf.mu.Lock()
	defer buf.mu.Unlock()

	if len(buf.messages) == 0 {
		buf.firstMsg = now
	}
	buf.messages = append(buf.messages, msg)

	// If MaxWait since the first buffered message is exceeded, flush immediately.
	if now.Sub(buf.firstMsg) >= m.debounce.MaxWait {
		return m.flushLocked(ctx, ch, sessionID, key, buf)
	}

	// Determine debounce window: use ActiveWindow if we sent recently, otherwise Window.
	window := m.debounce.Window
	if !buf.lastSend.IsZero() && now.Sub(buf.lastSend) < m.debounce.ActiveWindow {
		window = m.debounce.ActiveWindow
	}

	// Cap the window so we never exceed MaxWait from the first message.
	remaining := m.debounce.MaxWait - now.Sub(buf.firstMsg)
	if window > remaining {
		window = remaining
	}

	// Reset or start the debounce timer.
	if buf.timer != nil {
		buf.timer.Stop()
	}
	buf.timer = time.AfterFunc(window, func() {
		buf.mu.Lock()
		defer buf.mu.Unlock()
		// Use a background context since the original may have expired.
		_ = m.flushLocked(context.Background(), ch, sessionID, key, buf)
	})

	return nil
}

// flushLocked sends all buffered messages as a single combined Outbound.
// Caller must hold buf.mu.
func (m *Manager) flushLocked(ctx context.Context, ch Channel, sessionID, key string, buf *debounceBuffer) error {
	if len(buf.messages) == 0 {
		return nil
	}

	combined := mergeOutbound(buf.messages)

	if buf.timer != nil {
		buf.timer.Stop()
		buf.timer = nil
	}
	buf.messages = nil
	buf.lastSend = time.Now()
	buf.firstMsg = time.Time{}

	// Remove the buffer entry to prevent memory leaks from accumulating
	// empty buffers for sessions that are no longer active.
	m.mu.Lock()
	delete(m.buffers, key)
	m.mu.Unlock()

	return ch.Send(ctx, sessionID, combined)
}

// mergeOutbound combines multiple Outbound messages into one.
func mergeOutbound(msgs []Outbound) Outbound {
	if len(msgs) == 1 {
		return msgs[0]
	}

	var parts []string
	var media []MediaItem
	merged := make(map[string]any)
	kind := msgs[0].Kind

	for _, msg := range msgs {
		if msg.Content != "" {
			parts = append(parts, msg.Content)
		}
		media = append(media, msg.Media...)
		for k, v := range msg.Metadata {
			merged[k] = v
		}
		// If kinds differ, fall back to "text".
		if msg.Kind != kind {
			kind = "text"
		}
	}

	out := Outbound{
		Content: strings.Join(parts, "\n"),
		Kind:    kind,
	}
	if len(media) > 0 {
		out.Media = media
	}
	if len(merged) > 0 {
		out.Metadata = merged
	}
	return out
}

// Get returns a channel by name.
func (m *Manager) Get(name string) (Channel, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch, ok := m.channels[name]
	return ch, ok
}

// Channels returns all registered channel names.
func (m *Manager) Channels() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, 0, len(m.channels))
	for name := range m.channels {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Start starts all registered channels.
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ch := range m.channels {
		if err := ch.Start(ctx); err != nil {
			return fmt.Errorf("starting channel %q: %w", ch.Name(), err)
		}
	}
	return nil
}

// Stop stops all registered channels and flushes pending buffers.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Flush all pending debounce buffers.
	for key, buf := range m.buffers {
		buf.mu.Lock()
		parts := strings.SplitN(key, ":", 2)
		channelName, sessionID := parts[0], parts[1]
		if ch, ok := m.channels[channelName]; ok {
			_ = m.flushLocked(ctx, ch, sessionID, key, buf)
		}
		buf.mu.Unlock()
	}
	m.buffers = make(map[string]*debounceBuffer)

	// Stop all channels.
	var firstErr error
	for _, ch := range m.channels {
		if err := ch.Stop(ctx); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("stopping channel %q: %w", ch.Name(), err)
		}
	}
	return firstErr
}
