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

func TestMemorySearchDescriptionExpressesDiscoveryBoundary(t *testing.T) {
	t.Parallel()

	desc := NewMemorySearch(nil).Definition().Description
	if !strings.Contains(desc, "discover or recall prior preferences") ||
		!strings.Contains(desc, "before opening a specific note with memory_get") ||
		!strings.Contains(desc, "memory_related") {
		t.Fatalf("expected memory_search description to express discover-vs-open boundary, got %q", desc)
	}
}

func TestMemoryRelatedDescriptionExpressesLinkTraversalBoundary(t *testing.T) {
	t.Parallel()

	desc := NewMemoryRelated(nil).Definition().Description
	if !strings.Contains(desc, "related to a known memory path") || !strings.Contains(desc, "Not for repository/workspace source files") {
		t.Fatalf("expected memory_related description to express link-traversal boundary, got %q", desc)
	}
}
