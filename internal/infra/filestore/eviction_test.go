package filestore

import (
	"testing"
	"time"
)

type timedItem struct {
	Name string
	At   time.Time
}

func ageFn(v timedItem) time.Time { return v.At }

func TestEvictByTTL_RemovesExpired(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	items := map[string]timedItem{
		"old": {Name: "old", At: now.Add(-2 * time.Hour)},
		"new": {Name: "new", At: now.Add(-30 * time.Minute)},
	}

	evicted := EvictByTTL(items, now, 1*time.Hour, ageFn)
	if evicted != 1 {
		t.Fatalf("expected 1 evicted, got %d", evicted)
	}
	if _, ok := items["old"]; ok {
		t.Fatal("old should be evicted")
	}
	if _, ok := items["new"]; !ok {
		t.Fatal("new should remain")
	}
}

func TestEvictByTTL_NoneExpired(t *testing.T) {
	now := time.Now()
	items := map[string]timedItem{
		"a": {Name: "a", At: now},
	}
	evicted := EvictByTTL(items, now, time.Hour, ageFn)
	if evicted != 0 {
		t.Fatalf("expected 0 evicted, got %d", evicted)
	}
}

func TestEvictByCap_RemovesOldest(t *testing.T) {
	items := map[string]timedItem{
		"a": {Name: "a", At: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		"b": {Name: "b", At: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)},
		"c": {Name: "c", At: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)},
	}

	evicted := EvictByCap(items, 2, ageFn)
	if evicted != 1 {
		t.Fatalf("expected 1 evicted, got %d", evicted)
	}
	if _, ok := items["a"]; ok {
		t.Fatal("a (oldest) should be evicted")
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(items))
	}
}

func TestEvictByCap_UnderCapNoOp(t *testing.T) {
	items := map[string]timedItem{
		"a": {Name: "a", At: time.Now()},
	}
	evicted := EvictByCap(items, 5, ageFn)
	if evicted != 0 {
		t.Fatalf("expected 0 evicted, got %d", evicted)
	}
}

func TestEvictByCap_ExactCapNoOp(t *testing.T) {
	items := map[string]timedItem{
		"a": {Name: "a", At: time.Now()},
		"b": {Name: "b", At: time.Now()},
	}
	evicted := EvictByCap(items, 2, ageFn)
	if evicted != 0 {
		t.Fatalf("expected 0 evicted, got %d", evicted)
	}
}
