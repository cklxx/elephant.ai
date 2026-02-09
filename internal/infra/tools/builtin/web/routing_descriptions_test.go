package web

import (
	"strings"
	"testing"

	"alex/internal/infra/tools/builtin/shared"
)

func TestWebToolDescriptionsExpressDiscoveryVsFetchBoundary(t *testing.T) {
	t.Parallel()

	searchDesc := NewWebSearch("", WebSearchConfig{}).Definition().Description
	if !strings.Contains(searchDesc, "Discover authoritative web sources") || !strings.Contains(searchDesc, "use web_fetch") {
		t.Fatalf("expected web_search description to express discover-first boundary, got %q", searchDesc)
	}

	fetchDesc := NewWebFetch(shared.WebFetchConfig{}).Definition().Description
	if !strings.Contains(fetchDesc, "known/approved exact URL") {
		t.Fatalf("expected web_fetch description to express single-url retrieval boundary, got %q", fetchDesc)
	}
}
