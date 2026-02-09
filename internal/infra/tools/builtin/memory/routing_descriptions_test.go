package memory

import (
	"strings"
	"testing"
)

func TestMemoryGetDescriptionExpressesRepoBoundary(t *testing.T) {
	t.Parallel()

	desc := NewMemoryGet(nil).Definition().Description
	if !strings.Contains(desc, "after memory_search returns a memory path") || !strings.Contains(desc, "repository/workspace source files") {
		t.Fatalf("expected memory_get description to express memory-vs-repo boundary, got %q", desc)
	}
}

