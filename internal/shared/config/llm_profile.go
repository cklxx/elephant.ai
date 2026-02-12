package config

import (
	"fmt"
	"net/url"
	"strings"
)

// LLMProfile is the resolved runtime profile consumed by executors and app services.
// The profile is intentionally atomic to prevent provider/model/key/base_url mismatch
// drift across component boundaries.
type LLMProfile struct {
	Provider       string
	Model          string
	APIKey         string
	BaseURL        string
	Headers        map[string]string
	TimeoutSeconds int
}

// LLMProfileMismatchError indicates a provider/key/base_url mismatch.
type LLMProfileMismatchError struct {
	Provider string
	Detail   string
}

func (e *LLMProfileMismatchError) Error() string {
	if e == nil {
		return "invalid llm profile"
	}
	if strings.TrimSpace(e.Detail) == "" {
		return fmt.Sprintf("invalid llm profile for provider %q", strings.TrimSpace(e.Provider))
	}
	return fmt.Sprintf("invalid llm profile for provider %q: %s", strings.TrimSpace(e.Provider), strings.TrimSpace(e.Detail))
}

// ResolveLLMProfile resolves an atomic LLM profile from runtime config and enforces
// mismatch checks so calling components can safely use the profile without handling
// config heuristics on their own.
func ResolveLLMProfile(cfg RuntimeConfig) (LLMProfile, error) {
	profile := LLMProfile{
		Provider:       strings.TrimSpace(cfg.LLMProvider),
		Model:          strings.TrimSpace(cfg.LLMModel),
		APIKey:         strings.TrimSpace(cfg.APIKey),
		BaseURL:        strings.TrimSpace(cfg.BaseURL),
		TimeoutSeconds: cfg.LLMRequestTimeoutSeconds,
	}
	if err := ValidateLLMProfile(profile); err != nil {
		return LLMProfile{}, err
	}
	return profile, nil
}

// ValidateLLMProfile validates key/base URL consistency for the selected provider.
func ValidateLLMProfile(profile LLMProfile) error {
	provider := normalizeProviderFamily(profile.Provider)
	if provider == "" {
		return nil
	}
	if mismatch, detail := detectKeyProviderMismatch(provider, profile.APIKey, profile.BaseURL); mismatch {
		return &LLMProfileMismatchError{Provider: profile.Provider, Detail: detail}
	}
	if mismatch, detail := detectBaseURLProviderMismatch(provider, profile.BaseURL); mismatch {
		return &LLMProfileMismatchError{Provider: profile.Provider, Detail: detail}
	}
	return nil
}

func normalizeProviderFamily(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openai-responses", "responses", "codex":
		return "codex"
	case "openai", "openrouter", "deepseek", "kimi", "glm", "minimax":
		return "openai"
	case "anthropic", "claude":
		return "anthropic"
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func detectKeyProviderMismatch(provider, apiKey, baseURL string) (bool, string) {
	key := strings.TrimSpace(apiKey)
	if key == "" {
		return false, ""
	}

	switch provider {
	case "anthropic":
		// Anthropic keys can come from OAuth/session tokens without sk-ant-* prefix.
		// We only fail fast on clearly incompatible vendor-specific prefixes.
		if hasAnyPrefix(key, "sk-proj-", "sk-kimi-", "sk-deepseek-", "sess-codex-") {
			return true, fmt.Sprintf("api key prefix=%s looks incompatible with anthropic provider", safeAPIKeyPrefix(key))
		}
	case "codex", "openai":
		// codex family expects OpenAI/Codex credentials. Non-OpenAI vendor keys
		// should fail fast unless the endpoint is explicitly vendor-compatible.
		switch {
		case strings.HasPrefix(key, "sk-ant-"):
			return true, fmt.Sprintf("api key prefix=%s looks anthropic and is incompatible with %s provider", safeAPIKeyPrefix(key), provider)
		case strings.HasPrefix(key, "sk-kimi-") && !hostMatchesAny(baseURL, "moonshot", "kimi"):
			return true, fmt.Sprintf("api key prefix=%s looks moonshot/kimi but base_url=%q is not moonshot-compatible for %s provider", safeAPIKeyPrefix(key), strings.TrimSpace(baseURL), provider)
		case strings.HasPrefix(key, "sk-deepseek-") && !hostMatchesAny(baseURL, "deepseek"):
			return true, fmt.Sprintf("api key prefix=%s looks deepseek but base_url=%q is not deepseek-compatible for %s provider", safeAPIKeyPrefix(key), strings.TrimSpace(baseURL), provider)
		}
	}

	return false, ""
}

func hasAnyPrefix(value string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func detectBaseURLProviderMismatch(provider, baseURL string) (bool, string) {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return false, ""
	}

	switch provider {
	case "anthropic":
		if hostMatchesAny(trimmed, "openai", "chatgpt", "codex") {
			return true, fmt.Sprintf("base_url=%q looks like OpenAI/Codex endpoint for anthropic provider", trimmed)
		}
	case "codex", "openai":
		if hostMatchesAny(trimmed, "anthropic") {
			return true, fmt.Sprintf("base_url=%q looks like anthropic endpoint for %s provider", trimmed, provider)
		}
	}
	return false, ""
}

func hostMatchesAny(rawURL string, needles ...string) bool {
	host := extractHost(rawURL)
	if host == "" {
		return false
	}
	for _, needle := range needles {
		token := strings.ToLower(strings.TrimSpace(needle))
		if token == "" {
			continue
		}
		if strings.Contains(host, token) {
			return true
		}
	}
	return false
}

func extractHost(rawURL string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return strings.ToLower(trimmed)
	}
	return strings.ToLower(parsed.Host)
}

func safeAPIKeyPrefix(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:8] + "..."
}
