package channel

import (
	"context"
	"sync"
	"testing"
)

type mockChannel struct {
	name     string
	debounce bool
	sent     []Outbound
	started  bool
	stopped  bool
	mu       sync.Mutex
}

func (m *mockChannel) Name() string        { return m.name }
func (m *mockChannel) NeedsDebounce() bool  { return m.debounce }

func (m *mockChannel) Start(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
	return nil
}

func (m *mockChannel) Stop(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	return nil
}

func (m *mockChannel) Send(_ context.Context, _ string, msg Outbound) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sent = append(m.sent, msg)
	return nil
}

func (m *mockChannel) getSent() []Outbound {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]Outbound, len(m.sent))
	copy(cp, m.sent)
	return cp
}

func TestNewManager(t *testing.T) {
	cfg := DefaultDebounceConfig()
	mgr := NewManager(cfg)

	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}
	if mgr.debounce.Window != cfg.Window {
		t.Errorf("debounce.Window: got %v, want %v", mgr.debounce.Window, cfg.Window)
	}
}

func TestManager_RegisterGetUnregister(t *testing.T) {
	mgr := NewManager(DefaultDebounceConfig())
	ch := &mockChannel{name: "test-ch"}

	// Register
	mgr.Register(ch)
	got, ok := mgr.Get("test-ch")
	if !ok {
		t.Fatal("Get after Register: not found")
	}
	if got.Name() != "test-ch" {
		t.Errorf("Name: got %q, want %q", got.Name(), "test-ch")
	}

	// Get missing
	_, ok = mgr.Get("nonexistent")
	if ok {
		t.Error("Get for nonexistent should return false")
	}

	// Unregister
	mgr.Unregister("test-ch")
	_, ok = mgr.Get("test-ch")
	if ok {
		t.Error("Get after Unregister should return false")
	}
}

func TestManager_Channels(t *testing.T) {
	mgr := NewManager(DefaultDebounceConfig())
	mgr.Register(&mockChannel{name: "bravo"})
	mgr.Register(&mockChannel{name: "alpha"})

	names := mgr.Channels()
	// Channels returns sorted names
	if len(names) != 2 {
		t.Fatalf("Channels length: got %d, want 2", len(names))
	}
	if names[0] != "alpha" || names[1] != "bravo" {
		t.Errorf("Channels: got %v, want [alpha bravo]", names)
	}
}

func TestManager_Send_NonDebounce(t *testing.T) {
	mgr := NewManager(DefaultDebounceConfig())
	ch := &mockChannel{name: "direct", debounce: false}
	mgr.Register(ch)

	ctx := context.Background()
	msg := Outbound{Content: "hello", Kind: "text"}

	err := mgr.Send(ctx, "direct", "sess1", msg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	sent := ch.getSent()
	if len(sent) != 1 {
		t.Fatalf("sent count: got %d, want 1", len(sent))
	}
	if sent[0].Content != "hello" {
		t.Errorf("sent content: got %q, want %q", sent[0].Content, "hello")
	}
}

func TestManager_Send_UnknownChannel(t *testing.T) {
	mgr := NewManager(DefaultDebounceConfig())
	err := mgr.Send(context.Background(), "nope", "sess1", Outbound{Content: "x"})
	if err == nil {
		t.Error("expected error for unknown channel")
	}
}

func TestManager_StartStop(t *testing.T) {
	mgr := NewManager(DefaultDebounceConfig())
	ch1 := &mockChannel{name: "a"}
	ch2 := &mockChannel{name: "b"}
	mgr.Register(ch1)
	mgr.Register(ch2)

	ctx := context.Background()

	// Start
	if err := mgr.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !ch1.started || !ch2.started {
		t.Error("Start did not call through to all channels")
	}

	// Stop
	if err := mgr.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !ch1.stopped || !ch2.stopped {
		t.Error("Stop did not call through to all channels")
	}
}

func TestManager_Channels_Empty(t *testing.T) {
	mgr := NewManager(DefaultDebounceConfig())
	names := mgr.Channels()
	if len(names) != 0 {
		t.Errorf("Channels on empty manager: got %d, want 0", len(names))
	}
}

func TestManager_Send_MultipleMessages_NonDebounce(t *testing.T) {
	mgr := NewManager(DefaultDebounceConfig())
	ch := &mockChannel{name: "cli", debounce: false}
	mgr.Register(ch)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if err := mgr.Send(ctx, "cli", "s1", Outbound{Content: "msg"}); err != nil {
			t.Fatalf("Send %d: %v", i, err)
		}
	}

	sent := ch.getSent()
	if len(sent) != 5 {
		t.Errorf("sent count: got %d, want 5", len(sent))
	}
}
