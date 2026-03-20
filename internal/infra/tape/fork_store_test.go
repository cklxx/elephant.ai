package tape

import (
	"context"
	"testing"

	coretape "alex/internal/core/tape"
)

func TestForkStore_ReadIncludesParent(t *testing.T) {
	ctx := context.Background()
	parent := NewMemoryStore()
	_ = parent.Append(ctx, "t", coretape.NewMessage("user", "parent-msg", coretape.EntryMeta{}))

	fork := NewForkStore(parent)
	fork.Fork("t")

	_ = fork.Append(ctx, "t", coretape.NewMessage("assistant", "fork-msg", coretape.EntryMeta{}))

	entries, err := fork.Query(ctx, "t", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2 (parent+fork)", len(entries))
	}
}

func TestForkStore_WritesDoNotAffectParent(t *testing.T) {
	ctx := context.Background()
	parent := NewMemoryStore()

	fork := NewForkStore(parent)
	fork.Fork("t")

	_ = fork.Append(ctx, "t", coretape.NewMessage("user", "fork-only", coretape.EntryMeta{}))

	parentEntries, err := parent.Query(ctx, "t", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(parentEntries) != 0 {
		t.Fatalf("parent got %d entries, want 0", len(parentEntries))
	}
}

func TestForkStore_Merge(t *testing.T) {
	ctx := context.Background()
	parent := NewMemoryStore()

	fork := NewForkStore(parent)
	fork.Fork("t")

	_ = fork.Append(ctx, "t", coretape.NewMessage("user", "merged", coretape.EntryMeta{}))

	if err := fork.Merge(ctx); err != nil {
		t.Fatal(err)
	}

	parentEntries, err := parent.Query(ctx, "t", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(parentEntries) != 1 {
		t.Fatalf("parent got %d entries after merge, want 1", len(parentEntries))
	}

	// Fork should be empty after merge.
	forkEntries, err := fork.Query(ctx, "t", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	// After merge, fork state is cleared; queries go to parent.
	if len(forkEntries) != 1 {
		t.Fatalf("fork got %d entries after merge, want 1 (from parent)", len(forkEntries))
	}
}

func TestForkStore_Discard(t *testing.T) {
	ctx := context.Background()
	parent := NewMemoryStore()
	_ = parent.Append(ctx, "t", coretape.NewMessage("user", "parent-msg", coretape.EntryMeta{}))

	fork := NewForkStore(parent)
	fork.Fork("t")

	_ = fork.Append(ctx, "t", coretape.NewMessage("user", "discarded", coretape.EntryMeta{}))
	fork.Discard()

	// After discard, fork is no longer active — queries hit parent directly.
	entries, err := fork.Query(ctx, "t", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries after discard, want 1 (parent only)", len(entries))
	}
}

func TestForkStore_UnforkedTapeDelegatesToParent(t *testing.T) {
	ctx := context.Background()
	parent := NewMemoryStore()

	fork := NewForkStore(parent)

	// Write to unforked tape — should go directly to parent.
	_ = fork.Append(ctx, "t", coretape.NewMessage("user", "direct", coretape.EntryMeta{}))

	parentEntries, err := parent.Query(ctx, "t", coretape.Query())
	if err != nil {
		t.Fatal(err)
	}
	if len(parentEntries) != 1 {
		t.Fatalf("parent got %d entries, want 1", len(parentEntries))
	}
}

func TestForkStore_ListIncludesForks(t *testing.T) {
	ctx := context.Background()
	parent := NewMemoryStore()
	_ = parent.Append(ctx, "parent-tape", coretape.NewMessage("user", "hi", coretape.EntryMeta{}))

	fork := NewForkStore(parent)
	fork.Fork("fork-tape")

	names, err := fork.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("got %d names, want 2", len(names))
	}
}
