package llmclient

import (
	"fmt"
	"strings"

	portsllm "alex/internal/domain/agent/ports/llm"
	runtimeconfig "alex/internal/shared/config"
)

// CredentialRefresher resolves fresh API credentials for a given provider.
// Returns the api key, base URL and whether refresh succeeded.
type CredentialRefresher func(provider string) (apiKey, baseURL string, ok bool)

// CloneHeaders returns a sanitized copy of headers.
func CloneHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(headers))
	for key, value := range headers {
		k := strings.TrimSpace(key)
		if k == "" {
			continue
		}
		cloned[k] = value
	}
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

// BuildConfig maps a resolved profile to low-level LLM config.
func BuildConfig(profile runtimeconfig.LLMProfile) portsllm.LLMConfig {
	return portsllm.LLMConfig{
		APIKey:  strings.TrimSpace(profile.APIKey),
		BaseURL: strings.TrimSpace(profile.BaseURL),
		Headers: CloneHeaders(profile.Headers),
	}
}

// BuildConfigWithRefresh maps a profile to low-level config and optionally applies
// runtime credential refresh. Refresh is enabled only when refresh=true.
func BuildConfigWithRefresh(profile runtimeconfig.LLMProfile, refresher CredentialRefresher, refresh bool) portsllm.LLMConfig {
	cfg := BuildConfig(profile)
	if !refresh || refresher == nil {
		return cfg
	}
	apiKey, baseURL, ok := refresher(strings.TrimSpace(profile.Provider))
	if !ok {
		return cfg
	}
	cfg.APIKey = strings.TrimSpace(apiKey)
	if trimmed := strings.TrimSpace(baseURL); trimmed != "" {
		cfg.BaseURL = trimmed
	}
	return cfg
}

// GetIsolatedClientFromProfile creates an isolated client directly from profile.
func GetIsolatedClientFromProfile(factory portsllm.LLMClientFactory, profile runtimeconfig.LLMProfile, refresher CredentialRefresher, refresh bool) (portsllm.LLMClient, portsllm.LLMConfig, error) {
	provider := strings.TrimSpace(profile.Provider)
	model := strings.TrimSpace(profile.Model)
	if provider == "" || model == "" {
		return nil, portsllm.LLMConfig{}, fmt.Errorf("llm profile requires provider and model")
	}
	cfg := BuildConfigWithRefresh(profile, refresher, refresh)
	client, err := factory.GetIsolatedClient(provider, model, cfg)
	if err != nil {
		return nil, portsllm.LLMConfig{}, err
	}
	return client, cfg, nil
}

// GetClientFromProfile creates or retrieves a cached client directly from profile.
func GetClientFromProfile(factory portsllm.LLMClientFactory, profile runtimeconfig.LLMProfile, refresher CredentialRefresher, refresh bool) (portsllm.LLMClient, portsllm.LLMConfig, error) {
	provider := strings.TrimSpace(profile.Provider)
	model := strings.TrimSpace(profile.Model)
	if provider == "" || model == "" {
		return nil, portsllm.LLMConfig{}, fmt.Errorf("llm profile requires provider and model")
	}
	cfg := BuildConfigWithRefresh(profile, refresher, refresh)
	client, err := factory.GetClient(provider, model, cfg)
	if err != nil {
		return nil, portsllm.LLMConfig{}, err
	}
	return client, cfg, nil
}
