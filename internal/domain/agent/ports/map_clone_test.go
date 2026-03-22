package ports

import "testing"

func TestCloneStringMap(t *testing.T) {
	if got := CloneStringMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", got)
	}
	if got := CloneStringMap(map[string]string{}); got != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", got)
	}

	src := map[string]string{"mode": "plan"}
	cloned := CloneStringMap(src)
	if len(cloned) != 1 || cloned["mode"] != "plan" {
		t.Fatalf("unexpected clone contents: %+v", cloned)
	}

	src["mode"] = "execute"
	if cloned["mode"] != "plan" {
		t.Fatalf("expected clone to remain unchanged after source mutation, got %+v", cloned)
	}

	cloned["new"] = "value"
	if _, ok := src["new"]; ok {
		t.Fatalf("expected source to remain unchanged after clone mutation, got %+v", src)
	}
}

func TestDeepCloneAnyMap(t *testing.T) {
	if got := DeepCloneAnyMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", got)
	}
	if got := DeepCloneAnyMap(map[string]any{}); got != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", got)
	}

	src := map[string]any{"count": 3, "name": "demo"}
	cloned := DeepCloneAnyMap(src)
	if len(cloned) != 2 || cloned["count"] != 3 || cloned["name"] != "demo" {
		t.Fatalf("unexpected clone contents: %+v", cloned)
	}

	src["name"] = "changed"
	if cloned["name"] != "demo" {
		t.Fatalf("expected clone to remain unchanged after source mutation, got %+v", cloned)
	}

	cloned["extra"] = true
	if _, ok := src["extra"]; ok {
		t.Fatalf("expected source to remain unchanged after clone mutation, got %+v", src)
	}
}

func TestDeepCloneAnyMap_NestedMaps(t *testing.T) {
	inner := map[string]any{"key": "value"}
	src := map[string]any{"nested": inner}
	cloned := DeepCloneAnyMap(src)

	// Mutating the inner map of src should NOT affect cloned
	inner["key"] = "mutated"
	if clonedNested, ok := cloned["nested"].(map[string]any); ok {
		if clonedNested["key"] != "value" {
			t.Fatalf("deep clone did not isolate nested map: got %v", clonedNested["key"])
		}
	} else {
		t.Fatal("expected nested map in clone")
	}
}

func TestCloneAnyMap_BackwardCompat(t *testing.T) {
	src := map[string]any{"a": 1, "nested": map[string]any{"b": 2}}
	cloned := CloneAnyMap(src)
	if cloned["a"] != 1 {
		t.Fatal("backward compat CloneAnyMap failed")
	}
	// Verify deep clone via backward compat alias
	nested := src["nested"].(map[string]any)
	nested["b"] = 999
	clonedNested := cloned["nested"].(map[string]any)
	if clonedNested["b"] != 2 {
		t.Fatal("CloneAnyMap backward compat should deep clone nested maps")
	}
}
