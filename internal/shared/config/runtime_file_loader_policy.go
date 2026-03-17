package config

import (
	"strings"

	toolspolicy "alex/internal/infra/tools"
	"alex/internal/shared/utils"
)

func applyExternalAgentsFileConfig(cfg *RuntimeConfig, meta *Metadata, external *ExternalAgentsFileConfig) error {
	if external == nil {
		return nil
	}
	if external.MaxParallelAgents != nil {
		cfg.ExternalAgents.MaxParallelAgents = *external.MaxParallelAgents
		meta.sources["external_agents.max_parallel_agents"] = SourceFile
	}
	return nil
}

func applyToolPolicyFileConfig(cfg *RuntimeConfig, meta *Metadata, policy *ToolPolicyFileConfig) {
	if policy == nil {
		return
	}
	if utils.HasContent(policy.EnforcementMode) {
		cfg.ToolPolicy.EnforcementMode = strings.TrimSpace(policy.EnforcementMode)
		meta.sources["tool_policy.enforcement_mode"] = SourceFile
	}
	if policy.Timeout != nil {
		if policy.Timeout.Default != nil {
			cfg.ToolPolicy.Timeout.Default = *policy.Timeout.Default
			meta.sources["tool_policy.timeout.default"] = SourceFile
		}
		if policy.Timeout.PerTool != nil {
			cfg.ToolPolicy.Timeout.PerTool = cloneDurationMap(policy.Timeout.PerTool)
			meta.sources["tool_policy.timeout.per_tool"] = SourceFile
		}
	}
	if policy.Retry != nil {
		if policy.Retry.MaxRetries != nil {
			cfg.ToolPolicy.Retry.MaxRetries = *policy.Retry.MaxRetries
			meta.sources["tool_policy.retry.max_retries"] = SourceFile
		}
		if policy.Retry.InitialBackoff != nil {
			cfg.ToolPolicy.Retry.InitialBackoff = *policy.Retry.InitialBackoff
			meta.sources["tool_policy.retry.initial_backoff"] = SourceFile
		}
		if policy.Retry.MaxBackoff != nil {
			cfg.ToolPolicy.Retry.MaxBackoff = *policy.Retry.MaxBackoff
			meta.sources["tool_policy.retry.max_backoff"] = SourceFile
		}
		if policy.Retry.BackoffFactor != nil {
			cfg.ToolPolicy.Retry.BackoffFactor = *policy.Retry.BackoffFactor
			meta.sources["tool_policy.retry.backoff_factor"] = SourceFile
		}
	}
	if policy.Rules != nil {
		cfg.ToolPolicy.Rules = append([]toolspolicy.PolicyRule(nil), policy.Rules...)
		meta.sources["tool_policy.rules"] = SourceFile
	}
}
