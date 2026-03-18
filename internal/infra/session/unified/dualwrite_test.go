package unified

import (
	"context"
	"errors"
	"testing"

	storage "alex/internal/domain/agent/ports/storage"
	core "alex/internal/domain/agent/ports"
)

// failStore fails all reads but succeeds on writes.
type failStore struct{ mockStore }

func newFailStore() *failStore {
	return &failStore{mockStore{sessions: make(map[string]*storage.Session)}}
}

func (f *failStore) Get(_ context.Context, _ string) (*storage.Session, error) {
	return nil, errors.New("primary unavailable")
}

func TestDualWriteStore(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"CreateWritesBoth", testDWCreateBoth},
		{"GetReadsFromPrimary", testDWGetPrimary},
		{"GetFallsBackToSecondary", testDWGetFallback},
		{"SaveWritesBoth", testDWSaveBoth},
		{"SecondaryFailureNonFatal", testDWSecondaryFailNonFatal},
		{"DeleteBoth", testDWDeleteBoth},
		{"ValidateParityDetectsDiscrepancy", testDWParity},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func testDWCreateBoth(t *testing.T) {
	primary, secondary := newMockStore(), newMockStore()
	dw := NewDualWrite(primary, secondary, nil)
	ctx := context.Background()

	sess, err := dw.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, ok := primary.sessions[sess.ID]; !ok {
		t.Fatal("session missing from primary")
	}
	if _, ok := secondary.sessions[sess.ID]; !ok {
		t.Fatal("session missing from secondary")
	}
}

func testDWGetPrimary(t *testing.T) {
	primary, secondary := newMockStore(), newMockStore()
	dw := NewDualWrite(primary, secondary, nil)
	ctx := context.Background()

	sess, _ := dw.Create(ctx)
	got, err := dw.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ID != sess.ID {
		t.Fatalf("got %q, want %q", got.ID, sess.ID)
	}
}

func testDWGetFallback(t *testing.T) {
	primary := newFailStore()
	secondary := newMockStore()
	dw := NewDualWrite(primary, secondary, nil)
	ctx := context.Background()

	sess := storage.NewSession("fallback-1", fixedNow())
	_ = secondary.Save(ctx, sess)

	got, err := dw.Get(ctx, "fallback-1")
	if err != nil {
		t.Fatalf("fallback Get: %v", err)
	}
	if got.ID != "fallback-1" {
		t.Fatalf("got %q, want fallback-1", got.ID)
	}
}

func testDWSaveBoth(t *testing.T) {
	primary, secondary := newMockStore(), newMockStore()
	dw := NewDualWrite(primary, secondary, nil)
	ctx := context.Background()

	sess := storage.NewSession("save-both", fixedNow())
	if err := dw.Save(ctx, sess); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, ok := primary.sessions["save-both"]; !ok {
		t.Fatal("missing from primary")
	}
	if _, ok := secondary.sessions["save-both"]; !ok {
		t.Fatal("missing from secondary")
	}
}

func testDWSecondaryFailNonFatal(t *testing.T) {
	primary := newMockStore()
	secondary := &writeFailStore{mockStore: mockStore{sessions: make(map[string]*storage.Session)}}
	dw := NewDualWrite(primary, secondary, nil)
	ctx := context.Background()

	sess := storage.NewSession("fail-sec", fixedNow())
	err := dw.Save(ctx, sess)
	if err != nil {
		t.Fatalf("Save should succeed despite secondary failure: %v", err)
	}
}

func testDWDeleteBoth(t *testing.T) {
	primary, secondary := newMockStore(), newMockStore()
	dw := NewDualWrite(primary, secondary, nil)
	ctx := context.Background()

	sess, _ := dw.Create(ctx)
	if err := dw.Delete(ctx, sess.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := primary.sessions[sess.ID]; ok {
		t.Fatal("still in primary")
	}
	if _, ok := secondary.sessions[sess.ID]; ok {
		t.Fatal("still in secondary")
	}
}

func testDWParity(t *testing.T) {
	primary, secondary := newMockStore(), newMockStore()
	dw := NewDualWrite(primary, secondary, nil)
	ctx := context.Background()

	sess, _ := dw.Create(ctx)

	// Replace primary's copy with extra messages to create a discrepancy
	pCopy := *primary.sessions[sess.ID]
	pCopy.Messages = []core.Message{{Role: "user", Content: "hello"}}
	primary.sessions[sess.ID] = &pCopy

	issues, err := dw.ValidateParity(ctx)
	if err != nil {
		t.Fatalf("ValidateParity: %v", err)
	}
	if len(issues) == 0 {
		t.Fatal("expected parity issues")
	}

	found := false
	for _, issue := range issues {
		if issue.Field == "messages_count" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected messages_count discrepancy")
	}
}

// writeFailStore fails all Save calls.
type writeFailStore struct{ mockStore }

func (w *writeFailStore) Save(_ context.Context, _ *storage.Session) error {
	return errors.New("secondary write failure")
}

func TestDualWrite_ListDelegatesToPrimary(t *testing.T) {
	primary, secondary := newMockStore(), newMockStore()
	dw := NewDualWrite(primary, secondary, nil)
	ctx := context.Background()

	_, _ = dw.Create(ctx)
	ids, err := dw.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 1 {
		t.Fatalf("expected 1, got %d", len(ids))
	}
}
