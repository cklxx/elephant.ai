package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"strconv"
	"strings"
	"time"

	"alex/internal/shared/utils"
	toolspolicy "alex/internal/infra/tools"
	"gopkg.in/yaml.v3"
)

func applyFile(cfg *RuntimeConfig, meta *Metadata, opts loadOptions) error {
	configPath := strings.TrimSpace(opts.configPath)
	if configPath == "" {
		configPath, _ = ResolveConfigPath(opts.envLookup, opts.homeDir)
	}
	if configPath == "" {
		return nil
	}

	data, err := opts.readFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	parsed, err := parseRuntimeConfigYAML(data)
	if err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	lookup := opts.envLookup
	if lookup == nil {
		lookup = DefaultEnvLookup
	}
	parsed = expandRuntimeFileConfigEnv(lookup, parsed)

	if parsed.APIKey != "" {
		cfg.APIKey = parsed.APIKey
		meta.sources["api_key"] = SourceFile
	}
	if parsed.ArkAPIKey != "" {
		cfg.ArkAPIKey = parsed.ArkAPIKey
		meta.sources["ark_api_key"] = SourceFile
	}
	if parsed.LLMProvider != "" {
		cfg.LLMProvider = parsed.LLMProvider
		meta.sources["llm_provider"] = SourceFile
	}
	if parsed.LLMModel != "" {
		cfg.LLMModel = parsed.LLMModel
		meta.sources["llm_model"] = SourceFile
	}
	if parsed.LLMVisionModel != "" {
		cfg.LLMVisionModel = parsed.LLMVisionModel
		meta.sources["llm_vision_model"] = SourceFile
	}
	if parsed.BaseURL != "" {
		cfg.BaseURL = parsed.BaseURL
		meta.sources["base_url"] = SourceFile
	}
	if parsed.ACPExecutorAddr != "" {
		cfg.ACPExecutorAddr = parsed.ACPExecutorAddr
		meta.sources["acp_executor_addr"] = SourceFile
	}
	if parsed.ACPExecutorCWD != "" {
		cfg.ACPExecutorCWD = parsed.ACPExecutorCWD
		meta.sources["acp_executor_cwd"] = SourceFile
	}
	if parsed.ACPExecutorMode != "" {
		cfg.ACPExecutorMode = parsed.ACPExecutorMode
		meta.sources["acp_executor_mode"] = SourceFile
	}
	if parsed.ACPExecutorAutoApprove != nil {
		cfg.ACPExecutorAutoApprove = *parsed.ACPExecutorAutoApprove
		meta.sources["acp_executor_auto_approve"] = SourceFile
	}
	if parsed.ACPExecutorMaxCLICalls != nil {
		cfg.ACPExecutorMaxCLICalls = *parsed.ACPExecutorMaxCLICalls
		meta.sources["acp_executor_max_cli_calls"] = SourceFile
	}
	if parsed.ACPExecutorMaxDuration != nil {
		cfg.ACPExecutorMaxDuration = *parsed.ACPExecutorMaxDuration
		meta.sources["acp_executor_max_duration_seconds"] = SourceFile
	}
	if parsed.ACPExecutorRequireManifest != nil {
		cfg.ACPExecutorRequireManifest = *parsed.ACPExecutorRequireManifest
		meta.sources["acp_executor_require_manifest"] = SourceFile
	}
	if parsed.TavilyAPIKey != "" {
		cfg.TavilyAPIKey = parsed.TavilyAPIKey
		meta.sources["tavily_api_key"] = SourceFile
	}
	if parsed.MoltbookAPIKey != "" {
		cfg.MoltbookAPIKey = parsed.MoltbookAPIKey
		meta.sources["moltbook_api_key"] = SourceFile
	}
	if parsed.MoltbookBaseURL != "" {
		cfg.MoltbookBaseURL = parsed.MoltbookBaseURL
		meta.sources["moltbook_base_url"] = SourceFile
	}
	if parsed.SeedreamTextEndpointID != "" {
		cfg.SeedreamTextEndpointID = parsed.SeedreamTextEndpointID
		meta.sources["seedream_text_endpoint_id"] = SourceFile
	}
	if parsed.SeedreamImageEndpointID != "" {
		cfg.SeedreamImageEndpointID = parsed.SeedreamImageEndpointID
		meta.sources["seedream_image_endpoint_id"] = SourceFile
	}
	if parsed.SeedreamTextModel != "" {
		cfg.SeedreamTextModel = parsed.SeedreamTextModel
		meta.sources["seedream_text_model"] = SourceFile
	}
	if parsed.SeedreamImageModel != "" {
		cfg.SeedreamImageModel = parsed.SeedreamImageModel
		meta.sources["seedream_image_model"] = SourceFile
	}
	if parsed.SeedreamVisionModel != "" {
		cfg.SeedreamVisionModel = parsed.SeedreamVisionModel
		meta.sources["seedream_vision_model"] = SourceFile
	}
	if parsed.SeedreamVideoModel != "" {
		cfg.SeedreamVideoModel = parsed.SeedreamVideoModel
		meta.sources["seedream_video_model"] = SourceFile
	}
	if parsed.Profile != "" {
		cfg.Profile = parsed.Profile
		meta.sources["profile"] = SourceFile
	}
	if parsed.Environment != "" {
		cfg.Environment = parsed.Environment
		meta.sources["environment"] = SourceFile
	}
	if parsed.Verbose != nil {
		cfg.Verbose = *parsed.Verbose
		meta.sources["verbose"] = SourceFile
	}
	if parsed.DisableTUI != nil {
		cfg.DisableTUI = *parsed.DisableTUI
		meta.sources["disable_tui"] = SourceFile
	}
	if parsed.FollowTranscript != nil {
		cfg.FollowTranscript = *parsed.FollowTranscript
		meta.sources["follow_transcript"] = SourceFile
	}
	if parsed.FollowStream != nil {
		cfg.FollowStream = *parsed.FollowStream
		meta.sources["follow_stream"] = SourceFile
	}
	if parsed.MaxIterations != nil {
		cfg.MaxIterations = *parsed.MaxIterations
		meta.sources["max_iterations"] = SourceFile
	}
	if parsed.MaxTokens != nil {
		cfg.MaxTokens = *parsed.MaxTokens
		meta.sources["max_tokens"] = SourceFile
	}
	if parsed.ToolMaxConcurrent != nil {
		cfg.ToolMaxConcurrent = *parsed.ToolMaxConcurrent
		meta.sources["tool_max_concurrent"] = SourceFile
	}
	if parsed.LLMCacheSize != nil {
		cfg.LLMCacheSize = *parsed.LLMCacheSize
		meta.sources["llm_cache_size"] = SourceFile
	}
	if parsed.LLMCacheTTLSeconds != nil {
		cfg.LLMCacheTTLSeconds = *parsed.LLMCacheTTLSeconds
		meta.sources["llm_cache_ttl_seconds"] = SourceFile
	}
	if parsed.LLMRequestTimeoutSeconds != nil && *parsed.LLMRequestTimeoutSeconds > 0 {
		cfg.LLMRequestTimeoutSeconds = *parsed.LLMRequestTimeoutSeconds
		meta.sources["llm_request_timeout_seconds"] = SourceFile
	}
	if parsed.UserRateLimitRPS != nil {
		cfg.UserRateLimitRPS = *parsed.UserRateLimitRPS
		meta.sources["user_rate_limit_rps"] = SourceFile
	}
	if parsed.UserRateLimitBurst != nil {
		cfg.UserRateLimitBurst = *parsed.UserRateLimitBurst
		meta.sources["user_rate_limit_burst"] = SourceFile
	}
	if parsed.Temperature != nil {
		cfg.Temperature = *parsed.Temperature
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceFile
	}
	if parsed.TopP != nil {
		cfg.TopP = *parsed.TopP
		meta.sources["top_p"] = SourceFile
	}
	if len(parsed.StopSequences) > 0 {
		cfg.StopSequences = append([]string(nil), parsed.StopSequences...)
		meta.sources["stop_sequences"] = SourceFile
	}
	if parsed.SessionDir != "" {
		cfg.SessionDir = parsed.SessionDir
		meta.sources["session_dir"] = SourceFile
	}
	if parsed.CostDir != "" {
		cfg.CostDir = parsed.CostDir
		meta.sources["cost_dir"] = SourceFile
	}
	if parsed.SessionStaleAfter != "" {
		seconds, err := parseDurationSeconds(parsed.SessionStaleAfter)
		if err != nil {
			return fmt.Errorf("parse session_stale_after: %w", err)
		}
		cfg.SessionStaleAfterSeconds = seconds
		meta.sources["session_stale_after_seconds"] = SourceFile
	}
	if parsed.AgentPreset != "" {
		cfg.AgentPreset = parsed.AgentPreset
		meta.sources["agent_preset"] = SourceFile
	}
	if parsed.ToolPreset != "" {
		cfg.ToolPreset = parsed.ToolPreset
		meta.sources["tool_preset"] = SourceFile
	}
	if parsed.Toolset != "" {
		cfg.Toolset = parsed.Toolset
		meta.sources["toolset"] = SourceFile
	}
	if parsed.Browser != nil {
		if connector := strings.TrimSpace(parsed.Browser.Connector); connector != "" {
			cfg.Browser.Connector = connector
			meta.sources["browser.connector"] = SourceFile
		}
		if cdpURL := strings.TrimSpace(parsed.Browser.CDPURL); cdpURL != "" {
			cfg.Browser.CDPURL = cdpURL
			meta.sources["browser.cdp_url"] = SourceFile
		}
		if chromePath := strings.TrimSpace(parsed.Browser.ChromePath); chromePath != "" {
			cfg.Browser.ChromePath = chromePath
			meta.sources["browser.chrome_path"] = SourceFile
		}
		if parsed.Browser.Headless != nil {
			cfg.Browser.Headless = *parsed.Browser.Headless
			meta.sources["browser.headless"] = SourceFile
		}
		if userDataDir := strings.TrimSpace(parsed.Browser.UserDataDir); userDataDir != "" {
			cfg.Browser.UserDataDir = userDataDir
			meta.sources["browser.user_data_dir"] = SourceFile
		}
		if parsed.Browser.TimeoutSeconds != nil && *parsed.Browser.TimeoutSeconds > 0 {
			cfg.Browser.TimeoutSeconds = *parsed.Browser.TimeoutSeconds
			meta.sources["browser.timeout_seconds"] = SourceFile
		}
		if bridgeListen := strings.TrimSpace(parsed.Browser.BridgeListen); bridgeListen != "" {
			cfg.Browser.BridgeListen = bridgeListen
			meta.sources["browser.bridge_listen_addr"] = SourceFile
		}
		if bridgeToken := strings.TrimSpace(parsed.Browser.BridgeToken); bridgeToken != "" {
			cfg.Browser.BridgeToken = bridgeToken
			meta.sources["browser.bridge_token"] = SourceFile
		}
	}
	if parsed.ToolPolicy != nil {
		applyToolPolicyFileConfig(cfg, meta, parsed.ToolPolicy)
	}
	if parsed.HTTPLimits != nil {
		applyHTTPLimitsFileConfig(cfg, meta, parsed.HTTPLimits)
	}
	if parsed.Proactive != nil {
		applyProactiveFileConfig(cfg, meta, parsed.Proactive)
	}
	if parsed.ExternalAgents != nil {
		if err := applyExternalAgentsFileConfig(cfg, meta, parsed.ExternalAgents); err != nil {
			return err
		}
	}

	var fileCfg FileConfig
	if err := yaml.Unmarshal(data, &fileCfg); err == nil {
		fileCfg = expandFileConfigEnv(lookup, fileCfg)
		if fileCfg.Agent != nil && utils.HasContent(fileCfg.Agent.SessionStaleAfter) {
			seconds, err := parseDurationSeconds(fileCfg.Agent.SessionStaleAfter)
			if err != nil {
				return fmt.Errorf("parse agent.session_stale_after: %w", err)
			}
			cfg.SessionStaleAfterSeconds = seconds
			meta.sources["session_stale_after_seconds"] = SourceFile
		}
	}

	return nil
}

type runtimeFile struct {
	Runtime *RuntimeFileConfig `yaml:"runtime"`
}

func parseRuntimeConfigYAML(data []byte) (RuntimeFileConfig, error) {
	var wrapped runtimeFile
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return RuntimeFileConfig{}, err
	}
	if wrapped.Runtime != nil {
		return *wrapped.Runtime, nil
	}

	var parsed RuntimeFileConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return RuntimeFileConfig{}, err
	}
	return parsed, nil
}

func expandRuntimeFileConfigEnv(lookup EnvLookup, parsed RuntimeFileConfig) RuntimeFileConfig {
	parsed.LLMProvider = expandEnvValue(lookup, parsed.LLMProvider)
	parsed.LLMModel = expandEnvValue(lookup, parsed.LLMModel)
	parsed.LLMVisionModel = expandEnvValue(lookup, parsed.LLMVisionModel)
	parsed.APIKey = expandEnvValue(lookup, parsed.APIKey)
	parsed.ArkAPIKey = expandEnvValue(lookup, parsed.ArkAPIKey)
	parsed.BaseURL = expandEnvValue(lookup, parsed.BaseURL)
	parsed.ACPExecutorAddr = expandEnvValue(lookup, parsed.ACPExecutorAddr)
	parsed.ACPExecutorCWD = expandEnvValue(lookup, parsed.ACPExecutorCWD)
	parsed.ACPExecutorMode = expandEnvValue(lookup, parsed.ACPExecutorMode)
	parsed.TavilyAPIKey = expandEnvValue(lookup, parsed.TavilyAPIKey)
	parsed.MoltbookAPIKey = expandEnvValue(lookup, parsed.MoltbookAPIKey)
	parsed.MoltbookBaseURL = expandEnvValue(lookup, parsed.MoltbookBaseURL)
	parsed.SeedreamTextEndpointID = expandEnvValue(lookup, parsed.SeedreamTextEndpointID)
	parsed.SeedreamImageEndpointID = expandEnvValue(lookup, parsed.SeedreamImageEndpointID)
	parsed.SeedreamTextModel = expandEnvValue(lookup, parsed.SeedreamTextModel)
	parsed.SeedreamImageModel = expandEnvValue(lookup, parsed.SeedreamImageModel)
	parsed.SeedreamVisionModel = expandEnvValue(lookup, parsed.SeedreamVisionModel)
	parsed.SeedreamVideoModel = expandEnvValue(lookup, parsed.SeedreamVideoModel)
	parsed.Profile = expandEnvValue(lookup, parsed.Profile)
	parsed.Environment = expandEnvValue(lookup, parsed.Environment)
	parsed.SessionDir = expandEnvValue(lookup, parsed.SessionDir)
	parsed.CostDir = expandEnvValue(lookup, parsed.CostDir)
	parsed.SessionStaleAfter = expandEnvValue(lookup, parsed.SessionStaleAfter)
	parsed.AgentPreset = expandEnvValue(lookup, parsed.AgentPreset)
	parsed.ToolPreset = expandEnvValue(lookup, parsed.ToolPreset)
	parsed.Toolset = expandEnvValue(lookup, parsed.Toolset)
	if parsed.Browser != nil {
		parsed.Browser.CDPURL = expandEnvValue(lookup, parsed.Browser.CDPURL)
		parsed.Browser.ChromePath = expandEnvValue(lookup, parsed.Browser.ChromePath)
		parsed.Browser.UserDataDir = expandEnvValue(lookup, parsed.Browser.UserDataDir)
	}
	if parsed.Proactive != nil {
		expandProactiveFileConfigEnv(lookup, parsed.Proactive)
	}
	if parsed.ExternalAgents != nil {
		expandExternalAgentsFileConfigEnv(lookup, parsed.ExternalAgents)
	}

	if len(parsed.StopSequences) > 0 {
		expanded := make([]string, 0, len(parsed.StopSequences))
		for _, seq := range parsed.StopSequences {
			expanded = append(expanded, expandEnvValue(lookup, seq))
		}
		parsed.StopSequences = expanded
	}

	return parsed
}

func parseDurationSeconds(value string) (int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		return seconds, nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	return int(parsed.Seconds()), nil
}

func parseDuration(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, nil
	}
	if seconds, err := strconv.Atoi(trimmed); err == nil {
		return time.Duration(seconds) * time.Second, nil
	}
	return time.ParseDuration(trimmed)
}

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

func expandExternalAgentsFileConfigEnv(lookup EnvLookup, external *ExternalAgentsFileConfig) {
	if external == nil {
		return
	}
	if external.ClaudeCode != nil {
		cc := external.ClaudeCode
		cc.Binary = expandEnvValue(lookup, cc.Binary)
		cc.DefaultModel = expandEnvValue(lookup, cc.DefaultModel)
		cc.DefaultMode = expandEnvValue(lookup, cc.DefaultMode)
		cc.Timeout = expandEnvValue(lookup, cc.Timeout)
		if len(cc.AutonomousAllowedTools) > 0 {
			tools := make([]string, 0, len(cc.AutonomousAllowedTools))
			for _, tool := range cc.AutonomousAllowedTools {
				tools = append(tools, expandEnvValue(lookup, tool))
			}
			cc.AutonomousAllowedTools = tools
		}
		if len(cc.PlanAllowedTools) > 0 {
			tools := make([]string, 0, len(cc.PlanAllowedTools))
			for _, tool := range cc.PlanAllowedTools {
				tools = append(tools, expandEnvValue(lookup, tool))
			}
			cc.PlanAllowedTools = tools
		}
		if len(cc.Env) > 0 {
			expanded := make(map[string]string, len(cc.Env))
			for key, value := range cc.Env {
				expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
			}
			cc.Env = expanded
		}
	}
	if external.Codex != nil {
		cx := external.Codex
		cx.Binary = expandEnvValue(lookup, cx.Binary)
		cx.DefaultModel = expandEnvValue(lookup, cx.DefaultModel)
		cx.ApprovalPolicy = expandEnvValue(lookup, cx.ApprovalPolicy)
		cx.Sandbox = expandEnvValue(lookup, cx.Sandbox)
		cx.PlanApprovalPolicy = expandEnvValue(lookup, cx.PlanApprovalPolicy)
		cx.PlanSandbox = expandEnvValue(lookup, cx.PlanSandbox)
		cx.Timeout = expandEnvValue(lookup, cx.Timeout)
		if len(cx.Env) > 0 {
			expanded := make(map[string]string, len(cx.Env))
			for key, value := range cx.Env {
				expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
			}
			cx.Env = expanded
		}
	}
	if external.Kimi != nil {
		km := external.Kimi
		km.Binary = expandEnvValue(lookup, km.Binary)
		km.DefaultModel = expandEnvValue(lookup, km.DefaultModel)
		km.ApprovalPolicy = expandEnvValue(lookup, km.ApprovalPolicy)
		km.Sandbox = expandEnvValue(lookup, km.Sandbox)
		km.PlanApprovalPolicy = expandEnvValue(lookup, km.PlanApprovalPolicy)
		km.PlanSandbox = expandEnvValue(lookup, km.PlanSandbox)
		km.Timeout = expandEnvValue(lookup, km.Timeout)
		if len(km.Env) > 0 {
			expanded := make(map[string]string, len(km.Env))
			for key, value := range km.Env {
				expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
			}
			km.Env = expanded
		}
	}
	if len(external.Teams) > 0 {
		for i := range external.Teams {
			team := &external.Teams[i]
			team.Name = expandEnvValue(lookup, team.Name)
			team.Description = expandEnvValue(lookup, team.Description)
			if len(team.Roles) > 0 {
				for j := range team.Roles {
					role := &team.Roles[j]
					role.Name = expandEnvValue(lookup, role.Name)
					role.AgentType = expandEnvValue(lookup, role.AgentType)
					role.PromptTemplate = expandEnvValue(lookup, role.PromptTemplate)
					role.ExecutionMode = expandEnvValue(lookup, role.ExecutionMode)
					role.AutonomyLevel = expandEnvValue(lookup, role.AutonomyLevel)
					role.WorkspaceMode = expandEnvValue(lookup, role.WorkspaceMode)
					if len(role.Config) > 0 {
						expanded := make(map[string]string, len(role.Config))
						for key, value := range role.Config {
							expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
						}
						role.Config = expanded
					}
				}
			}
			if len(team.Stages) > 0 {
				for j := range team.Stages {
					stage := &team.Stages[j]
					stage.Name = expandEnvValue(lookup, stage.Name)
					if len(stage.Roles) > 0 {
						roles := make([]string, 0, len(stage.Roles))
						for _, roleName := range stage.Roles {
							roles = append(roles, expandEnvValue(lookup, roleName))
						}
						stage.Roles = roles
					}
				}
			}
		}
	}
}

func convertTeamFileConfigs(raw []TeamFileConfig) []TeamConfig {
	if len(raw) == 0 {
		return nil
	}
	teams := make([]TeamConfig, 0, len(raw))
	for _, team := range raw {
		roles := make([]TeamRoleConfig, 0, len(team.Roles))
		for _, role := range team.Roles {
			inheritContext := false
			if role.InheritContext != nil {
				inheritContext = *role.InheritContext
			}
			roles = append(roles, TeamRoleConfig{
				Name:           role.Name,
				AgentType:      role.AgentType,
				PromptTemplate: role.PromptTemplate,
				ExecutionMode:  role.ExecutionMode,
				AutonomyLevel:  role.AutonomyLevel,
				WorkspaceMode:  role.WorkspaceMode,
				Config:         cloneStringMap(role.Config),
				InheritContext: inheritContext,
			})
		}
		stages := make([]TeamStageConfig, 0, len(team.Stages))
		for _, stage := range team.Stages {
			stages = append(stages, TeamStageConfig{
				Name:  stage.Name,
				Roles: append([]string(nil), stage.Roles...),
			})
		}
		teams = append(teams, TeamConfig{
			Name:        team.Name,
			Description: team.Description,
			Roles:       roles,
			Stages:      stages,
		})
	}
	return teams
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	return maps.Clone(in)
}

func cloneDurationMap(in map[string]time.Duration) map[string]time.Duration {
	if len(in) == 0 {
		return nil
	}
	return maps.Clone(in)
}
