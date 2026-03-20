package tape

import (
	"context"
	"testing"
	"time"

	coretape "alex/internal/core/tape"
)

func TestFileStore_AppendAndQuery(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	e1 := coretape.NewMessage("user", "hello", coretape.EntryMeta{SessionID: "s1", Seq: 1})
	e2 := coretape.NewMessage("assistant", "hi", coretape.EntryMeta{SessionID: "s1", Seq: 2})

	if err := s.Append(ctx, "tape1", e1); err != nil {
		t.Fatal(err)
	}
	if err := s.Append(ctx, "tape1", e2); err != nil {
		t.Fatal(err)
	}

	entries, err := s.Query(ctx, "tape1", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if c, _ := entries[0].Payload["content"].(string); c != "hello" {
		t.Fatalf("got content %q, want 'hello'", c)
	}
}

func TestFileStore_List(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	_ = s.Append(ctx, "bravo", coretape.NewMessage("user", "b", coretape.EntryMeta{}))
	_ = s.Append(ctx, "alpha", coretape.NewMessage("user", "a", coretape.EntryMeta{}))

	names, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "alpha" || names[1] != "bravo" {
		t.Fatalf("list: got %v, want [alpha bravo]", names)
	}
}

func TestFileStore_Delete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	_ = s.Append(ctx, "tape1", coretape.NewMessage("user", "hi", coretape.EntryMeta{}))

	if err := s.Delete(ctx, "tape1"); err != nil {
		t.Fatal(err)
	}

	names, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("got %d tapes after delete, want 0", len(names))
	}
}

func TestFileStore_DeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(context.Background(), "nope"); err != nil {
		t.Fatalf("delete nonexistent: %v", err)
	}
}

func TestFileStore_QueryFilters(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	now := time.Now()
	e1 := coretape.NewMessage("user", "a", coretape.EntryMeta{SessionID: "s1", Seq: 1})
	e1.Date = now.Add(-time.Hour)
	e2 := coretape.NewMessage("user", "b", coretape.EntryMeta{SessionID: "s2", Seq: 2})
	e2.Date = now
	e3 := coretape.NewAnchor("cp", coretape.EntryMeta{SessionID: "s1", Seq: 3})
	e3.Date = now.Add(time.Hour)

	_ = s.Append(ctx, "t", e1)
	_ = s.Append(ctx, "t", e2)
	_ = s.Append(ctx, "t", e3)

	// Filter by kind.
	entries, err := s.Query(ctx, "t", coretape.Query().Kinds(coretape.KindMessage))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("kinds filter: got %d, want 2", len(entries))
	}

	// Filter by sessionID.
	entries, err = s.Query(ctx, "t", coretape.Query().SessionID("s1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("session filter: got %d, want 2", len(entries))
	}

	// Limit.
	entries, err = s.Query(ctx, "t", coretape.Query().Limit(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("limit: got %d, want 1", len(entries))
	}
}

func TestFileStore_QueryEmptyTape(t *testing.T) {
	dir := t.TempDir()
	s, err := NewFileStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := s.Query(context.Background(), "nonexistent", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}
