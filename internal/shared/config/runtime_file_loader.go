package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

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
	if parsed.LLMSmallProvider != "" {
		cfg.LLMSmallProvider = parsed.LLMSmallProvider
		meta.sources["llm_small_provider"] = SourceFile
	}
	if parsed.LLMSmallModel != "" {
		cfg.LLMSmallModel = parsed.LLMSmallModel
		meta.sources["llm_small_model"] = SourceFile
	}
	if parsed.LLMVisionModel != "" {
		cfg.LLMVisionModel = parsed.LLMVisionModel
		meta.sources["llm_vision_model"] = SourceFile
	}
	if parsed.BaseURL != "" {
		cfg.BaseURL = parsed.BaseURL
		meta.sources["base_url"] = SourceFile
	}
	if parsed.SandboxBaseURL != "" {
		cfg.SandboxBaseURL = parsed.SandboxBaseURL
		meta.sources["sandbox_base_url"] = SourceFile
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
		if fileCfg.Agent != nil && strings.TrimSpace(fileCfg.Agent.SessionStaleAfter) != "" {
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
	parsed.LLMSmallProvider = expandEnvValue(lookup, parsed.LLMSmallProvider)
	parsed.LLMSmallModel = expandEnvValue(lookup, parsed.LLMSmallModel)
	parsed.LLMVisionModel = expandEnvValue(lookup, parsed.LLMVisionModel)
	parsed.APIKey = expandEnvValue(lookup, parsed.APIKey)
	parsed.ArkAPIKey = expandEnvValue(lookup, parsed.ArkAPIKey)
	parsed.BaseURL = expandEnvValue(lookup, parsed.BaseURL)
	parsed.SandboxBaseURL = expandEnvValue(lookup, parsed.SandboxBaseURL)
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
	if external.ClaudeCode != nil {
		cc := external.ClaudeCode
		if cc.Enabled != nil {
			cfg.ExternalAgents.ClaudeCode.Enabled = *cc.Enabled
			meta.sources["external_agents.claude_code.enabled"] = SourceFile
		}
		if strings.TrimSpace(cc.Binary) != "" {
			cfg.ExternalAgents.ClaudeCode.Binary = cc.Binary
			meta.sources["external_agents.claude_code.binary"] = SourceFile
		}
		if strings.TrimSpace(cc.DefaultModel) != "" {
			cfg.ExternalAgents.ClaudeCode.DefaultModel = cc.DefaultModel
			meta.sources["external_agents.claude_code.default_model"] = SourceFile
		}
		if strings.TrimSpace(cc.DefaultMode) != "" {
			cfg.ExternalAgents.ClaudeCode.DefaultMode = cc.DefaultMode
			meta.sources["external_agents.claude_code.default_mode"] = SourceFile
		}
		if len(cc.AutonomousAllowedTools) > 0 {
			cfg.ExternalAgents.ClaudeCode.AutonomousAllowedTools = append([]string(nil), cc.AutonomousAllowedTools...)
			meta.sources["external_agents.claude_code.autonomous_allowed_tools"] = SourceFile
		}
		if cc.MaxBudgetUSD != nil {
			cfg.ExternalAgents.ClaudeCode.MaxBudgetUSD = *cc.MaxBudgetUSD
			meta.sources["external_agents.claude_code.max_budget_usd"] = SourceFile
		}
		if cc.MaxTurns != nil {
			cfg.ExternalAgents.ClaudeCode.MaxTurns = *cc.MaxTurns
			meta.sources["external_agents.claude_code.max_turns"] = SourceFile
		}
		if strings.TrimSpace(cc.Timeout) != "" {
			timeout, err := parseDuration(cc.Timeout)
			if err != nil {
				return fmt.Errorf("parse external_agents.claude_code.timeout: %w", err)
			}
			cfg.ExternalAgents.ClaudeCode.Timeout = timeout
			meta.sources["external_agents.claude_code.timeout"] = SourceFile
		}
		if len(cc.Env) > 0 {
			cfg.ExternalAgents.ClaudeCode.Env = cloneStringMap(cc.Env)
			meta.sources["external_agents.claude_code.env"] = SourceFile
		}
	}
	if external.Codex != nil {
		cx := external.Codex
		if cx.Enabled != nil {
			cfg.ExternalAgents.Codex.Enabled = *cx.Enabled
			meta.sources["external_agents.codex.enabled"] = SourceFile
		}
		if strings.TrimSpace(cx.Binary) != "" {
			cfg.ExternalAgents.Codex.Binary = cx.Binary
			meta.sources["external_agents.codex.binary"] = SourceFile
		}
		if strings.TrimSpace(cx.DefaultModel) != "" {
			cfg.ExternalAgents.Codex.DefaultModel = cx.DefaultModel
			meta.sources["external_agents.codex.default_model"] = SourceFile
		}
		if strings.TrimSpace(cx.ApprovalPolicy) != "" {
			cfg.ExternalAgents.Codex.ApprovalPolicy = cx.ApprovalPolicy
			meta.sources["external_agents.codex.approval_policy"] = SourceFile
		}
		if strings.TrimSpace(cx.Sandbox) != "" {
			cfg.ExternalAgents.Codex.Sandbox = cx.Sandbox
			meta.sources["external_agents.codex.sandbox"] = SourceFile
		}
		if strings.TrimSpace(cx.Timeout) != "" {
			timeout, err := parseDuration(cx.Timeout)
			if err != nil {
				return fmt.Errorf("parse external_agents.codex.timeout: %w", err)
			}
			cfg.ExternalAgents.Codex.Timeout = timeout
			meta.sources["external_agents.codex.timeout"] = SourceFile
		}
		if len(cx.Env) > 0 {
			cfg.ExternalAgents.Codex.Env = cloneStringMap(cx.Env)
			meta.sources["external_agents.codex.env"] = SourceFile
		}
	}
	return nil
}

func applyToolPolicyFileConfig(cfg *RuntimeConfig, meta *Metadata, policy *ToolPolicyFileConfig) {
	if policy == nil {
		return
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
		cx.Timeout = expandEnvValue(lookup, cx.Timeout)
		if len(cx.Env) > 0 {
			expanded := make(map[string]string, len(cx.Env))
			for key, value := range cx.Env {
				expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
			}
			cx.Env = expanded
		}
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneDurationMap(in map[string]time.Duration) map[string]time.Duration {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]time.Duration, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
