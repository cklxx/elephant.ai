package utils

import "testing"

func TestCloneSlice(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := CloneSlice[int](nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		got := CloneSlice([]int{})
		if got == nil || len(got) != 0 {
			t.Fatalf("expected empty slice, got %v", got)
		}
	})
	t.Run("values", func(t *testing.T) {
		src := []string{"a", "b", "c"}
		got := CloneSlice(src)
		if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
			t.Fatalf("unexpected clone: %v", got)
		}
		got[0] = "x"
		if src[0] == "x" {
			t.Fatal("clone shares backing array with source")
		}
	})
}

func TestCloneMap(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if got := CloneMap[string, int](nil); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		got := CloneMap(map[string]int{})
		if got == nil || len(got) != 0 {
			t.Fatalf("expected empty map, got %v", got)
		}
	})
	t.Run("values", func(t *testing.T) {
		src := map[string]string{"a": "1", "b": "2"}
		got := CloneMap(src)
		if len(got) != 2 || got["a"] != "1" || got["b"] != "2" {
			t.Fatalf("unexpected clone: %v", got)
		}
		got["a"] = "x"
		if src["a"] == "x" {
			t.Fatal("clone shares map with source")
		}
	})
}
