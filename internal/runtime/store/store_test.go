package store_test

import (
	"testing"

	"alex/internal/runtime/session"
	"alex/internal/runtime/store"
)

func TestSaveLoadDelete(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	s := session.New("abc123", session.MemberClaudeCode, "test goal", "/tmp")
	_ = s.Transition(session.StateStarting)

	if err := st.Save(s); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := st.Load("abc123")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.ID != "abc123" || loaded.State != session.StateStarting {
		t.Fatalf("unexpected loaded session: %+v", loaded)
	}

	if err := st.Delete("abc123"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.Load("abc123"); err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestLoadAll(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"s1", "s2", "s3"} {
		s := session.New(id, session.MemberCodex, "goal", "/tmp")
		if err := st.Save(s); err != nil {
			t.Fatal(err)
		}
	}

	all, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(all))
	}
}

func TestLoad_notFound(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.New(dir)
	if _, err := st.Load("nonexistent"); err == nil {
		t.Fatal("expected error for missing session")
	}
}
