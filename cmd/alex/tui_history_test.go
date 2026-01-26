package main

import "testing"

func TestInputHistoryNavigation(t *testing.T) {
	history := newInputHistory()
	history.Add("one")
	history.Add("two")

	draft := "draft"
	value, ok := history.Prev(draft)
	if !ok || value != "two" {
		t.Fatalf("prev expected %q, got %q (ok=%v)", "two", value, ok)
	}

	value, ok = history.Prev(value)
	if !ok || value != "one" {
		t.Fatalf("prev expected %q, got %q (ok=%v)", "one", value, ok)
	}

	value, ok = history.Next(value)
	if !ok || value != "two" {
		t.Fatalf("next expected %q, got %q (ok=%v)", "two", value, ok)
	}

	value, ok = history.Next(value)
	if !ok || value != draft {
		t.Fatalf("next expected draft %q, got %q (ok=%v)", draft, value, ok)
	}

	value, ok = history.Next("changed")
	if ok {
		t.Fatalf("next expected no change at draft, got %q (ok=%v)", value, ok)
	}
}

func TestInputHistoryDedup(t *testing.T) {
	history := newInputHistory()
	history.Add("same")
	history.Add("same")
	if len(history.entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(history.entries))
	}
}
