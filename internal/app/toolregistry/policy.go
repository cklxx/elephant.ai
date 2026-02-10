package toolregistry

import (
	"fmt"
	"strings"

	ports "alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	toolspolicy "alex/internal/infra/tools"
)

// WithPolicy returns a registry wrapper that enforces tool policy rules.
func (r *Registry) WithPolicy(policy toolspolicy.ToolPolicy, channel string) tools.ToolRegistry {
	if policy == nil {
		return r
	}
	return &policyAwareRegistry{parent: r, policy: policy, channel: channel}
}

// WithPolicy returns a registry wrapper that enforces tool policy rules.
func (f *filteredRegistry) WithPolicy(policy toolspolicy.ToolPolicy, channel string) tools.ToolRegistry {
	if policy == nil {
		return f
	}
	return &policyAwareRegistry{parent: f, policy: policy, channel: channel}
}

type policyAwareRegistry struct {
	parent  tools.ToolRegistry
	policy  toolspolicy.ToolPolicy
	channel string
}

// WithPolicy replaces the policy wrapper with a new policy/channel.
func (p *policyAwareRegistry) WithPolicy(policy toolspolicy.ToolPolicy, channel string) tools.ToolRegistry {
	if policy == nil {
		return p.parent
	}
	return &policyAwareRegistry{parent: p.parent, policy: policy, channel: channel}
}

// WithoutSubagent returns a policy-aware registry that excludes the subagent tool.
func (p *policyAwareRegistry) WithoutSubagent() tools.ToolRegistry {
	type registryWithFilter interface {
		WithoutSubagent() tools.ToolRegistry
	}
	if filtered, ok := p.parent.(registryWithFilter); ok {
		return &policyAwareRegistry{parent: filtered.WithoutSubagent(), policy: p.policy, channel: p.channel}
	}
	return p
}

func (p *policyAwareRegistry) Get(name string) (tools.ToolExecutor, error) {
	tool, err := p.parent.Get(name)
	if err != nil {
		return nil, err
	}
	if !p.isAllowed(tool) {
		return nil, fmt.Errorf("tool denied by policy: %s", name)
	}
	return tool, nil
}

func (p *policyAwareRegistry) List() []ports.ToolDefinition {
	defs := p.parent.List()
	if p.policy == nil {
		return defs
	}
	filtered := make([]ports.ToolDefinition, 0, len(defs))
	for _, def := range defs {
		tool, err := p.parent.Get(def.Name)
		if err != nil {
			continue
		}
		if p.isAllowed(tool) {
			filtered = append(filtered, def)
		}
	}
	return filtered
}

func (p *policyAwareRegistry) Register(tool tools.ToolExecutor) error {
	return p.parent.Register(tool)
}

func (p *policyAwareRegistry) Unregister(name string) error {
	return p.parent.Unregister(name)
}

func (p *policyAwareRegistry) isAllowed(tool tools.ToolExecutor) bool {
	if p.policy == nil || tool == nil {
		return true
	}
	meta := tool.Metadata()
	name := meta.Name
	if name == "" {
		name = tool.Definition().Name
	}
	ctx := toolspolicy.ToolCallContext{
		ToolName:    name,
		Category:    meta.Category,
		Tags:        meta.Tags,
		Dangerous:   meta.Dangerous,
		Channel:     strings.TrimSpace(p.channel),
		SafetyLevel: meta.EffectiveSafetyLevel(),
	}
	resolved := p.policy.Resolve(ctx)
	if resolved.Enabled {
		return true
	}
	return strings.EqualFold(resolved.EnforcementMode, "warn_allow")
}

var _ tools.ToolRegistry = (*policyAwareRegistry)(nil)
