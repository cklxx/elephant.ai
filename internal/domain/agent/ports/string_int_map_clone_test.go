package ports

import "testing"

func TestCloneStringIntMapReturnsNilForNilOrEmptyInput(t *testing.T) {
	if got := CloneStringIntMap(nil); got != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", got)
	}
	if got := CloneStringIntMap(map[string]int{}); got != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", got)
	}
}

func TestCloneStringIntMapCreatesIndependentCopy(t *testing.T) {
	src := map[string]int{
		"seed.png": 2,
	}

	cloned := CloneStringIntMap(src)
	if len(cloned) != 1 || cloned["seed.png"] != 2 {
		t.Fatalf("unexpected clone contents: %+v", cloned)
	}

	src["seed.png"] = 9
	if cloned["seed.png"] != 2 {
		t.Fatalf("expected clone to remain unchanged after source mutation, got %+v", cloned)
	}

	cloned["extra.png"] = 4
	if _, ok := src["extra.png"]; ok {
		t.Fatalf("expected source to remain unchanged after clone mutation, got %+v", src)
	}
}
