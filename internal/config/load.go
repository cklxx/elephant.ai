package config

import (
	"os"
	"strings"
	"time"
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
		SessionDir:                 "~/.alex-sessions",
		CostDir:                    "~/.alex-costs",
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

	// Apply caller overrides last.
	applyOverrides(&cfg, &meta, options.overrides)

	normalizeRuntimeConfig(&cfg)
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
	// If API key remains unset, default to mock provider (unless ollama).
	if cfg.APIKey == "" && cfg.LLMProvider != "mock" && cfg.LLMProvider != "ollama" {
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

	return cfg, meta, nil
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
	cfg.SeedreamTextEndpointID = strings.TrimSpace(cfg.SeedreamTextEndpointID)
	cfg.SeedreamImageEndpointID = strings.TrimSpace(cfg.SeedreamImageEndpointID)
	cfg.SeedreamTextModel = strings.TrimSpace(cfg.SeedreamTextModel)
	cfg.SeedreamImageModel = strings.TrimSpace(cfg.SeedreamImageModel)
	cfg.SeedreamVisionModel = strings.TrimSpace(cfg.SeedreamVisionModel)
	cfg.SeedreamVideoModel = strings.TrimSpace(cfg.SeedreamVideoModel)
	cfg.Environment = strings.TrimSpace(cfg.Environment)
	cfg.SessionDir = strings.TrimSpace(cfg.SessionDir)
	cfg.CostDir = strings.TrimSpace(cfg.CostDir)
	cfg.AgentPreset = strings.TrimSpace(cfg.AgentPreset)
	cfg.ToolPreset = strings.TrimSpace(cfg.ToolPreset)

	if cfg.ToolMaxConcurrent <= 0 {
		cfg.ToolMaxConcurrent = DefaultToolMaxConcurrent
	}
	if cfg.LLMCacheSize < 0 {
		cfg.LLMCacheSize = 0
	}
	if cfg.LLMCacheTTLSeconds < 0 {
		cfg.LLMCacheTTLSeconds = 0
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
	case "codex", "openai-responses", "responses", "anthropic", "claude", "antigravity":
		return true
	default:
		return false
	}
}
