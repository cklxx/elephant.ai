package sandbox

import (
	"strings"
	"testing"
)

func TestSandboxBrowserDescriptionsExpressReadVsActionBoundary(t *testing.T) {
	t.Parallel()

	cfg := SandboxConfig{}
	actionDesc := NewSandboxBrowser(cfg).Definition().Description
	if !strings.Contains(actionDesc, "only to inspect URL/title/session metadata") || !strings.Contains(actionDesc, "deterministic computation/recalculation") {
		t.Fatalf("expected browser_action description to discourage metadata-only use, got %q", actionDesc)
	}
}

func TestSandboxFileDescriptionsExpressReplaceVsDiscoveryBoundary(t *testing.T) {
	t.Parallel()

	cfg := SandboxConfig{}
	replaceDesc := NewSandboxFileReplace(cfg).Definition().Description
	if !strings.Contains(replaceDesc, "in-place patch/hotfix") || !strings.Contains(replaceDesc, "artifact deletion/cleanup") {
		t.Fatalf("expected replace_in_file description to mention in-place-only routing, got %q", replaceDesc)
	}

	readDesc := NewSandboxFileRead(cfg).Definition().Description
	if !strings.Contains(readDesc, "context windows") || !strings.Contains(readDesc, "memory_search/memory_get") {
		t.Fatalf("expected read_file description to mention repo-vs-memory boundary, got %q", readDesc)
	}
}

func TestSandboxExecuteCodeDescriptionExpressesDeterministicBoundary(t *testing.T) {
	t.Parallel()

	cfg := SandboxConfig{}
	desc := NewSandboxCodeExecute(cfg).Definition().Description
	if !strings.Contains(desc, "deterministic computation/recalculation/invariant checks") || !strings.Contains(desc, "Do not use for browser interaction or calendar querying") {
		t.Fatalf("expected execute_code description to express deterministic boundary, got %q", desc)
	}
}
