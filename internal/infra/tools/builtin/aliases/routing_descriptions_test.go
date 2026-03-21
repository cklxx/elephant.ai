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
	if !strings.Contains(replaceDesc, "surgical edit") || !strings.Contains(replaceDesc, "read_file") {
		t.Fatalf("expected replace_in_file description to mention surgical edit scope with cross-refs, got %q", replaceDesc)
	}

	writeDesc := NewWriteFile(cfg).Definition().Description
	if !strings.Contains(writeDesc, "create a new file") || !strings.Contains(writeDesc, "replace_in_file") {
		t.Fatalf("expected write_file description to mention create-vs-replace boundary, got %q", writeDesc)
	}

	readDesc := NewReadFile(cfg).Definition().Description
	if !strings.Contains(readDesc, "inspect file contents") || !strings.Contains(readDesc, "replace_in_file") {
		t.Fatalf("expected read_file description to mention read-only scope, got %q", readDesc)
	}
}

func TestShellExecDescriptionExpressesBoundary(t *testing.T) {
	t.Parallel()

	cfg := shared.ShellToolConfig{}

	shellDesc := NewShellExec(cfg).Definition().Description
	if !strings.Contains(shellDesc, "shell commands") {
		t.Fatalf("expected shell_exec description to mention shell commands, got %q", shellDesc)
	}
}
