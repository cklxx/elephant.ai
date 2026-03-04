package teamruntime

import (
	"testing"

	"alex/internal/infra/coding"
)

func TestSelectRoleBinding_TargetCLITakesPrecedence(t *testing.T) {
	caps := []coding.DiscoveredCLICapability{
		{ID: "codex", Path: "/x/codex", Executable: true, AdapterSupport: true, AgentType: "codex"},
		{ID: "claude_code", Path: "/x/claude", Executable: true, AdapterSupport: true, AgentType: "claude_code"},
	}
	binding := SelectRoleBinding("executor", "planning", "codex", caps, "/tmp/executor.log")
	if binding.SelectedCLI != "codex" {
		t.Fatalf("expected target cli codex, got %q", binding.SelectedCLI)
	}
	if len(binding.FallbackCLIs) == 0 || binding.FallbackCLIs[0] != "claude_code" {
		t.Fatalf("unexpected fallback chain: %v", binding.FallbackCLIs)
	}
}

func TestSelectRoleBinding_ProfileFallback(t *testing.T) {
	caps := []coding.DiscoveredCLICapability{
		{ID: "gemini", Path: "/x/gemini", Executable: true},
		{ID: "codex", Path: "/x/codex", Executable: true, AdapterSupport: true, AgentType: "codex"},
	}
	binding := SelectRoleBinding("long", "long_context", "", caps, "")
	if binding.SelectedCLI != "gemini" {
		t.Fatalf("expected long_context to prefer gemini, got %q", binding.SelectedCLI)
	}
	if binding.SelectedAgentType != "generic_cli" {
		t.Fatalf("expected generic_cli for non-adapter gemini, got %q", binding.SelectedAgentType)
	}
}
