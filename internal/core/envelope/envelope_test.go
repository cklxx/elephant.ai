package envelope

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	m := map[string]any{"content": "hello", "user": "alice"}
	e := New(m)

	// Verify fields are set
	v, ok := e.Get("content")
	if !ok || v != "hello" {
		t.Errorf("Get(content): got %v, ok=%v", v, ok)
	}
}

func TestNew_DefensiveCopy(t *testing.T) {
	m := map[string]any{"key": "original"}
	e := New(m)

	// Mutate the original map
	m["key"] = "mutated"

	v, _ := e.Get("key")
	if v != "original" {
		t.Error("New did not make a defensive copy of the input map")
	}
}

func TestNew_NilMap(t *testing.T) {
	e := New(nil)
	_, ok := e.Get("anything")
	if ok {
		t.Error("expected ok=false for nil-initialized envelope")
	}
}

func TestFieldOf(t *testing.T) {
	e := New(map[string]any{
		"name":  "alice",
		"count": 42,
		"flag":  true,
	})

	t.Run("string", func(t *testing.T) {
		got := FieldOf[string](e, "name")
		if got != "alice" {
			t.Errorf("got %q, want %q", got, "alice")
		}
	})

	t.Run("int", func(t *testing.T) {
		got := FieldOf[int](e, "count")
		if got != 42 {
			t.Errorf("got %d, want 42", got)
		}
	})

	t.Run("bool", func(t *testing.T) {
		got := FieldOf[bool](e, "flag")
		if got != true {
			t.Errorf("got %v, want true", got)
		}
	})

	t.Run("wrong type returns zero", func(t *testing.T) {
		got := FieldOf[int](e, "name") // name is string, not int
		if got != 0 {
			t.Errorf("got %d, want 0 for wrong type", got)
		}
	})

	t.Run("missing key returns zero", func(t *testing.T) {
		got := FieldOf[string](e, "nonexistent")
		if got != "" {
			t.Errorf("got %q, want empty string for missing key", got)
		}
	})
}

func TestContentOf(t *testing.T) {
	e := New(map[string]any{"content": "hello world"})
	if got := e.ContentOf(); got != "hello world" {
		t.Errorf("ContentOf: got %q, want %q", got, "hello world")
	}
}

func TestContentOf_Missing(t *testing.T) {
	e := New(map[string]any{})
	if got := e.ContentOf(); got != "" {
		t.Errorf("ContentOf on empty: got %q, want empty", got)
	}
}

func TestNormalize_AddsTimestamp(t *testing.T) {
	e := New(map[string]any{"content": "hi"})
	n := e.Normalize()

	ts, ok := n.Get("timestamp")
	if !ok {
		t.Fatal("Normalize did not add timestamp")
	}
	s, ok := ts.(string)
	if !ok || s == "" {
		t.Errorf("timestamp: got %v, want non-empty string", ts)
	}
}

func TestNormalize_PreservesExistingTimestamp(t *testing.T) {
	e := New(map[string]any{"timestamp": "2024-01-01T00:00:00Z"})
	n := e.Normalize()

	ts, _ := n.Get("timestamp")
	if ts != "2024-01-01T00:00:00Z" {
		t.Errorf("Normalize overwrote existing timestamp: got %v", ts)
	}
}

func TestNormalize_DoesNotMutateOriginal(t *testing.T) {
	e := New(map[string]any{"content": "hi"})
	_ = e.Normalize()

	_, ok := e.Get("timestamp")
	if ok {
		t.Error("Normalize mutated the original envelope")
	}
}

func TestSet_Immutability(t *testing.T) {
	e := New(map[string]any{"a": 1})
	e2 := e.Set("b", 2)

	// Original should not have "b"
	_, ok := e.Get("b")
	if ok {
		t.Error("Set mutated the original envelope")
	}

	// New envelope should have both
	v, ok := e2.Get("a")
	if !ok || v != 1 {
		t.Error("Set lost existing field")
	}
	v, ok = e2.Get("b")
	if !ok || v != 2 {
		t.Error("Set did not add new field")
	}
}

func TestFields_ReturnsCopy(t *testing.T) {
	e := New(map[string]any{"key": "val"})
	fields := e.Fields()

	// Mutate the returned map
	fields["key"] = "mutated"

	v, _ := e.Get("key")
	if v == "mutated" {
		t.Error("Fields() returned a reference instead of a copy")
	}
}

func TestFields_Content(t *testing.T) {
	e := New(map[string]any{"a": 1, "b": "two"})
	fields := e.Fields()

	if len(fields) != 2 {
		t.Errorf("Fields length: got %d, want 2", len(fields))
	}
	if fields["a"] != 1 || fields["b"] != "two" {
		t.Errorf("Fields content mismatch: %v", fields)
	}
}

func TestString_WithContent(t *testing.T) {
	e := New(map[string]any{"content": "the message"})
	if got := e.String(); got != "the message" {
		t.Errorf("String: got %q, want %q", got, "the message")
	}
}

func TestString_WithoutContent(t *testing.T) {
	e := New(map[string]any{"a": 1, "b": 2})
	got := e.String()
	if !strings.HasPrefix(got, "Envelope{") {
		t.Errorf("String without content: got %q, want Envelope{...} format", got)
	}
	if !strings.Contains(got, "2 fields") {
		t.Errorf("String: got %q, want to contain '2 fields'", got)
	}
}

func TestString_EmptyEnvelope(t *testing.T) {
	e := New(nil)
	got := e.String()
	if !strings.Contains(got, "0 fields") {
		t.Errorf("String for empty: got %q, want '0 fields'", got)
	}
}
