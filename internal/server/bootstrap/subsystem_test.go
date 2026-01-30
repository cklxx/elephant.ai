package bootstrap

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"alex/internal/logging"
)

type fakeSubsystem struct {
	name     string
	startErr error

	mu       sync.Mutex
	started  bool
	stopped  bool
	startCtx context.Context
}

func (f *fakeSubsystem) Name() string { return f.name }

func (f *fakeSubsystem) Start(ctx context.Context) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.mu.Lock()
	f.started = true
	f.startCtx = ctx
	f.mu.Unlock()
	return nil
}

func (f *fakeSubsystem) Stop() {
	f.mu.Lock()
	f.stopped = true
	f.mu.Unlock()
}

func (f *fakeSubsystem) isStarted() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.started
}

func (f *fakeSubsystem) isStopped() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.stopped
}

func TestSubsystemManagerStartStopOrder(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	mgr := NewSubsystemManager(logger)

	var order []string
	var mu sync.Mutex

	makeSubsystem := func(name string) *orderTrackingSubsystem {
		return &orderTrackingSubsystem{
			name: name,
			onStop: func() {
				mu.Lock()
				order = append(order, name)
				mu.Unlock()
			},
		}
	}

	a := makeSubsystem("a")
	b := makeSubsystem("b")
	c := makeSubsystem("c")

	ctx := context.Background()
	for _, sub := range []Subsystem{a, b, c} {
		if err := mgr.Start(ctx, sub); err != nil {
			t.Fatalf("Start(%s) failed: %v", sub.Name(), err)
		}
	}

	mgr.StopAll()

	// Verify LIFO order
	if len(order) != 3 {
		t.Fatalf("expected 3 stops, got %d", len(order))
	}
	if order[0] != "c" || order[1] != "b" || order[2] != "a" {
		t.Fatalf("expected LIFO order [c, b, a], got %v", order)
	}
}

type orderTrackingSubsystem struct {
	name   string
	onStop func()
}

func (o *orderTrackingSubsystem) Name() string                    { return o.name }
func (o *orderTrackingSubsystem) Start(_ context.Context) error   { return nil }
func (o *orderTrackingSubsystem) Stop()                           { o.onStop() }

func TestSubsystemManagerCancelsContextOnStop(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	mgr := NewSubsystemManager(logger)

	sub := &fakeSubsystem{name: "ctx-check"}
	if err := mgr.Start(context.Background(), sub); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Context should still be valid
	if sub.startCtx.Err() != nil {
		t.Fatal("context should not be cancelled before StopAll")
	}

	mgr.StopAll()

	// After StopAll, derived context should be cancelled
	if sub.startCtx.Err() == nil {
		t.Fatal("context should be cancelled after StopAll")
	}
	if !sub.isStopped() {
		t.Fatal("subsystem should be stopped")
	}
}

func TestSubsystemManagerStartFailureNotTracked(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	mgr := NewSubsystemManager(logger)

	good := &fakeSubsystem{name: "good"}
	bad := &fakeSubsystem{name: "bad", startErr: fmt.Errorf("init failed")}

	ctx := context.Background()
	if err := mgr.Start(ctx, good); err != nil {
		t.Fatalf("Start(good) failed: %v", err)
	}
	if err := mgr.Start(ctx, bad); err == nil {
		t.Fatal("expected error from Start(bad)")
	}

	mgr.StopAll()

	// Only good should be stopped; bad was never tracked
	if !good.isStopped() {
		t.Fatal("good should be stopped")
	}
	if bad.isStopped() {
		t.Fatal("bad should not be stopped (was never tracked)")
	}
}

func TestSubsystemManagerStopAllIdempotent(t *testing.T) {
	logger := logging.NewComponentLogger("test")
	mgr := NewSubsystemManager(logger)

	sub := &fakeSubsystem{name: "once"}
	if err := mgr.Start(context.Background(), sub); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	mgr.StopAll()
	mgr.StopAll() // should not panic or double-stop

	if !sub.isStopped() {
		t.Fatal("subsystem should be stopped")
	}
}
