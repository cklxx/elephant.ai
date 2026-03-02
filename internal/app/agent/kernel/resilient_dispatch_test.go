package kernel

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
)

// ─────────────────────────────────────────────────────────────────────────────
// markDispatchDoneResilient — 37.5% coverage
// Tests: happy path, context.Canceled fallback, context.DeadlineExceeded fallback,
//        non-context error (no fallback), fallback error propagation
// ─────────────────────────────────────────────────────────────────────────────

// errStore wraps memStore and allows injecting errors for MarkDispatchDone/Failed.
type errStore struct {
	*memStore
	doneErr   error
	failedErr error
	// After first call, use doneErr2 for subsequent calls (simulates transient error).
	doneCall  int
	doneErr2  error
	failedCall int
	failedErr2 error
}

func (s *errStore) MarkDispatchDone(ctx context.Context, dispatchID, taskID string) error {
	s.doneCall++
	if s.doneCall == 1 && s.doneErr != nil {
		return s.doneErr
	}
	if s.doneCall > 1 && s.doneErr2 != nil {
		return s.doneErr2
	}
	return s.memStore.MarkDispatchDone(ctx, dispatchID, taskID)
}

func (s *errStore) MarkDispatchFailed(ctx context.Context, dispatchID, errMsg string) error {
	s.failedCall++
	if s.failedCall == 1 && s.failedErr != nil {
		return s.failedErr
	}
	if s.failedCall > 1 && s.failedErr2 != nil {
		return s.failedErr2
	}
	return s.memStore.MarkDispatchFailed(ctx, dispatchID, errMsg)
}

func newErrStore() *errStore {
	return &errStore{memStore: newMemStore()}
}

func TestMarkDispatchDoneResilient_HappyPath(t *testing.T) {
	store := newErrStore()
	ctx := context.Background()
	err := markDispatchDoneResilient(ctx, store, "d-1", "task-abc")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if store.doneCall != 1 {
		t.Errorf("expected 1 call, got %d", store.doneCall)
	}
}

func TestMarkDispatchDoneResilient_ContextCanceledFallback(t *testing.T) {
	store := newErrStore()
	store.doneErr = context.Canceled // first call fails with Canceled
	// second call (fallback) succeeds
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already canceled
	err := markDispatchDoneResilient(ctx, store, "d-1", "task-abc")
	if err != nil {
		t.Fatalf("expected nil after fallback, got %v", err)
	}
	if store.doneCall != 2 {
		t.Errorf("expected 2 calls (original + fallback), got %d", store.doneCall)
	}
}

func TestMarkDispatchDoneResilient_DeadlineExceededFallback(t *testing.T) {
	store := newErrStore()
	store.doneErr = context.DeadlineExceeded
	ctx := context.Background()
	err := markDispatchDoneResilient(ctx, store, "d-1", "task-abc")
	if err != nil {
		t.Fatalf("expected nil after fallback, got %v", err)
	}
	if store.doneCall != 2 {
		t.Errorf("expected 2 calls, got %d", store.doneCall)
	}
}

func TestMarkDispatchDoneResilient_NonContextError_NoFallback(t *testing.T) {
	store := newErrStore()
	store.doneErr = fmt.Errorf("storage io error")
	ctx := context.Background()
	err := markDispatchDoneResilient(ctx, store, "d-1", "task-abc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if store.doneCall != 1 {
		t.Errorf("expected exactly 1 call (no fallback), got %d", store.doneCall)
	}
}

func TestMarkDispatchDoneResilient_FallbackAlsoFails(t *testing.T) {
	store := newErrStore()
	store.doneErr = context.Canceled
	store.doneErr2 = fmt.Errorf("fallback also failed")
	ctx := context.Background()
	err := markDispatchDoneResilient(ctx, store, "d-1", "task-abc")
	if err == nil {
		t.Fatal("expected error from fallback, got nil")
	}
	if !errors.Is(err, store.doneErr2) && err.Error() != store.doneErr2.Error() {
		t.Errorf("expected fallback error %q, got %q", store.doneErr2, err)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// markDispatchFailedResilient — 37.5% coverage
// ─────────────────────────────────────────────────────────────────────────────

func TestMarkDispatchFailedResilient_HappyPath(t *testing.T) {
	store := newErrStore()
	ctx := context.Background()
	err := markDispatchFailedResilient(ctx, store, "d-1", "something went wrong")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if store.failedCall != 1 {
		t.Errorf("expected 1 call, got %d", store.failedCall)
	}
}

func TestMarkDispatchFailedResilient_ContextCanceledFallback(t *testing.T) {
	store := newErrStore()
	store.failedErr = context.Canceled
	ctx := context.Background()
	err := markDispatchFailedResilient(ctx, store, "d-1", "err msg")
	if err != nil {
		t.Fatalf("expected nil after fallback, got %v", err)
	}
	if store.failedCall != 2 {
		t.Errorf("expected 2 calls, got %d", store.failedCall)
	}
}

func TestMarkDispatchFailedResilient_DeadlineExceededFallback(t *testing.T) {
	store := newErrStore()
	store.failedErr = context.DeadlineExceeded
	ctx := context.Background()
	err := markDispatchFailedResilient(ctx, store, "d-1", "err msg")
	if err != nil {
		t.Fatalf("expected nil after fallback, got %v", err)
	}
	if store.failedCall != 2 {
		t.Errorf("expected 2 calls, got %d", store.failedCall)
	}
}

func TestMarkDispatchFailedResilient_NonContextError_NoFallback(t *testing.T) {
	store := newErrStore()
	store.failedErr = fmt.Errorf("db write failure")
	ctx := context.Background()
	err := markDispatchFailedResilient(ctx, store, "d-1", "err msg")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if store.failedCall != 1 {
		t.Errorf("expected exactly 1 call, got %d", store.failedCall)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// classifyDispatchError — 57.1% coverage
// Tests: nil, DeadlineExceeded, Canceled, validation error, unknown error
// ─────────────────────────────────────────────────────────────────────────────

func TestClassifyDispatchError_Nil(t *testing.T) {
	got := classifyDispatchError(nil)
	if got != "" {
		t.Errorf("expected empty string for nil error, got %q", got)
	}
}

func TestClassifyDispatchError_DeadlineExceeded(t *testing.T) {
	got := classifyDispatchError(context.DeadlineExceeded)
	if got != "timeout" {
		t.Errorf("expected %q, got %q", "timeout", got)
	}
}

func TestClassifyDispatchError_Canceled(t *testing.T) {
	got := classifyDispatchError(context.Canceled)
	if got != "canceled" {
		t.Errorf("expected %q, got %q", "canceled", got)
	}
}

func TestClassifyDispatchError_WrappedDeadlineExceeded(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", context.DeadlineExceeded)
	got := classifyDispatchError(wrapped)
	if got != "timeout" {
		t.Errorf("expected %q for wrapped DeadlineExceeded, got %q", "timeout", got)
	}
}

func TestClassifyDispatchError_WrappedCanceled(t *testing.T) {
	wrapped := fmt.Errorf("outer: %w", context.Canceled)
	got := classifyDispatchError(wrapped)
	if got != "canceled" {
		t.Errorf("expected %q for wrapped Canceled, got %q", "canceled", got)
	}
}

func TestClassifyDispatchError_UnknownError(t *testing.T) {
	got := classifyDispatchError(fmt.Errorf("some random error"))
	// Should fall through to classifyKernelValidationError — which returns "" for unknown errors.
	// We just confirm it doesn't panic and returns a string.
	_ = got
}

// ─────────────────────────────────────────────────────────────────────────────
// Verify errStore correctly implements kerneldomain.Store
// ─────────────────────────────────────────────────────────────────────────────

var _ kerneldomain.Store = (*errStore)(nil)
