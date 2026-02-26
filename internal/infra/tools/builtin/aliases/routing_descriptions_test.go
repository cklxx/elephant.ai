package aliases

import (
	"strings"
	"testing"

	"alex/internal/infra/tools/builtin/shared"
)

func TestFileToolDescriptionsExpressRoutingBoundaries(t *testing.T) {
	t.Parallel()

	cfg := shared.FileToolConfig{}

	replaceDesc := NewReplaceInFile(cfg).Definition().Description
	if !strings.Contains(replaceDesc, "in-place patch/hotfix") || !strings.Contains(replaceDesc, "artifact deletion/cleanup") {
		t.Fatalf("expected replace_in_file description to mention in-place-only scope, got %q", replaceDesc)
	}

	writeDesc := NewWriteFile(cfg).Definition().Description
	if !strings.Contains(writeDesc, "new file") || !strings.Contains(writeDesc, "For in-place edits to existing text use replace_in_file") {
		t.Fatalf("expected write_file description to mention create-vs-replace boundary, got %q", writeDesc)
	}

	readDesc := NewReadFile(cfg).Definition().Description
	if !strings.Contains(readDesc, "read-only inspection/extraction") {
		t.Fatalf("expected read_file description to mention read-only scope, got %q", readDesc)
	}
}

func TestShellExecDescriptionExpressesBoundary(t *testing.T) {
	t.Parallel()

	cfg := shared.ShellToolConfig{}

	shellDesc := NewShellExec(cfg).Definition().Description
	if !strings.Contains(shellDesc, "terminal evidence collection") {
		t.Fatalf("expected shell_exec description to mention terminal boundary, got %q", shellDesc)
	}
}
