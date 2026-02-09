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
	if !strings.Contains(searchDesc, "path/name discovery") || !strings.Contains(searchDesc, "file bodies") {
		t.Fatalf("expected search_file description to mention discovery boundary, got %q", searchDesc)
	}

	replaceDesc := NewReplaceInFile(cfg).Definition().Description
	if !strings.Contains(replaceDesc, "in-place patch/hotfix") || !strings.Contains(replaceDesc, "artifact deletion/cleanup") {
		t.Fatalf("expected replace_in_file description to mention in-place-only scope, got %q", replaceDesc)
	}

	readDesc := NewReadFile(cfg).Definition().Description
	if !strings.Contains(readDesc, "context windows") || !strings.Contains(readDesc, "memory_search/memory_get") || !strings.Contains(readDesc, "lark_chat_history") {
		t.Fatalf("expected read_file description to mention repo-vs-memory boundary, got %q", readDesc)
	}
}
