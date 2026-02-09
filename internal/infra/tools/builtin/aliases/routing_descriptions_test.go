package aliases

import (
	"strings"
	"testing"

	"alex/internal/infra/tools/builtin/shared"
)

func TestFileToolDescriptionsExpressRoutingBoundaries(t *testing.T) {
	t.Parallel()

	cfg := shared.FileToolConfig{}

	searchDesc := NewSearchFile(cfg).Definition().Description
	if !strings.Contains(searchDesc, "path/name discovery") {
		t.Fatalf("expected search_file description to mention discovery boundary, got %q", searchDesc)
	}

	replaceDesc := NewReplaceInFile(cfg).Definition().Description
	if !strings.Contains(replaceDesc, "in-place edits") || !strings.Contains(replaceDesc, "clarification questions") {
		t.Fatalf("expected replace_in_file description to mention in-place-only scope, got %q", replaceDesc)
	}
}

