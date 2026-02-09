package toolregistry

import (
	"context"
	"slices"
	"testing"

	ports "alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	toolspolicy "alex/internal/infra/tools"
)

func TestPolicyAwareRegistry_DeniesByName(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
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
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
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

// policyStubTool is a minimal tool stub for policy tests with configurable metadata.
type policyStubTool struct {
	def  ports.ToolDefinition
	meta ports.ToolMetadata
}

func (t *policyStubTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
}
func (t *policyStubTool) Definition() ports.ToolDefinition { return t.def }
func (t *policyStubTool) Metadata() ports.ToolMetadata     { return t.meta }

var _ tools.ToolExecutor = (*policyStubTool)(nil)

func TestPolicyAwareRegistry_SafetyLevelPropagation(t *testing.T) {
	// Verify that isAllowed populates SafetyLevel from tool metadata so that
	// rules matching on safety_levels can fire.
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	// Register an L4 tool
	l4Tool := &policyStubTool{
		def:  ports.ToolDefinition{Name: "test_l4_tool", Description: "test"},
		meta: ports.ToolMetadata{Name: "test_l4_tool", SafetyLevel: ports.SafetyLevelIrreversible},
	}
	if err := registry.Register(l4Tool); err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	// Create a policy that denies L4 tools
	disabled := false
	cfg := toolspolicy.DefaultToolPolicyConfig()
	cfg.Rules = []toolspolicy.PolicyRule{
		{
			Name:    "deny-l4",
			Match:   toolspolicy.PolicySelector{SafetyLevels: []int{ports.SafetyLevelIrreversible}},
			Enabled: &disabled,
		},
	}
	policy := toolspolicy.NewToolPolicy(cfg)
	wrapped := registry.WithPolicy(policy, "cli")

	// L4 tool should be denied
	if _, err := wrapped.Get("test_l4_tool"); err == nil {
		t.Fatal("expected L4 tool to be denied by safety level policy")
	}

	// L4 tool should be absent from List
	defs := wrapped.List()
	if slices.ContainsFunc(defs, func(def ports.ToolDefinition) bool { return def.Name == "test_l4_tool" }) {
		t.Fatal("expected L4 tool to be filtered from List by safety level policy")
	}

	// Other tools (e.g. file_read at L1) should still be allowed
	if _, err := wrapped.Get("file_read"); err != nil {
		t.Fatalf("expected file_read (L1) to be allowed: %v", err)
	}
}
