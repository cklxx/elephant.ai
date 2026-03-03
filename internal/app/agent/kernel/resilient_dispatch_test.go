package kernel

import (
	"context"
	"errors"
	"fmt"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
)

// ─────────────────────────────────────────────────────────────────────────────
// resilientStoreOp
// ─────────────────────────────────────────────────────────────────────────────

// errStore wraps memStore and allows injecting errors for MarkDispatchDone/Failed.
type errStore struct {
	*memStore
	doneErr    error
	failedErr  error
	doneCall   int
	doneErr2   error
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

func TestResilientStoreOp_HappyPath(t *testing.T) {
	calls := 0
	err := resilientStoreOp(context.Background(), func(_ context.Context) error {
		calls++
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestResilientStoreOp_ContextCanceledRetries(t *testing.T) {
	calls := 0
	err := resilientStoreOp(context.Background(), func(_ context.Context) error {
		calls++
		if calls == 1 {
			return context.Canceled
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil after retry, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestResilientStoreOp_DeadlineExceededRetries(t *testing.T) {
	calls := 0
	err := resilientStoreOp(context.Background(), func(_ context.Context) error {
		calls++
		if calls == 1 {
			return context.DeadlineExceeded
		}
		return nil
	})
	if err != nil {
		t.Fatalf("expected nil after retry, got %v", err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestResilientStoreOp_NonContextError_NoRetry(t *testing.T) {
	calls := 0
	ioErr := fmt.Errorf("storage io error")
	err := resilientStoreOp(context.Background(), func(_ context.Context) error {
		calls++
		return ioErr
	})
	if !errors.Is(err, ioErr) {
		t.Fatalf("expected %v, got %v", ioErr, err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry), got %d", calls)
	}
}

func TestResilientStoreOp_RetryAlsoFails(t *testing.T) {
	calls := 0
	retryErr := fmt.Errorf("fallback also failed")
	err := resilientStoreOp(context.Background(), func(_ context.Context) error {
		calls++
		if calls == 1 {
			return context.Canceled
		}
		return retryErr
	})
	if !errors.Is(err, retryErr) {
		t.Fatalf("expected retry error %q, got %q", retryErr, err)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Integration: resilientStoreOp with MarkDispatchDone/Failed via errStore
// ─────────────────────────────────────────────────────────────────────────────

func TestResilientStoreOp_MarkDone_Integration(t *testing.T) {
	store := newErrStore()
	store.doneErr = context.Canceled
	ctx := context.Background()
	err := resilientStoreOp(ctx, func(c context.Context) error {
		return store.MarkDispatchDone(c, "d-1", "task-abc")
	})
	if err != nil {
		t.Fatalf("expected nil after fallback, got %v", err)
	}
	if store.doneCall != 2 {
		t.Errorf("expected 2 calls (original + fallback), got %d", store.doneCall)
	}
}

func TestResilientStoreOp_MarkFailed_Integration(t *testing.T) {
	store := newErrStore()
	store.failedErr = context.DeadlineExceeded
	ctx := context.Background()
	err := resilientStoreOp(ctx, func(c context.Context) error {
		return store.MarkDispatchFailed(c, "d-1", "err msg")
	})
	if err != nil {
		t.Fatalf("expected nil after fallback, got %v", err)
	}
	if store.failedCall != 2 {
		t.Errorf("expected 2 calls, got %d", store.failedCall)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// classifyDispatchError
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
	_ = got // verify no panic
}

// ─────────────────────────────────────────────────────────────────────────────
// Verify errStore correctly implements kerneldomain.Store
// ─────────────────────────────────────────────────────────────────────────────

var _ kerneldomain.Store = (*errStore)(nil)
