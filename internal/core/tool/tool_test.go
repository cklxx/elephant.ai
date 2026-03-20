package tool

import (
	"testing"
)

func TestNewToolSet(t *testing.T) {
	a := Tool{Name: "alpha", Description: "first"}
	b := Tool{Name: "beta", Description: "second"}
	c := Tool{Name: "gamma", Description: "third"}

	ts := NewToolSet(a, b, c)

	if ts.Len() != 3 {
		t.Fatalf("Len: got %d, want 3", ts.Len())
	}
}

func TestNewToolSet_Duplicates(t *testing.T) {
	a1 := Tool{Name: "alpha", Description: "v1"}
	a2 := Tool{Name: "alpha", Description: "v2"}
	b := Tool{Name: "beta", Description: "b"}

	ts := NewToolSet(a1, b, a2)

	if ts.Len() != 2 {
		t.Fatalf("Len: got %d, want 2 (duplicates collapsed)", ts.Len())
	}
	got, ok := ts.Get("alpha")
	if !ok {
		t.Fatal("Get(alpha): not found")
	}
	if got.Description != "v2" {
		t.Errorf("Get(alpha).Description: got %q, want %q (last wins)", got.Description, "v2")
	}
}

func TestToolSet_Get(t *testing.T) {
	ts := NewToolSet(
		Tool{Name: "read", Description: "reads"},
		Tool{Name: "write", Description: "writes"},
	)

	t.Run("existing", func(t *testing.T) {
		tool, ok := ts.Get("read")
		if !ok {
			t.Fatal("expected ok=true")
		}
		if tool.Name != "read" {
			t.Errorf("Name: got %q, want %q", tool.Name, "read")
		}
	})

	t.Run("missing", func(t *testing.T) {
		_, ok := ts.Get("delete")
		if ok {
			t.Error("expected ok=false for missing tool")
		}
	})
}

func TestToolSet_Names_InsertionOrder(t *testing.T) {
	ts := NewToolSet(
		Tool{Name: "charlie"},
		Tool{Name: "alpha"},
		Tool{Name: "bravo"},
	)

	names := ts.Names()
	want := []string{"charlie", "alpha", "bravo"}
	if len(names) != len(want) {
		t.Fatalf("Names length: got %d, want %d", len(names), len(want))
	}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("Names[%d]: got %q, want %q", i, n, want[i])
		}
	}
}

func TestToolSet_Names_ReturnsCopy(t *testing.T) {
	ts := NewToolSet(Tool{Name: "a"}, Tool{Name: "b"})
	names := ts.Names()
	names[0] = "MUTATED"

	original := ts.Names()
	if original[0] == "MUTATED" {
		t.Error("Names() returned a reference instead of a copy")
	}
}

func TestToolSet_Schemas(t *testing.T) {
	a := Tool{Name: "a", Description: "da"}
	b := Tool{Name: "b", Description: "db"}
	ts := NewToolSet(a, b)

	schemas := ts.Schemas()
	if len(schemas) != 2 {
		t.Fatalf("Schemas length: got %d, want 2", len(schemas))
	}
	if schemas[0].Name != "a" || schemas[1].Name != "b" {
		t.Errorf("Schemas order: got [%s, %s], want [a, b]", schemas[0].Name, schemas[1].Name)
	}
}

func TestToolSet_Merge(t *testing.T) {
	base := NewToolSet(
		Tool{Name: "read", Description: "base-read"},
		Tool{Name: "write", Description: "base-write"},
	)
	override := NewToolSet(
		Tool{Name: "write", Description: "override-write"},
		Tool{Name: "exec", Description: "override-exec"},
	)

	merged := base.Merge(override)

	if merged.Len() != 3 {
		t.Fatalf("Len: got %d, want 3", merged.Len())
	}

	// "write" should be overridden by other
	w, _ := merged.Get("write")
	if w.Description != "override-write" {
		t.Errorf("write.Description: got %q, want %q", w.Description, "override-write")
	}

	// "read" preserved from base
	r, _ := merged.Get("read")
	if r.Description != "base-read" {
		t.Errorf("read.Description: got %q, want %q", r.Description, "base-read")
	}

	// "exec" added from other
	e, ok := merged.Get("exec")
	if !ok {
		t.Fatal("exec not found in merged set")
	}
	if e.Description != "override-exec" {
		t.Errorf("exec.Description: got %q, want %q", e.Description, "override-exec")
	}

	// Check order: base tools first, then new tools from other
	names := merged.Names()
	want := []string{"read", "write", "exec"}
	for i, n := range names {
		if n != want[i] {
			t.Errorf("Names[%d]: got %q, want %q", i, n, want[i])
		}
	}
}

func TestToolSet_Merge_DoesNotMutateOriginal(t *testing.T) {
	base := NewToolSet(Tool{Name: "a"})
	other := NewToolSet(Tool{Name: "b"})

	_ = base.Merge(other)

	if base.Len() != 1 {
		t.Error("Merge mutated the base ToolSet")
	}
}

func TestNewToolContext(t *testing.T) {
	tc := NewToolContext("my-tape", "run-123", "sess-456")

	if tc.TapeName != "my-tape" {
		t.Errorf("TapeName: got %q, want %q", tc.TapeName, "my-tape")
	}
	if tc.RunID != "run-123" {
		t.Errorf("RunID: got %q, want %q", tc.RunID, "run-123")
	}
	if tc.SessionID != "sess-456" {
		t.Errorf("SessionID: got %q, want %q", tc.SessionID, "sess-456")
	}
	if tc.Meta == nil {
		t.Error("Meta map not initialized")
	}
	if tc.State == nil {
		t.Error("State map not initialized")
	}
}
