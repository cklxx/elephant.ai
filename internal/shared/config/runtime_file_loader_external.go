package config

import (
	"fmt"
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
	if external.ClaudeCode != nil {
		cc := external.ClaudeCode
		if cc.Enabled != nil {
			cfg.ExternalAgents.ClaudeCode.Enabled = *cc.Enabled
			meta.sources["external_agents.claude_code.enabled"] = SourceFile
		}
		if utils.HasContent(cc.Binary) {
			cfg.ExternalAgents.ClaudeCode.Binary = cc.Binary
			meta.sources["external_agents.claude_code.binary"] = SourceFile
		}
		if utils.HasContent(cc.DefaultModel) {
			cfg.ExternalAgents.ClaudeCode.DefaultModel = cc.DefaultModel
			meta.sources["external_agents.claude_code.default_model"] = SourceFile
		}
		if utils.HasContent(cc.DefaultMode) {
			cfg.ExternalAgents.ClaudeCode.DefaultMode = cc.DefaultMode
			meta.sources["external_agents.claude_code.default_mode"] = SourceFile
		}
		if len(cc.AutonomousAllowedTools) > 0 {
			cfg.ExternalAgents.ClaudeCode.AutonomousAllowedTools = append([]string(nil), cc.AutonomousAllowedTools...)
			meta.sources["external_agents.claude_code.autonomous_allowed_tools"] = SourceFile
		}
		if len(cc.PlanAllowedTools) > 0 {
			cfg.ExternalAgents.ClaudeCode.PlanAllowedTools = append([]string(nil), cc.PlanAllowedTools...)
			meta.sources["external_agents.claude_code.plan_allowed_tools"] = SourceFile
		}
		if cc.MaxBudgetUSD != nil {
			cfg.ExternalAgents.ClaudeCode.MaxBudgetUSD = *cc.MaxBudgetUSD
			meta.sources["external_agents.claude_code.max_budget_usd"] = SourceFile
		}
		if cc.MaxTurns != nil {
			cfg.ExternalAgents.ClaudeCode.MaxTurns = *cc.MaxTurns
			meta.sources["external_agents.claude_code.max_turns"] = SourceFile
		}
		if utils.HasContent(cc.Timeout) {
			timeout, err := parseDuration(cc.Timeout)
			if err != nil {
				return fmt.Errorf("parse external_agents.claude_code.timeout: %w", err)
			}
			cfg.ExternalAgents.ClaudeCode.Timeout = timeout
			meta.sources["external_agents.claude_code.timeout"] = SourceFile
		}
		if cc.ResumeEnabled != nil {
			cfg.ExternalAgents.ClaudeCode.ResumeEnabled = *cc.ResumeEnabled
			meta.sources["external_agents.claude_code.resume_enabled"] = SourceFile
		}
		if env := cloneStringMap(cc.Env); env != nil {
			cfg.ExternalAgents.ClaudeCode.Env = env
			meta.sources["external_agents.claude_code.env"] = SourceFile
		}
	}
	if external.Codex != nil {
		cx := external.Codex
		if cx.Enabled != nil {
			cfg.ExternalAgents.Codex.Enabled = *cx.Enabled
			meta.sources["external_agents.codex.enabled"] = SourceFile
		}
		if utils.HasContent(cx.Binary) {
			cfg.ExternalAgents.Codex.Binary = cx.Binary
			meta.sources["external_agents.codex.binary"] = SourceFile
		}
		if utils.HasContent(cx.DefaultModel) {
			cfg.ExternalAgents.Codex.DefaultModel = cx.DefaultModel
			meta.sources["external_agents.codex.default_model"] = SourceFile
		}
		if utils.HasContent(cx.ApprovalPolicy) {
			cfg.ExternalAgents.Codex.ApprovalPolicy = cx.ApprovalPolicy
			meta.sources["external_agents.codex.approval_policy"] = SourceFile
		}
		if utils.HasContent(cx.Sandbox) {
			cfg.ExternalAgents.Codex.Sandbox = cx.Sandbox
			meta.sources["external_agents.codex.sandbox"] = SourceFile
		}
		if utils.HasContent(cx.PlanApprovalPolicy) {
			cfg.ExternalAgents.Codex.PlanApprovalPolicy = cx.PlanApprovalPolicy
			meta.sources["external_agents.codex.plan_approval_policy"] = SourceFile
		}
		if utils.HasContent(cx.PlanSandbox) {
			cfg.ExternalAgents.Codex.PlanSandbox = cx.PlanSandbox
			meta.sources["external_agents.codex.plan_sandbox"] = SourceFile
		}
		if utils.HasContent(cx.Timeout) {
			timeout, err := parseDuration(cx.Timeout)
			if err != nil {
				return fmt.Errorf("parse external_agents.codex.timeout: %w", err)
			}
			cfg.ExternalAgents.Codex.Timeout = timeout
			meta.sources["external_agents.codex.timeout"] = SourceFile
		}
		if cx.ResumeEnabled != nil {
			cfg.ExternalAgents.Codex.ResumeEnabled = *cx.ResumeEnabled
			meta.sources["external_agents.codex.resume_enabled"] = SourceFile
		}
		if env := cloneStringMap(cx.Env); env != nil {
			cfg.ExternalAgents.Codex.Env = env
			meta.sources["external_agents.codex.env"] = SourceFile
		}
	}
	if external.Kimi != nil {
		km := external.Kimi
		if km.Enabled != nil {
			cfg.ExternalAgents.Kimi.Enabled = *km.Enabled
			meta.sources["external_agents.kimi.enabled"] = SourceFile
		}
		if strings.TrimSpace(km.Binary) != "" {
			cfg.ExternalAgents.Kimi.Binary = km.Binary
			meta.sources["external_agents.kimi.binary"] = SourceFile
		}
		if strings.TrimSpace(km.DefaultModel) != "" {
			cfg.ExternalAgents.Kimi.DefaultModel = km.DefaultModel
			meta.sources["external_agents.kimi.default_model"] = SourceFile
		}
		if strings.TrimSpace(km.ApprovalPolicy) != "" {
			cfg.ExternalAgents.Kimi.ApprovalPolicy = km.ApprovalPolicy
			meta.sources["external_agents.kimi.approval_policy"] = SourceFile
		}
		if strings.TrimSpace(km.Sandbox) != "" {
			cfg.ExternalAgents.Kimi.Sandbox = km.Sandbox
			meta.sources["external_agents.kimi.sandbox"] = SourceFile
		}
		if strings.TrimSpace(km.PlanApprovalPolicy) != "" {
			cfg.ExternalAgents.Kimi.PlanApprovalPolicy = km.PlanApprovalPolicy
			meta.sources["external_agents.kimi.plan_approval_policy"] = SourceFile
		}
		if strings.TrimSpace(km.PlanSandbox) != "" {
			cfg.ExternalAgents.Kimi.PlanSandbox = km.PlanSandbox
			meta.sources["external_agents.kimi.plan_sandbox"] = SourceFile
		}
		if strings.TrimSpace(km.Timeout) != "" {
			timeout, err := parseDuration(km.Timeout)
			if err != nil {
				return fmt.Errorf("parse external_agents.kimi.timeout: %w", err)
			}
			cfg.ExternalAgents.Kimi.Timeout = timeout
			meta.sources["external_agents.kimi.timeout"] = SourceFile
		}
		if km.ResumeEnabled != nil {
			cfg.ExternalAgents.Kimi.ResumeEnabled = *km.ResumeEnabled
			meta.sources["external_agents.kimi.resume_enabled"] = SourceFile
		}
		if env := cloneStringMap(km.Env); env != nil {
			cfg.ExternalAgents.Kimi.Env = env
			meta.sources["external_agents.kimi.env"] = SourceFile
		}
	}
	if len(external.Teams) > 0 {
		cfg.ExternalAgents.Teams = convertTeamFileConfigs(external.Teams)
		meta.sources["external_agents.teams"] = SourceFile
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
