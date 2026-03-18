package unified

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	storage "alex/internal/domain/agent/ports/storage"
)

// mockStore is a minimal in-memory SessionStore for testing.
type mockStore struct {
	sessions map[string]*storage.Session
	nextID   int
}

func newMockStore() *mockStore {
	return &mockStore{sessions: make(map[string]*storage.Session)}
}

func (m *mockStore) Create(_ context.Context) (*storage.Session, error) {
	m.nextID++
	s := storage.NewSession("mock-"+string(rune('0'+m.nextID)), fixedNow())
	m.sessions[s.ID] = s
	return s, nil
}

func (m *mockStore) Get(_ context.Context, id string) (*storage.Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, storage.ErrSessionNotFound
	}
	return s, nil
}

func (m *mockStore) Save(_ context.Context, s *storage.Session) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockStore) List(_ context.Context, limit int, _ int) ([]string, error) {
	var ids []string
	for id := range m.sessions {
		ids = append(ids, id)
		if limit > 0 && len(ids) >= limit {
			break
		}
	}
	return ids, nil
}

func (m *mockStore) Delete(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func newTestStore(t *testing.T) (*Store, *mockStore) {
	t.Helper()
	inner := newMockStore()
	indexPath := filepath.Join(t.TempDir(), "index.json")
	s, err := New(inner, indexPath, fixedNow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, inner
}

func TestStore(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{"CreateDelegatesToInner", testCreateDelegates},
		{"BindAndLookup", testStoreBindLookup},
		{"HandoffCreatesNewBinding", testHandoff},
		{"DeleteCleansBindings", testDeleteCleansBindings},
		{"ListDelegatesToInner", testListDelegates},
		{"LookupMissReturnsNotFound", testLookupMiss_Store},
		{"BindFailsForMissingSession", testBindMissingSession},
	}
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func testCreateDelegates(t *testing.T) {
	store, inner := newTestStore(t)
	ctx := context.Background()

	sess, err := store.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, ok := inner.sessions[sess.ID]; !ok {
		t.Fatal("session should exist in inner store")
	}
}

func testStoreBindLookup(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	sess, _ := store.Create(ctx)
	if err := store.Bind(ctx, SurfaceLark, "oc_abc", sess.ID); err != nil {
		t.Fatalf("Bind: %v", err)
	}

	found, err := store.LookupByBinding(ctx, SurfaceLark, "oc_abc")
	if err != nil {
		t.Fatalf("LookupByBinding: %v", err)
	}
	if found.ID != sess.ID {
		t.Fatalf("got session %q, want %q", found.ID, sess.ID)
	}
}

func testHandoff(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	sess, _ := store.Create(ctx)
	_ = store.Bind(ctx, SurfaceCLI, "my-session", sess.ID)

	if err := store.Handoff(ctx, sess.ID, SurfaceLark, "oc_xyz"); err != nil {
		t.Fatalf("Handoff: %v", err)
	}

	found, err := store.LookupByBinding(ctx, SurfaceLark, "oc_xyz")
	if err != nil {
		t.Fatalf("LookupByBinding after handoff: %v", err)
	}
	if found.ID != sess.ID {
		t.Fatalf("handoff session mismatch: got %q, want %q", found.ID, sess.ID)
	}

	bindings, _ := store.ListBindings(ctx, sess.ID)
	if len(bindings) != 2 {
		t.Fatalf("expected 2 bindings, got %d", len(bindings))
	}
}

func testDeleteCleansBindings(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	sess, _ := store.Create(ctx)
	_ = store.Bind(ctx, SurfaceLark, "oc_del", sess.ID)
	_ = store.Bind(ctx, SurfaceCLI, "cli_del", sess.ID)

	if err := store.Delete(ctx, sess.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := store.LookupByBinding(ctx, SurfaceLark, "oc_del")
	if err != storage.ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound after delete, got %v", err)
	}

	bindings, _ := store.ListBindings(ctx, sess.ID)
	if len(bindings) != 0 {
		t.Fatalf("expected 0 bindings after delete, got %d", len(bindings))
	}
}

func testListDelegates(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, _ = store.Create(ctx)
	_, _ = store.Create(ctx)

	ids, err := store.List(ctx, 10, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 ids, got %d", len(ids))
	}
}

func testLookupMiss_Store(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	_, err := store.LookupByBinding(ctx, SurfaceWeb, "nonexistent")
	if err != storage.ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func testBindMissingSession(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	err := store.Bind(ctx, SurfaceLark, "oc_x", "no-such-session")
	if err == nil {
		t.Fatal("expected error when binding to nonexistent session")
	}
}

func TestListBindings_Empty(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	bindings, err := store.ListBindings(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("ListBindings: %v", err)
	}
	if len(bindings) != 0 {
		t.Fatalf("expected 0, got %d", len(bindings))
	}
}

func TestStore_NowFn(t *testing.T) {
	inner := newMockStore()
	indexPath := filepath.Join(t.TempDir(), "index.json")
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s, err := New(inner, indexPath, func() time.Time { return fixed })
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	ctx := context.Background()
	sess, _ := s.Create(ctx)
	_ = s.Bind(ctx, SurfaceLark, "oc_time", sess.ID)
	bindings, _ := s.ListBindings(ctx, sess.ID)
	if len(bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(bindings))
	}
	if !bindings[0].BoundAt.Equal(fixed) {
		t.Fatalf("BoundAt = %v, want %v", bindings[0].BoundAt, fixed)
	}
}
