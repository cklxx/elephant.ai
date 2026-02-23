package memory

import "testing"

func TestExtractMemoryEdgesParsesMemoryAndMarkdownLinks(t *testing.T) {
	text := `
See [[memory:memory/2026-02-02.md#Deploy]] and [[memory:MEMORY.md]].
Also [summary](memory/2026-02-03.md#Summary) for follow-up.
Duplicate [[memory:memory/2026-02-02.md#Deploy]] should be deduped.
`
	edges := extractMemoryEdges(text)
	if len(edges) != 3 {
		t.Fatalf("expected 3 unique edges, got %d (%+v)", len(edges), edges)
	}
	if edges[0].EdgeType != "related" || edges[0].Direction != "directed" {
		t.Fatalf("expected default relation metadata, got %+v", edges[0])
	}
}

func TestNormalizeLinkedPathRejectsURLs(t *testing.T) {
	if got := normalizeLinkedPath("https://example.com/a.md"); got != "" {
		t.Fatalf("expected URLs to be rejected, got %q", got)
	}
	if got := normalizeLinkedPath("../secrets.md"); got != "" {
		t.Fatalf("expected parent traversal paths to be rejected, got %q", got)
	}
	if got := normalizeLinkedPath("./memory/2026-02-02.md"); got != "memory/2026-02-02.md" {
		t.Fatalf("expected cleaned memory path, got %q", got)
	}
}

func TestBuildNodeID(t *testing.T) {
	if got := buildNodeID("memory/2026-02-02.md", 3, 8); got != "memory/2026-02-02.md:3-8" {
		t.Fatalf("unexpected node id: %q", got)
	}
}
