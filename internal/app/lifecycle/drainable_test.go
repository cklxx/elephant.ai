package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- DrainFunc tests ---

func TestDrainFunc_Name(t *testing.T) {
	df := DrainFunc{DrainName: "test-subsystem"}
	if got := df.Name(); got != "test-subsystem" {
		t.Errorf("Name() = %q, want test-subsystem", got)
	}
}

func TestDrainFunc_Drain(t *testing.T) {
	var called atomic.Bool
	df := DrainFunc{
		DrainName: "fn",
		Fn:        func(ctx context.Context) { called.Store(true) },
	}
	err := df.Drain(context.Background())
	if err != nil {
		t.Errorf("Drain() error = %v, want nil", err)
	}
	if !called.Load() {
		t.Error("expected Fn to be called")
	}
}

func TestDrainFunc_DrainAlwaysReturnsNil(t *testing.T) {
	df := DrainFunc{
		DrainName: "never-errors",
		Fn:        func(ctx context.Context) {},
	}
	if err := df.Drain(context.Background()); err != nil {
		t.Errorf("DrainFunc.Drain should always return nil, got %v", err)
	}
}

func TestDrainFunc_ReceivesContext(t *testing.T) {
	type ctxKey struct{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "hello")

	var received string
	df := DrainFunc{
		DrainName: "ctx-check",
		Fn: func(ctx context.Context) {
			if v, ok := ctx.Value(ctxKey{}).(string); ok {
				received = v
			}
		},
	}
	_ = df.Drain(ctx)
	if received != "hello" {
		t.Errorf("context value = %q, want hello", received)
	}
}

// --- DrainAll tests ---

type mockDrainable struct {
	name     string
	err      error
	duration time.Duration
	called   atomic.Bool
}

func (m *mockDrainable) Drain(ctx context.Context) error {
	m.called.Store(true)
	if m.duration > 0 {
		select {
		case <-time.After(m.duration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.err
}

func (m *mockDrainable) Name() string { return m.name }

func TestDrainAll_Empty(t *testing.T) {
	errs := DrainAll(context.Background(), time.Second)
	if len(errs) != 0 {
		t.Errorf("expected no errors for empty subsystems, got %v", errs)
	}
}

func TestDrainAll_AllSucceed(t *testing.T) {
	subs := []Drainable{
		&mockDrainable{name: "a"},
		&mockDrainable{name: "b"},
		&mockDrainable{name: "c"},
	}
	errs := DrainAll(context.Background(), time.Second, subs...)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
	for _, s := range subs {
		if !s.(*mockDrainable).called.Load() {
			t.Errorf("subsystem %q was not drained", s.Name())
		}
	}
}

func TestDrainAll_SomeFail(t *testing.T) {
	subs := []Drainable{
		&mockDrainable{name: "ok"},
		&mockDrainable{name: "fail1", err: errors.New("boom1")},
		&mockDrainable{name: "fail2", err: errors.New("boom2")},
	}
	errs := DrainAll(context.Background(), time.Second, subs...)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d: %v", len(errs), errs)
	}
	// Errors should be prefixed with subsystem name.
	if !strings.Contains(errs[0].Error(), "fail1") {
		t.Errorf("error[0] = %q, want to contain 'fail1'", errs[0])
	}
	if !strings.Contains(errs[1].Error(), "fail2") {
		t.Errorf("error[1] = %q, want to contain 'fail2'", errs[1])
	}
}

func TestDrainAll_AllFail(t *testing.T) {
	subs := []Drainable{
		&mockDrainable{name: "a", err: errors.New("err-a")},
		&mockDrainable{name: "b", err: errors.New("err-b")},
	}
	errs := DrainAll(context.Background(), time.Second, subs...)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}
}

func TestDrainAll_ErrorWrapping(t *testing.T) {
	sentinel := fmt.Errorf("sentinel error")
	subs := []Drainable{
		&mockDrainable{name: "wrap-test", err: sentinel},
	}
	errs := DrainAll(context.Background(), time.Second, subs...)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if !errors.Is(errs[0], sentinel) {
		t.Errorf("error should wrap sentinel: %v", errs[0])
	}
}

func TestDrainAll_SequentialExecution(t *testing.T) {
	var order []string
	subs := []Drainable{
		DrainFunc{DrainName: "first", Fn: func(ctx context.Context) { order = append(order, "first") }},
		DrainFunc{DrainName: "second", Fn: func(ctx context.Context) { order = append(order, "second") }},
		DrainFunc{DrainName: "third", Fn: func(ctx context.Context) { order = append(order, "third") }},
	}
	_ = DrainAll(context.Background(), time.Second, subs...)
	if len(order) != 3 || order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("expected sequential order [first second third], got %v", order)
	}
}

func TestDrainAll_TimeoutPerSubsystem(t *testing.T) {
	slow := &mockDrainable{name: "slow", duration: 5 * time.Second}
	fast := &mockDrainable{name: "fast"}

	errs := DrainAll(context.Background(), 100*time.Millisecond, slow, fast)

	// slow should timeout, fast should succeed.
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (slow timeout), got %d: %v", len(errs), errs)
	}
	if !strings.Contains(errs[0].Error(), "slow") {
		t.Errorf("error should mention 'slow': %v", errs[0])
	}
	// fast should still have been called.
	if !fast.called.Load() {
		t.Error("fast subsystem should have been called despite slow timeout")
	}
}

func TestDrainAll_CancelledParentContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	slow := &mockDrainable{name: "slow", duration: time.Second}
	errs := DrainAll(ctx, time.Second, slow)

	// With parent context cancelled, the sub-context should also be cancelled.
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d: %v", len(errs), errs)
	}
}

// --- Interface compliance ---

func TestDrainFunc_ImplementsDrainable(t *testing.T) {
	var _ Drainable = DrainFunc{}
}
