package toolregistry

import (
	"slices"
	"testing"

	ports "alex/internal/agent/ports"
	toolspolicy "alex/internal/tools"
)

func TestPolicyAwareRegistry_DeniesByName(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryService: newTestMemoryService()})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	disabled := false
	cfg := toolspolicy.DefaultToolPolicyConfig()
	cfg.Rules = []toolspolicy.PolicyRule{
		{
			Name:    "deny-file-read",
			Match:   toolspolicy.PolicySelector{Tools: []string{"file_read"}},
			Enabled: &disabled,
		},
	}
	policy := toolspolicy.NewToolPolicy(cfg)
	wrapped := registry.WithPolicy(policy, "cli")

	if _, err := wrapped.Get("file_read"); err == nil {
		t.Fatal("expected file_read to be denied by policy")
	}

	defs := wrapped.List()
	if slices.ContainsFunc(defs, func(def ports.ToolDefinition) bool { return def.Name == "file_read" }) {
		t.Fatal("expected file_read to be filtered from List")
	}
}

func TestPolicyAwareRegistry_ChannelMatch(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryService: newTestMemoryService()})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	disabled := false
	cfg := toolspolicy.DefaultToolPolicyConfig()
	cfg.Rules = []toolspolicy.PolicyRule{
		{
			Name:    "deny-web-file-read",
			Match:   toolspolicy.PolicySelector{Tools: []string{"file_read"}, Channels: []string{"web"}},
			Enabled: &disabled,
		},
	}
	policy := toolspolicy.NewToolPolicy(cfg)

	webRegistry := registry.WithPolicy(policy, "web")
	if _, err := webRegistry.Get("file_read"); err == nil {
		t.Fatal("expected file_read to be denied on web channel")
	}

	cliRegistry := registry.WithPolicy(policy, "cli")
	if _, err := cliRegistry.Get("file_read"); err != nil {
		t.Fatalf("expected file_read to be allowed on cli channel: %v", err)
	}
}
