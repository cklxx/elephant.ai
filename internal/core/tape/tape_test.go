package tape

import (
	"context"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Factory function tests
// ---------------------------------------------------------------------------

func TestNewMessage(t *testing.T) {
	e := NewMessage("user", "hello", EntryMeta{Model: "gpt-4"})
	assertEntry(t, e, KindMessage)
	assertEqual(t, "role", e.Payload["role"], "user")
	assertEqual(t, "content", e.Payload["content"], "hello")
	assertEqual(t, "meta.Model", e.Meta.Model, "gpt-4")
}

func TestNewSystem(t *testing.T) {
	e := NewSystem("you are helpful", EntryMeta{})
	assertEntry(t, e, KindSystem)
	assertEqual(t, "content", e.Payload["content"], "you are helpful")
}

func TestNewAnchor(t *testing.T) {
	e := NewAnchor("start", EntryMeta{})
	assertEntry(t, e, KindAnchor)
	assertEqual(t, "label", e.Payload["label"], "start")
}

func TestNewToolCall(t *testing.T) {
	args := map[string]any{"q": "weather"}
	e := NewToolCall("search", args, EntryMeta{})
	assertEntry(t, e, KindToolCall)
	assertEqual(t, "name", e.Payload["name"], "search")
	if e.Payload["args"] == nil {
		t.Fatal("expected args in payload")
	}
}

func TestNewToolResult(t *testing.T) {
	e := NewToolResult("search", "sunny", false, EntryMeta{})
	assertEntry(t, e, KindToolResult)
	assertEqual(t, "name", e.Payload["name"], "search")
	assertEqual(t, "result", e.Payload["result"], "sunny")
	assertEqual(t, "is_error", e.Payload["is_error"], false)
}

func TestNewToolResult_Error(t *testing.T) {
	e := NewToolResult("search", "timeout", true, EntryMeta{})
	assertEqual(t, "is_error", e.Payload["is_error"], true)
}

func TestNewError(t *testing.T) {
	e := NewError("something broke", "E001", EntryMeta{})
	assertEntry(t, e, KindError)
	assertEqual(t, "error", e.Payload["error"], "something broke")
	assertEqual(t, "code", e.Payload["code"], "E001")
}

func TestNewEvent(t *testing.T) {
	data := map[string]any{"key": "val"}
	e := NewEvent("task.started", data, EntryMeta{})
	assertEntry(t, e, KindEvent)
	assertEqual(t, "type", e.Payload["type"], "task.started")
	if e.Payload["data"] == nil {
		t.Fatal("expected data in payload")
	}
}

func TestNewThinking(t *testing.T) {
	e := NewThinking("let me think", EntryMeta{})
	assertEntry(t, e, KindThinking)
	assertEqual(t, "content", e.Payload["content"], "let me think")
}

func TestNewCheckpoint(t *testing.T) {
	state := map[string]any{"step": 3}
	e := NewCheckpoint("mid", state, EntryMeta{})
	assertEntry(t, e, KindCheckpoint)
	assertEqual(t, "label", e.Payload["label"], "mid")
	if e.Payload["state"] == nil {
		t.Fatal("expected state in payload")
	}
}

func TestFactoryIDsAreUnique(t *testing.T) {
	a := NewMessage("user", "a", EntryMeta{})
	b := NewMessage("user", "b", EntryMeta{})
	if a.ID == b.ID {
		t.Fatal("expected distinct IDs for separate entries")
	}
}

// ---------------------------------------------------------------------------
// TapeQuery builder immutability
// ---------------------------------------------------------------------------

func TestTapeQuery_Immutability(t *testing.T) {
	base := Query()

	q1 := base.AfterAnchor("anchor-1")
	if base.GetAfterAnchor() != "" {
		t.Fatal("AfterAnchor mutated the original query")
	}
	assertEqual(t, "q1.AfterAnchor", q1.GetAfterAnchor(), "anchor-1")

	q2 := base.Kinds(KindMessage, KindError)
	if len(base.GetKinds()) != 0 {
		t.Fatal("Kinds mutated the original query")
	}
	if len(q2.GetKinds()) != 2 {
		t.Fatalf("expected 2 kinds, got %d", len(q2.GetKinds()))
	}

	q3 := base.Limit(10)
	if base.GetLimit() != 0 {
		t.Fatal("Limit mutated the original query")
	}
	assertEqual(t, "q3.Limit", q3.GetLimit(), 10)

	q4 := base.SessionID("sess-1")
	if base.GetSessionID() != "" {
		t.Fatal("SessionID mutated the original query")
	}
	assertEqual(t, "q4.SessionID", q4.GetSessionID(), "sess-1")

	q5 := base.RunID("run-1")
	if base.GetRunID() != "" {
		t.Fatal("RunID mutated the original query")
	}
	assertEqual(t, "q5.RunID", q5.GetRunID(), "run-1")

	q6 := base.BeforeSeq(42)
	if base.GetBeforeSeq() != 0 {
		t.Fatal("BeforeSeq mutated the original query")
	}
	assertEqual(t, "q6.BeforeSeq", q6.GetBeforeSeq(), int64(42))

	q7 := base.AfterSeq(99)
	if base.GetAfterSeq() != 0 {
		t.Fatal("AfterSeq mutated the original query")
	}
	assertEqual(t, "q7.AfterSeq", q7.GetAfterSeq(), int64(99))

	now := time.Now()
	later := now.Add(time.Hour)
	q8 := base.BetweenDates(now, later)
	if !base.GetFromDate().IsZero() {
		t.Fatal("BetweenDates mutated the original query")
	}
	if q8.GetFromDate() != now || q8.GetToDate() != later {
		t.Fatal("BetweenDates did not set dates correctly")
	}
}

func TestTapeQuery_KindsDefensiveCopy(t *testing.T) {
	src := []EntryKind{KindMessage, KindError}
	q := Query().Kinds(src...)
	// Mutating the source slice should not affect the query.
	src[0] = KindEvent
	if q.GetKinds()[0] != KindMessage {
		t.Fatal("Kinds did not defensively copy the input slice")
	}
}

// ---------------------------------------------------------------------------
// TapeManager tests
// ---------------------------------------------------------------------------

type mockStore struct {
	entries map[string][]TapeEntry
	mu      sync.Mutex
}

func newMockStore() *mockStore {
	return &mockStore{entries: make(map[string][]TapeEntry)}
}

func (m *mockStore) Append(_ context.Context, tapeName string, entry TapeEntry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[tapeName] = append(m.entries[tapeName], entry)
	return nil
}

func (m *mockStore) Query(_ context.Context, tapeName string, _ TapeQuery) ([]TapeEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entries[tapeName], nil
}

func (m *mockStore) List(_ context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var names []string
	for k := range m.entries {
		names = append(names, k)
	}
	return names, nil
}

func (m *mockStore) Delete(_ context.Context, tapeName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, tapeName)
	return nil
}

func (m *mockStore) last(tapeName string) TapeEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	es := m.entries[tapeName]
	return es[len(es)-1]
}

func TestTapeManager_AppendFillsMeta(t *testing.T) {
	store := newMockStore()
	mgr := NewTapeManager(store, TapeContext{
		TapeName: "test-tape",
		RunID:    "run-ctx",
		Meta:     EntryMeta{SessionID: "sess-ctx"},
	})

	entry := NewMessage("user", "hi", EntryMeta{})
	if err := mgr.Append(context.Background(), entry); err != nil {
		t.Fatal(err)
	}

	got := store.last("test-tape")
	assertEqual(t, "SessionID", got.Meta.SessionID, "sess-ctx")
	assertEqual(t, "RunID", got.Meta.RunID, "run-ctx")
}

func TestTapeManager_AppendDoesNotOverwriteMeta(t *testing.T) {
	store := newMockStore()
	mgr := NewTapeManager(store, TapeContext{
		TapeName: "test-tape",
		RunID:    "run-ctx",
		Meta:     EntryMeta{SessionID: "sess-ctx"},
	})

	entry := NewMessage("user", "hi", EntryMeta{
		SessionID: "sess-existing",
		RunID:     "run-existing",
	})
	if err := mgr.Append(context.Background(), entry); err != nil {
		t.Fatal(err)
	}

	got := store.last("test-tape")
	assertEqual(t, "SessionID", got.Meta.SessionID, "sess-existing")
	assertEqual(t, "RunID", got.Meta.RunID, "run-existing")
}

func TestTapeManager_QueryDelegatesToStore(t *testing.T) {
	store := newMockStore()
	mgr := NewTapeManager(store, TapeContext{TapeName: "tp", RunID: "r1", Meta: EntryMeta{SessionID: "s1"}})

	_ = mgr.Append(context.Background(), NewMessage("user", "a", EntryMeta{}))
	_ = mgr.Append(context.Background(), NewMessage("user", "b", EntryMeta{}))

	entries, err := mgr.Query(context.Background(), Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
}

func TestTapeManager_StoreAndContextAccessors(t *testing.T) {
	store := newMockStore()
	tctx := TapeContext{TapeName: "tp", RunID: "r1"}
	mgr := NewTapeManager(store, tctx)

	if mgr.Store() != store {
		t.Fatal("Store() returned wrong store")
	}
	if mgr.Context().TapeName != "tp" {
		t.Fatal("Context() returned wrong tape name")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertEntry(t *testing.T, e TapeEntry, wantKind EntryKind) {
	t.Helper()
	if e.Kind != wantKind {
		t.Fatalf("Kind = %q, want %q", e.Kind, wantKind)
	}
	if e.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if e.Date.IsZero() {
		t.Fatal("expected non-zero Date")
	}
}

func assertEqual[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", field, got, want)
	}
}
