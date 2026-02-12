package config

import (
	"os"
	"strings"
	"time"

	toolspolicy "alex/internal/infra/tools"
)

func Load(opts ...Option) (RuntimeConfig, Metadata, error) {
	options := loadOptions{
		envLookup: DefaultEnvLookup,
		readFile:  os.ReadFile,
		homeDir:   os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(&options)
	}

	meta := Metadata{sources: map[string]ValueSource{}, loadedAt: time.Now()}

	cfg := RuntimeConfig{
		LLMProvider:                DefaultLLMProvider,
		LLMModel:                   DefaultLLMModel,
		LLMSmallProvider:           DefaultLLMProvider,
		LLMSmallModel:              DefaultLLMModel,
		BaseURL:                    DefaultLLMBaseURL,
		SandboxBaseURL:             "http://localhost:18086",
		ACPExecutorAddr:            defaultACPExecutorAddr(options.envLookup),
		ACPExecutorCWD:             defaultACPExecutorCWD(),
		ACPExecutorMode:            "sandbox",
		ACPExecutorAutoApprove:     true,
		ACPExecutorMaxCLICalls:     12,
		ACPExecutorMaxDuration:     900,
		ACPExecutorRequireManifest: true,
		SeedreamTextModel:          DefaultSeedreamTextModel,
		SeedreamImageModel:         DefaultSeedreamImageModel,
		SeedreamVisionModel:        DefaultSeedreamVisionModel,
		SeedreamVideoModel:         DefaultSeedreamVideoModel,
		Profile:                    DefaultRuntimeProfile,
		Environment:                "development",
		FollowTranscript:           true,
		FollowStream:               true,
		MaxIterations:              150,
		MaxTokens:                  DefaultMaxTokens,
		ToolMaxConcurrent:          DefaultToolMaxConcurrent,
		LLMCacheSize:               DefaultLLMCacheSize,
		LLMCacheTTLSeconds:         int(DefaultLLMCacheTTL.Seconds()),
		UserRateLimitRPS:           1.0,
		UserRateLimitBurst:         3,
		Temperature:                0.7,
		TopP:                       1.0,
		SessionDir:                 "~/.alex/sessions",
		CostDir:                    "~/.alex/costs",
		SessionStaleAfterSeconds:   int((48 * time.Hour).Seconds()),
		Toolset:                    "default",
		Browser: BrowserConfig{
			Connector:    "cdp",
			BridgeListen: "127.0.0.1:17333",
		},
		HTTPLimits:     DefaultHTTPLimitsConfig(),
		ToolPolicy:     toolspolicy.DefaultToolPolicyConfigWithRules(),
		Proactive:      DefaultProactiveConfig(),
		ExternalAgents: DefaultExternalAgentsConfig(),
	}

	// Helper to set provenance only when a value actually changes precedence.
	setSource := func(field string, source ValueSource) {
		meta.sources[field] = source
	}

	// Load from config file if present.
	if err := applyFile(&cfg, &meta, options); err != nil {
		return RuntimeConfig{}, Metadata{}, err
	}

	// Apply environment overrides next.
	if err := applyEnv(&cfg, &meta, options); err != nil {
		return RuntimeConfig{}, Metadata{}, err
	}

	cfgBeforeOverrides := cfg
	metaSourcesBeforeOverrides := cloneMetadataSources(meta.sources)

	// Apply caller overrides last.
	applyOverrides(&cfg, &meta, options.overrides)

	normalizeRuntimeConfig(&cfg)
	autoEnableExternalAgents(&cfg, &meta)
	cliCreds := CLICredentials{}
	if shouldLoadCLICredentials(cfg) {
		cliCreds = LoadCLICredentials(
			WithEnv(options.envLookup),
			WithFileReader(options.readFile),
			WithHomeDir(options.homeDir),
		)
	}
	resolveAutoProvider(&cfg, &meta, options.envLookup, cliCreds)
	resolveProviderCredentials(&cfg, &meta, options.envLookup, cliCreds)
	// If API key remains unset, default to mock provider (unless keyless providers).
	providerLower := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	if cfg.APIKey == "" && ProviderRequiresAPIKey(providerLower) && cfg.Profile != RuntimeProfileProduction {
		cfg.LLMProvider = "mock"
		if cfg.LLMSmallProvider != "mock" {
			cfg.LLMSmallProvider = "mock"
			setSource("llm_small_provider", SourceDefault)
		}
		if cfg.LLMSmallModel != "mock" {
			cfg.LLMSmallModel = "mock"
			setSource("llm_small_model", SourceDefault)
		}
		setSource("llm_provider", SourceDefault)
	}

	// Enforce an atomic provider/model/auth/base_url profile so downstream
	// components can consume it without provider-specific mismatch handling.
	if _, err := ResolveLLMProfile(cfg); err != nil {
		if attemptRepairManagedBaseURLOverrideMismatch(
			&cfg,
			&meta,
			cfgBeforeOverrides,
			metaSourcesBeforeOverrides,
		) {
			if _, retryErr := ResolveLLMProfile(cfg); retryErr == nil {
				return cfg, meta, nil
			}
		}
		return RuntimeConfig{}, Metadata{}, err
	}

	return cfg, meta, nil
}

func cloneMetadataSources(src map[string]ValueSource) map[string]ValueSource {
	if len(src) == 0 {
		return map[string]ValueSource{}
	}
	dst := make(map[string]ValueSource, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func attemptRepairManagedBaseURLOverrideMismatch(
	cfg *RuntimeConfig,
	meta *Metadata,
	cfgBeforeOverrides RuntimeConfig,
	metaSourcesBeforeOverrides map[string]ValueSource,
) bool {
	if cfg == nil || meta == nil {
		return false
	}
	if cfg.Profile == RuntimeProfileProduction {
		return false
	}
	if meta.Source("base_url") != SourceOverride {
		return false
	}

	fallbackBaseURL := strings.TrimSpace(cfgBeforeOverrides.BaseURL)
	if fallbackBaseURL == "" || fallbackBaseURL == strings.TrimSpace(cfg.BaseURL) {
		return false
	}

	candidate := *cfg
	candidate.BaseURL = fallbackBaseURL
	if _, err := ResolveLLMProfile(candidate); err != nil {
		return false
	}

	*cfg = candidate
	if source, ok := metaSourcesBeforeOverrides["base_url"]; ok {
		meta.sources["base_url"] = source
	} else {
		delete(meta.sources, "base_url")
	}
	return true
}

func normalizeRuntimeConfig(cfg *RuntimeConfig) {
	cfg.LLMProvider = strings.TrimSpace(cfg.LLMProvider)
	cfg.LLMModel = strings.TrimSpace(cfg.LLMModel)
	cfg.LLMSmallProvider = strings.TrimSpace(cfg.LLMSmallProvider)
	cfg.LLMSmallModel = strings.TrimSpace(cfg.LLMSmallModel)
	cfg.LLMVisionModel = strings.TrimSpace(cfg.LLMVisionModel)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.ArkAPIKey = strings.TrimSpace(cfg.ArkAPIKey)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.SandboxBaseURL = strings.TrimSpace(cfg.SandboxBaseURL)
	cfg.ACPExecutorAddr = strings.TrimSpace(cfg.ACPExecutorAddr)
	cfg.ACPExecutorCWD = strings.TrimSpace(cfg.ACPExecutorCWD)
	cfg.ACPExecutorMode = strings.TrimSpace(cfg.ACPExecutorMode)
	cfg.TavilyAPIKey = strings.TrimSpace(cfg.TavilyAPIKey)
	cfg.MoltbookAPIKey = strings.TrimSpace(cfg.MoltbookAPIKey)
	cfg.MoltbookBaseURL = strings.TrimSpace(cfg.MoltbookBaseURL)
	cfg.SeedreamTextEndpointID = strings.TrimSpace(cfg.SeedreamTextEndpointID)
	cfg.SeedreamImageEndpointID = strings.TrimSpace(cfg.SeedreamImageEndpointID)
	cfg.SeedreamTextModel = strings.TrimSpace(cfg.SeedreamTextModel)
	cfg.SeedreamImageModel = strings.TrimSpace(cfg.SeedreamImageModel)
	cfg.SeedreamVisionModel = strings.TrimSpace(cfg.SeedreamVisionModel)
	cfg.SeedreamVideoModel = strings.TrimSpace(cfg.SeedreamVideoModel)
	cfg.Profile = NormalizeRuntimeProfile(cfg.Profile)
	cfg.Environment = strings.TrimSpace(cfg.Environment)
	cfg.SessionDir = strings.TrimSpace(cfg.SessionDir)
	cfg.CostDir = strings.TrimSpace(cfg.CostDir)
	cfg.AgentPreset = strings.TrimSpace(cfg.AgentPreset)
	cfg.ToolPreset = strings.TrimSpace(cfg.ToolPreset)
	cfg.Toolset = strings.TrimSpace(cfg.Toolset)
	cfg.Browser.CDPURL = strings.TrimSpace(cfg.Browser.CDPURL)
	cfg.Browser.Connector = strings.TrimSpace(cfg.Browser.Connector)
	cfg.Browser.ChromePath = strings.TrimSpace(cfg.Browser.ChromePath)
	cfg.Browser.UserDataDir = strings.TrimSpace(cfg.Browser.UserDataDir)
	cfg.Browser.BridgeListen = strings.TrimSpace(cfg.Browser.BridgeListen)
	cfg.Browser.BridgeToken = strings.TrimSpace(cfg.Browser.BridgeToken)
	normalizeProactiveConfig(&cfg.Proactive)
	normalizeExternalAgentsConfig(&cfg.ExternalAgents)
	normalizeHTTPLimits(&cfg.HTTPLimits)
	normalizeToolPolicy(&cfg.ToolPolicy)

	if cfg.ToolMaxConcurrent <= 0 {
		cfg.ToolMaxConcurrent = DefaultToolMaxConcurrent
	}
	if cfg.LLMCacheSize < 0 {
		cfg.LLMCacheSize = 0
	}
	if cfg.LLMCacheTTLSeconds < 0 {
		cfg.LLMCacheTTLSeconds = 0
	}
	if cfg.SessionStaleAfterSeconds < 0 {
		cfg.SessionStaleAfterSeconds = 0
	}
	if cfg.Browser.TimeoutSeconds < 0 {
		cfg.Browser.TimeoutSeconds = 0
	}
	if cfg.Browser.Connector == "" {
		cfg.Browser.Connector = "cdp"
	}
	if cfg.Browser.BridgeListen == "" {
		cfg.Browser.BridgeListen = "127.0.0.1:17333"
	}

	if len(cfg.StopSequences) > 0 {
		filtered := cfg.StopSequences[:0]
		seen := make(map[string]struct{}, len(cfg.StopSequences))
		for _, seq := range cfg.StopSequences {
			trimmed := strings.TrimSpace(seq)
			if trimmed == "" {
				continue
			}
			if _, exists := seen[trimmed]; exists {
				continue
			}
			seen[trimmed] = struct{}{}
			filtered = append(filtered, trimmed)
		}
		cfg.StopSequences = filtered
	}
}

func normalizeExternalAgentsConfig(cfg *ExternalAgentsConfig) {
	if cfg == nil {
		return
	}
	cfg.ClaudeCode.Binary = strings.TrimSpace(cfg.ClaudeCode.Binary)
	cfg.ClaudeCode.DefaultModel = strings.TrimSpace(cfg.ClaudeCode.DefaultModel)
	cfg.ClaudeCode.DefaultMode = strings.TrimSpace(cfg.ClaudeCode.DefaultMode)
	if len(cfg.ClaudeCode.AutonomousAllowedTools) > 0 {
		filtered := cfg.ClaudeCode.AutonomousAllowedTools[:0]
		seen := make(map[string]struct{}, len(cfg.ClaudeCode.AutonomousAllowedTools))
		for _, tool := range cfg.ClaudeCode.AutonomousAllowedTools {
			trimmed := strings.TrimSpace(tool)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			filtered = append(filtered, trimmed)
		}
		cfg.ClaudeCode.AutonomousAllowedTools = filtered
	}
	cfg.Codex.Binary = strings.TrimSpace(cfg.Codex.Binary)
	cfg.Codex.DefaultModel = strings.TrimSpace(cfg.Codex.DefaultModel)
	cfg.Codex.ApprovalPolicy = strings.TrimSpace(cfg.Codex.ApprovalPolicy)
	cfg.Codex.Sandbox = strings.TrimSpace(cfg.Codex.Sandbox)
}

func normalizeHTTPLimits(cfg *HTTPLimitsConfig) {
	if cfg == nil {
		return
	}
	if cfg.DefaultMaxResponseBytes <= 0 {
		cfg.DefaultMaxResponseBytes = DefaultHTTPMaxResponse
	}
	if cfg.WebFetchMaxResponseBytes <= 0 {
		cfg.WebFetchMaxResponseBytes = 2 * DefaultHTTPMaxResponse
	}
	if cfg.WebSearchMaxResponseBytes <= 0 {
		cfg.WebSearchMaxResponseBytes = DefaultHTTPMaxResponse
	}
	if cfg.MusicSearchMaxResponseBytes <= 0 {
		cfg.MusicSearchMaxResponseBytes = DefaultHTTPMaxResponse
	}
	if cfg.ModelListMaxResponseBytes <= 0 {
		cfg.ModelListMaxResponseBytes = 512 * 1024
	}
	if cfg.SandboxMaxResponseBytes <= 0 {
		cfg.SandboxMaxResponseBytes = 8 * 1024 * 1024
	}
}

func normalizeProactiveConfig(cfg *ProactiveConfig) {
	if cfg == nil {
		return
	}
	cfg.Prompt.Mode = strings.TrimSpace(strings.ToLower(cfg.Prompt.Mode))
	switch cfg.Prompt.Mode {
	case "full", "minimal", "none":
	default:
		cfg.Prompt.Mode = "full"
	}
	cfg.Prompt.Timezone = strings.TrimSpace(cfg.Prompt.Timezone)
	if cfg.Prompt.BootstrapMaxChars <= 0 {
		cfg.Prompt.BootstrapMaxChars = 20000
	}
	if len(cfg.Prompt.BootstrapFiles) == 0 {
		cfg.Prompt.BootstrapFiles = []string{
			"AGENTS.md",
			"SOUL.md",
			"TOOLS.md",
			"IDENTITY.md",
			"USER.md",
			"HEARTBEAT.md",
			"BOOTSTRAP.md",
		}
	} else {
		filtered := cfg.Prompt.BootstrapFiles[:0]
		for _, path := range cfg.Prompt.BootstrapFiles {
			if trimmed := strings.TrimSpace(path); trimmed != "" {
				filtered = append(filtered, trimmed)
			}
		}
		cfg.Prompt.BootstrapFiles = filtered
	}
	if cfg.FinalAnswerReview.MaxExtraIterations <= 0 {
		cfg.FinalAnswerReview.MaxExtraIterations = 1
	}
	cfg.Memory.Index.DBPath = strings.TrimSpace(cfg.Memory.Index.DBPath)
	cfg.Memory.Index.EmbedderModel = strings.TrimSpace(cfg.Memory.Index.EmbedderModel)
	if cfg.Memory.Index.ChunkTokens <= 0 {
		cfg.Memory.Index.ChunkTokens = 400
	}
	if cfg.Memory.Index.ChunkOverlap < 0 {
		cfg.Memory.Index.ChunkOverlap = 0
	}
	if cfg.Memory.Index.MinScore <= 0 {
		cfg.Memory.Index.MinScore = 0.35
	}
	if cfg.Memory.Index.FusionWeightVector == 0 && cfg.Memory.Index.FusionWeightBM25 == 0 {
		cfg.Memory.Index.FusionWeightVector = 0.7
		cfg.Memory.Index.FusionWeightBM25 = 0.3
	}
	if cfg.Skills.AutoActivation.MaxActivated <= 0 {
		cfg.Skills.AutoActivation.MaxActivated = 3
	}
	if cfg.Skills.AutoActivation.TokenBudget <= 0 {
		cfg.Skills.AutoActivation.TokenBudget = 4000
	}
	if cfg.Skills.AutoActivation.ConfidenceThreshold <= 0 {
		cfg.Skills.AutoActivation.ConfidenceThreshold = 0.6
	}
	if cfg.Skills.CacheTTLSeconds <= 0 {
		cfg.Skills.CacheTTLSeconds = 300
	}
	cfg.Skills.ProactiveLevel = strings.ToLower(strings.TrimSpace(cfg.Skills.ProactiveLevel))
	switch cfg.Skills.ProactiveLevel {
	case "", "low", "medium", "high":
	default:
		cfg.Skills.ProactiveLevel = "medium"
	}
	if cfg.Skills.ProactiveLevel == "" {
		cfg.Skills.ProactiveLevel = "medium"
	}
	cfg.Skills.PolicyPath = strings.TrimSpace(cfg.Skills.PolicyPath)
	if cfg.Skills.PolicyPath == "" {
		cfg.Skills.PolicyPath = "configs/skills/meta-orchestrator.yaml"
	}
	if cfg.Attention.MaxDailyNotifications <= 0 {
		cfg.Attention.MaxDailyNotifications = 5
	}
	if cfg.Attention.MinIntervalSeconds <= 0 {
		cfg.Attention.MinIntervalSeconds = 1800
	}
	if cfg.Attention.PriorityThreshold <= 0 {
		cfg.Attention.PriorityThreshold = 0.6
	}
	if cfg.Attention.QuietHours[0] == 0 && cfg.Attention.QuietHours[1] == 0 {
		cfg.Attention.QuietHours = [2]int{22, 8}
	}
	if strings.TrimSpace(cfg.Scheduler.ConcurrencyPolicy) == "" {
		cfg.Scheduler.ConcurrencyPolicy = "skip"
	}
	if cfg.Scheduler.TriggerTimeoutSeconds <= 0 {
		cfg.Scheduler.TriggerTimeoutSeconds = 900
	}
	if cfg.Scheduler.CooldownSeconds < 0 {
		cfg.Scheduler.CooldownSeconds = 0
	}
	if cfg.Scheduler.MaxConcurrent <= 0 {
		cfg.Scheduler.MaxConcurrent = 1
	}
	if cfg.Scheduler.RecoveryMaxRetries < 0 {
		cfg.Scheduler.RecoveryMaxRetries = 0
	}
	if cfg.Scheduler.RecoveryBackoffSeconds <= 0 {
		cfg.Scheduler.RecoveryBackoffSeconds = 60
	}
	cfg.Scheduler.Heartbeat.Schedule = strings.TrimSpace(cfg.Scheduler.Heartbeat.Schedule)
	if cfg.Scheduler.Heartbeat.Schedule == "" {
		cfg.Scheduler.Heartbeat.Schedule = "*/30 * * * *"
	}
	cfg.Scheduler.Heartbeat.Task = strings.TrimSpace(cfg.Scheduler.Heartbeat.Task)
	if cfg.Scheduler.Heartbeat.Task == "" {
		cfg.Scheduler.Heartbeat.Task = "Read HEARTBEAT.md if it exists. Follow it strictly. If nothing needs attention, reply HEARTBEAT_OK."
	}
	cfg.Scheduler.Heartbeat.Channel = strings.TrimSpace(cfg.Scheduler.Heartbeat.Channel)
	cfg.Scheduler.Heartbeat.UserID = strings.TrimSpace(cfg.Scheduler.Heartbeat.UserID)
	cfg.Scheduler.Heartbeat.ChatID = strings.TrimSpace(cfg.Scheduler.Heartbeat.ChatID)
	if cfg.Scheduler.Heartbeat.WindowLookbackHr <= 0 {
		cfg.Scheduler.Heartbeat.WindowLookbackHr = 8
	}
	if cfg.Scheduler.Heartbeat.QuietHours[0] == 0 && cfg.Scheduler.Heartbeat.QuietHours[1] == 0 {
		cfg.Scheduler.Heartbeat.QuietHours = [2]int{23, 8}
	}
	cfg.Scheduler.JobStorePath = strings.TrimSpace(cfg.Scheduler.JobStorePath)
	cfg.Timer.StorePath = strings.TrimSpace(cfg.Timer.StorePath)
	if cfg.Timer.HeartbeatMinutes <= 0 {
		cfg.Timer.HeartbeatMinutes = 30
	}
}

func shouldLoadCLICredentials(cfg RuntimeConfig) bool {
	provider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	switch provider {
	case "auto", "cli":
		return true
	}

	if strings.TrimSpace(cfg.APIKey) != "" {
		return false
	}

	switch provider {
	case "codex", "openai-responses", "responses", "anthropic", "claude":
		return true
	default:
		return false
	}
}

func normalizeToolPolicy(cfg *toolspolicy.ToolPolicyConfig) {
	if cfg == nil {
		return
	}
	cfg.EnforcementMode = strings.TrimSpace(strings.ToLower(cfg.EnforcementMode))
	switch cfg.EnforcementMode {
	case "", "enforce", "warn_allow":
	default:
		cfg.EnforcementMode = ""
	}
	defaults := toolspolicy.DefaultToolPolicyConfig()
	if cfg.Timeout.Default <= 0 {
		cfg.Timeout.Default = defaults.Timeout.Default
	}
	if cfg.Timeout.PerTool == nil {
		cfg.Timeout.PerTool = map[string]time.Duration{}
	}
	if cfg.Retry.InitialBackoff <= 0 {
		cfg.Retry.InitialBackoff = defaults.Retry.InitialBackoff
	}
	if cfg.Retry.MaxBackoff <= 0 {
		cfg.Retry.MaxBackoff = defaults.Retry.MaxBackoff
	}
	if cfg.Retry.BackoffFactor <= 0 {
		cfg.Retry.BackoffFactor = defaults.Retry.BackoffFactor
	}
	// MaxRetries == 0 is a valid value (no retries), so we only fix negative.
	if cfg.Retry.MaxRetries < 0 {
		cfg.Retry.MaxRetries = 0
	}
	if cfg.EnforcementMode == "" {
		cfg.EnforcementMode = defaults.EnforcementMode
	}
}
