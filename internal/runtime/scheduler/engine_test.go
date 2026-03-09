package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/session"
)

// fakeRuntime implements RuntimeManager for testing.
type fakeRuntime struct {
	mu       sync.Mutex
	sessions map[string]*session.Session
	started  []string
}

func newFakeRuntime() *fakeRuntime {
	return &fakeRuntime{sessions: make(map[string]*session.Session)}
}

func (f *fakeRuntime) CreateSession(member session.MemberType, goal, workDir string) (*session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := session.New("s-"+goal, member, goal, workDir)
	f.sessions[s.ID] = s
	return s, nil
}

func (f *fakeRuntime) StartSession(_ context.Context, id string, _ int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, id)
	return nil
}

func (f *fakeRuntime) getStarted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.started))
	copy(out, f.started)
	return out
}

func TestEngine_NoDependencies(t *testing.T) {
	bus := hooks.NewInProcessBus()
	rt := newFakeRuntime()
	engine := NewEngine(rt, bus, -1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := engine.Schedule(ctx, []SessionSpec{
		{Member: session.MemberShell, Goal: "task1"},
		{Member: session.MemberShell, Goal: "task2"},
	})
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}

	// Run engine in background, then complete all sessions.
	done := make(chan error, 1)
	go func() { done <- engine.Run(ctx) }()

	// Give engine a moment to start sessions.
	time.Sleep(100 * time.Millisecond)

	// Mark both as completed.
	for _, sp := range engine.specs {
		bus.Publish(sp.id, hooks.Event{Type: hooks.EventCompleted, SessionID: sp.id, At: time.Now()})
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("engine.Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("engine did not finish")
	}

	if len(rt.getStarted()) != 2 {
		t.Fatalf("expected 2 started, got %d", len(rt.getStarted()))
	}
}

func TestEngine_SerialDependency(t *testing.T) {
	bus := hooks.NewInProcessBus()
	rt := newFakeRuntime()
	engine := NewEngine(rt, bus, -1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ids, err := engine.Schedule(ctx, []SessionSpec{
		{Member: session.MemberShell, Goal: "first"},
	})
	if err != nil {
		t.Fatal(err)
	}
	firstID := ids[0]

	_, err = engine.Schedule(ctx, []SessionSpec{
		{Member: session.MemberShell, Goal: "second", DependsOn: []string{firstID}},
	})
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() { done <- engine.Run(ctx) }()

	// Give engine time to start the first session (no deps).
	time.Sleep(150 * time.Millisecond)

	// Verify only first started.
	started := rt.getStarted()
	if len(started) != 1 || started[0] != firstID {
		t.Fatalf("expected only first started, got %v", started)
	}

	// Complete first session.
	bus.Publish(firstID, hooks.Event{Type: hooks.EventCompleted, SessionID: firstID, At: time.Now()})

	// Give engine time to start second.
	time.Sleep(150 * time.Millisecond)

	// Complete second session.
	for _, sp := range engine.specs {
		if sp.id != firstID {
			bus.Publish(sp.id, hooks.Event{Type: hooks.EventCompleted, SessionID: sp.id, At: time.Now()})
		}
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("engine.Run: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("engine did not finish")
	}

	if len(rt.getStarted()) != 2 {
		t.Fatalf("expected 2 started, got %d", len(rt.getStarted()))
	}
}
