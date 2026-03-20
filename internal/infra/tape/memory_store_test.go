package tape

import (
	"context"
	"sync"
	"testing"
	"time"

	coretape "alex/internal/core/tape"
)

func TestMemoryStore_AppendAndQuery(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

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
}

func TestMemoryStore_QueryKinds(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	_ = s.Append(ctx, "t", coretape.NewMessage("user", "hi", coretape.EntryMeta{}))
	_ = s.Append(ctx, "t", coretape.NewAnchor("a1", coretape.EntryMeta{}))
	_ = s.Append(ctx, "t", coretape.NewMessage("assistant", "yo", coretape.EntryMeta{}))

	entries, err := s.Query(ctx, "t", coretape.Query().Kinds(coretape.KindMessage))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 messages", len(entries))
	}
}

func TestMemoryStore_QuerySessionID(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	_ = s.Append(ctx, "t", coretape.NewMessage("user", "a", coretape.EntryMeta{SessionID: "s1"}))
	_ = s.Append(ctx, "t", coretape.NewMessage("user", "b", coretape.EntryMeta{SessionID: "s2"}))

	entries, err := s.Query(ctx, "t", coretape.Query().SessionID("s1"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d, want 1", len(entries))
	}
	if c, _ := entries[0].Payload["content"].(string); c != "a" {
		t.Fatalf("got content %q, want 'a'", c)
	}
}

func TestMemoryStore_QueryBetweenDates(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	now := time.Now()
	e1 := coretape.NewMessage("user", "old", coretape.EntryMeta{})
	e1.Date = now.Add(-2 * time.Hour)
	e2 := coretape.NewMessage("user", "recent", coretape.EntryMeta{})
	e2.Date = now.Add(-30 * time.Minute)
	e3 := coretape.NewMessage("user", "future", coretape.EntryMeta{})
	e3.Date = now.Add(time.Hour)

	_ = s.Append(ctx, "t", e1)
	_ = s.Append(ctx, "t", e2)
	_ = s.Append(ctx, "t", e3)

	entries, err := s.Query(ctx, "t", coretape.Query().BetweenDates(now.Add(-time.Hour), now))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d, want 1", len(entries))
	}
}

func TestMemoryStore_QueryLimit(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	for i := 0; i < 10; i++ {
		_ = s.Append(ctx, "t", coretape.NewMessage("user", "msg", coretape.EntryMeta{Seq: int64(i)}))
	}

	entries, err := s.Query(ctx, "t", coretape.Query().Limit(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d, want 3", len(entries))
	}
}

func TestMemoryStore_QueryAfterAnchor(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	anchor := coretape.NewAnchor("checkpoint", coretape.EntryMeta{})
	_ = s.Append(ctx, "t", coretape.NewMessage("user", "before", coretape.EntryMeta{}))
	_ = s.Append(ctx, "t", anchor)
	_ = s.Append(ctx, "t", coretape.NewMessage("user", "after1", coretape.EntryMeta{}))
	_ = s.Append(ctx, "t", coretape.NewMessage("user", "after2", coretape.EntryMeta{}))

	entries, err := s.Query(ctx, "t", coretape.Query().AfterAnchor(anchor.ID))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d, want 2", len(entries))
	}
}

func TestMemoryStore_QueryAfterAnchorNotFound(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	_ = s.Append(ctx, "t", coretape.NewMessage("user", "msg", coretape.EntryMeta{}))

	_, err := s.Query(ctx, "t", coretape.Query().AfterAnchor("nonexistent"))
	if err == nil {
		t.Fatal("expected error for missing anchor")
	}
}

func TestMemoryStore_QuerySeqFilters(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	for i := int64(1); i <= 5; i++ {
		_ = s.Append(ctx, "t", coretape.NewMessage("user", "msg", coretape.EntryMeta{Seq: i}))
	}

	// beforeSeq=4 → seq 1,2,3
	entries, err := s.Query(ctx, "t", coretape.Query().BeforeSeq(4))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("beforeSeq: got %d, want 3", len(entries))
	}

	// afterSeq=3 → seq 4,5
	entries, err = s.Query(ctx, "t", coretape.Query().AfterSeq(3))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("afterSeq: got %d, want 2", len(entries))
	}
}

func TestMemoryStore_ListAndDelete(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	_ = s.Append(ctx, "b", coretape.NewMessage("user", "hi", coretape.EntryMeta{}))
	_ = s.Append(ctx, "a", coretape.NewMessage("user", "hi", coretape.EntryMeta{}))

	names, err := s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("list: got %v, want [a b]", names)
	}

	if err := s.Delete(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	names, err = s.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "b" {
		t.Fatalf("after delete: got %v, want [b]", names)
	}
}

func TestMemoryStore_ConcurrentAppend(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	const n = 100
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = s.Append(ctx, "t", coretape.NewMessage("user", "msg", coretape.EntryMeta{}))
		}()
	}
	wg.Wait()

	entries, err := s.Query(ctx, "t", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != n {
		t.Fatalf("got %d entries, want %d", len(entries), n)
	}
}

func TestMemoryStore_QueryEmptyTape(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	entries, err := s.Query(ctx, "nonexistent", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d entries, want 0", len(entries))
	}
}
