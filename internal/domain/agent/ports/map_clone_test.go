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

func TestCloneAnyMap(t *testing.T) {
	if got := CloneAnyMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", got)
	}
	if got := CloneAnyMap(map[string]any{}); got != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", got)
	}

	src := map[string]any{"count": 3, "name": "demo"}
	cloned := CloneAnyMap(src)
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
