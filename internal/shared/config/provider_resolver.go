package config

import "strings"

type autoProviderCandidate struct {
	Provider    string
	APIKeyEnv   string
	BaseURLEnv  string
	DefaultBase string
	Source      ValueSource
}

func applyAutoProviderCandidate(cfg *RuntimeConfig, meta *Metadata, lookup EnvLookup, cand autoProviderCandidate) bool {
	key, ok := lookup(cand.APIKeyEnv)
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return false
	}

	cfg.LLMProvider = cand.Provider
	meta.sources["llm_provider"] = cand.Source
	if cfg.APIKey == "" {
		cfg.APIKey = key
		meta.sources["api_key"] = cand.Source
	}

	if cfg.LLMSmallProvider == "" || strings.EqualFold(cfg.LLMSmallProvider, "auto") || strings.EqualFold(cfg.LLMSmallProvider, "cli") {
		cfg.LLMSmallProvider = cand.Provider
		meta.sources["llm_small_provider"] = cand.Source
	}

	if base, ok := lookup(cand.BaseURLEnv); ok && strings.TrimSpace(base) != "" {
		cfg.BaseURL = strings.TrimSpace(base)
		meta.sources["base_url"] = SourceEnv
	} else if cand.DefaultBase != "" && meta.Source("base_url") == SourceDefault {
		cfg.BaseURL = cand.DefaultBase
		meta.sources["base_url"] = cand.Source
	}

	return true
}

func applyCLICandidates(cfg *RuntimeConfig, meta *Metadata, candidates ...CLICredential) bool {
	for _, cand := range candidates {
		if strings.TrimSpace(cand.APIKey) == "" {
			continue
		}
		cfg.LLMProvider = cand.Provider
		meta.sources["llm_provider"] = cand.Source
		if cfg.APIKey == "" {
			cfg.APIKey = cand.APIKey
			meta.sources["api_key"] = cand.Source
		}
		if cfg.LLMSmallProvider == "" || strings.EqualFold(cfg.LLMSmallProvider, "auto") || strings.EqualFold(cfg.LLMSmallProvider, "cli") {
			cfg.LLMSmallProvider = cand.Provider
			meta.sources["llm_small_provider"] = cand.Source
		}
		if cand.BaseURL != "" && meta.Source("base_url") == SourceDefault {
			cfg.BaseURL = cand.BaseURL
			meta.sources["base_url"] = cand.Source
		}
		if cand.Model != "" && meta.Source("llm_model") == SourceDefault {
			cfg.LLMModel = cand.Model
			meta.sources["llm_model"] = cand.Source
		}
		return true
	}
	return false
}

func resolveAutoProvider(cfg *RuntimeConfig, meta *Metadata, lookup EnvLookup, cli CLICredentials) {
	if cfg == nil {
		return
	}
	if lookup == nil {
		lookup = DefaultEnvLookup
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	if provider != "auto" && provider != "cli" {
		return
	}

	candidates := []autoProviderCandidate{
		{
			Provider:    "anthropic",
			APIKeyEnv:   "CLAUDE_CODE_OAUTH_TOKEN",
			BaseURLEnv:  "ANTHROPIC_BASE_URL",
			DefaultBase: "https://api.anthropic.com/v1",
			Source:      SourceClaudeCLI,
		},
		{
			Provider:    "anthropic",
			APIKeyEnv:   "ANTHROPIC_AUTH_TOKEN",
			BaseURLEnv:  "ANTHROPIC_BASE_URL",
			DefaultBase: "https://api.anthropic.com/v1",
			Source:      SourceClaudeCLI,
		},
		{
			Provider:    "anthropic",
			APIKeyEnv:   "ANTHROPIC_API_KEY",
			BaseURLEnv:  "ANTHROPIC_BASE_URL",
			DefaultBase: "https://api.anthropic.com/v1",
			Source:      SourceEnv,
		},
		{
			Provider:    "codex",
			APIKeyEnv:   "CODEX_API_KEY",
			BaseURLEnv:  "CODEX_BASE_URL",
			DefaultBase: codexCLIBaseURL,
			Source:      SourceEnv,
		},
		{
			Provider:   "openai",
			APIKeyEnv:  "OPENAI_API_KEY",
			BaseURLEnv: "OPENAI_BASE_URL",
			Source:     SourceEnv,
		},
	}

	applyEnv := func() bool {
		for _, cand := range candidates {
			if applyAutoProviderCandidate(cfg, meta, lookup, cand) {
				return true
			}
		}
		return false
	}
	applyCLI := func() bool {
		return applyCLICandidates(cfg, meta, cli.Codex, cli.Claude)
	}
	applyUnified := func() bool {
		if key, ok := lookup("LLM_API_KEY"); ok && strings.TrimSpace(key) != "" {
			cfg.LLMProvider = "openai"
			meta.sources["llm_provider"] = SourceEnv
			if cfg.APIKey == "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
			if cfg.LLMSmallProvider == "" || strings.EqualFold(cfg.LLMSmallProvider, "auto") || strings.EqualFold(cfg.LLMSmallProvider, "cli") {
				cfg.LLMSmallProvider = "openai"
				meta.sources["llm_small_provider"] = SourceEnv
			}
			if base, ok := lookup("OPENAI_BASE_URL"); ok && strings.TrimSpace(base) != "" {
				cfg.BaseURL = strings.TrimSpace(base)
				meta.sources["base_url"] = SourceEnv
			}
			return true
		}
		return false
	}

	if provider == "cli" {
		if applyCLI() {
			return
		}
		if applyEnv() {
			return
		}
		_ = applyUnified()
		return
	}

	if applyEnv() {
		return
	}
	if applyUnified() {
		return
	}
	_ = applyCLI()
}

func resolveProviderCredentials(cfg *RuntimeConfig, meta *Metadata, lookup EnvLookup, cli CLICredentials) {
	if cfg == nil {
		return
	}
	if lookup == nil {
		lookup = DefaultEnvLookup
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	if provider == "" {
		return
	}

	if cfg.APIKey == "" {
		switch provider {
		case "anthropic", "claude":
			if cli.Claude.APIKey != "" {
				cfg.APIKey = strings.TrimSpace(cli.Claude.APIKey)
				meta.sources["api_key"] = cli.Claude.Source
				break
			}
			if key, ok := lookup("ANTHROPIC_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
		case "openai-responses", "responses", "codex":
			if cli.Codex.APIKey != "" {
				cfg.APIKey = strings.TrimSpace(cli.Codex.APIKey)
				meta.sources["api_key"] = cli.Codex.Source
				break
			}
			if key, ok := lookup("CODEX_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
				break
			}
			if key, ok := lookup("OPENAI_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
		case "openai", "openrouter", "deepseek":
			if key, ok := lookup("OPENAI_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
		}
		if cfg.APIKey == "" {
			if key, ok := lookup("LLM_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
		}
	}

	if meta.Source("llm_model") == SourceDefault {
		switch provider {
		case "codex":
			if cli.Codex.Model != "" {
				cfg.LLMModel = cli.Codex.Model
				meta.sources["llm_model"] = cli.Codex.Source
			}
		}
	}

	if meta.Source("base_url") == SourceDefault {
		switch provider {
		case "anthropic", "claude":
			if base, ok := lookup("ANTHROPIC_BASE_URL"); ok && strings.TrimSpace(base) != "" {
				cfg.BaseURL = strings.TrimSpace(base)
				meta.sources["base_url"] = SourceEnv
			} else {
				cfg.BaseURL = "https://api.anthropic.com/v1"
				meta.sources["base_url"] = SourceEnv
			}
		case "openai-responses", "responses", "codex":
			if cli.Codex.BaseURL != "" {
				cfg.BaseURL = cli.Codex.BaseURL
				meta.sources["base_url"] = cli.Codex.Source
				break
			}
			if base, ok := lookup("CODEX_BASE_URL"); ok && strings.TrimSpace(base) != "" {
				cfg.BaseURL = strings.TrimSpace(base)
				meta.sources["base_url"] = SourceEnv
				break
			}
			if provider == "codex" {
				cfg.BaseURL = codexCLIBaseURL
				meta.sources["base_url"] = SourceEnv
			}
		case "openai", "openrouter", "deepseek":
			if base, ok := lookup("OPENAI_BASE_URL"); ok && strings.TrimSpace(base) != "" {
				cfg.BaseURL = strings.TrimSpace(base)
				meta.sources["base_url"] = SourceEnv
			}
		}
	}
}
