package web

import (
	"strings"
	"testing"
)

func TestWebSearchDescriptionExpressesDiscoveryBoundary(t *testing.T) {
	t.Parallel()

	searchDesc := NewWebSearch("", WebSearchConfig{}).Definition().Description
	if !strings.Contains(searchDesc, "find web sources") || !strings.Contains(searchDesc, "no trusted URL") {
		t.Fatalf("expected web_search description to express discover-first boundary, got %q", searchDesc)
	}
}
