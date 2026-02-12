package subscription

import (
	"sort"
	"strings"
)

// ModelRecommendation marks a curated model choice for onboarding and pickers.
type ModelRecommendation struct {
	ID      string `json:"id"`
	Tier    string `json:"tier,omitempty"`
	Default bool   `json:"default,omitempty"`
	Note    string `json:"note,omitempty"`
}

type providerPreset struct {
	DisplayName       string
	AuthMode          string
	DefaultBaseURL    string
	DefaultModel      string
	RecommendedModels []ModelRecommendation
	KeyCreateURL      string
	SetupHint         string
}

var providerPresets = map[string]providerPreset{
	"codex": {
		DisplayName:    "Codex",
		AuthMode:       "cli_oauth_or_api_key",
		DefaultBaseURL: "https://chatgpt.com/backend-api/codex",
		DefaultModel:   "gpt-5.2-codex",
		RecommendedModels: []ModelRecommendation{
			{ID: "gpt-5.2-codex", Tier: "balanced", Default: true},
			{ID: "gpt-5.2", Tier: "quality"},
			{ID: "gpt-5.1-codex-mini", Tier: "fast"},
			{ID: "gpt-5.1-codex-max", Tier: "quality"},
		},
		KeyCreateURL: "https://chatgpt.com/#settings/Codex",
		SetupHint: "Sign in with Codex CLI (`codex login`) or set CODEX_API_KEY.",
	},
	"openai": {
		DisplayName:    "OpenAI",
		AuthMode:       "api_key",
		DefaultBaseURL: "https://api.openai.com/v1",
		DefaultModel:   "gpt-5-mini",
		RecommendedModels: []ModelRecommendation{
			{ID: "gpt-5-mini", Tier: "balanced", Default: true},
			{ID: "gpt-5", Tier: "quality"},
			{ID: "gpt-4o-mini", Tier: "fast"},
		},
		KeyCreateURL: "https://platform.openai.com/api-keys",
		SetupHint:    "Create an API key in OpenAI console and paste it here.",
	},
	"openrouter": {
		DisplayName:    "OpenRouter",
		AuthMode:       "api_key",
		DefaultBaseURL: "https://openrouter.ai/api/v1",
		DefaultModel:   "openai/gpt-4o-mini",
		RecommendedModels: []ModelRecommendation{
			{ID: "openai/gpt-4o-mini", Tier: "balanced", Default: true},
			{ID: "anthropic/claude-3.7-sonnet", Tier: "quality"},
			{ID: "deepseek/deepseek-chat", Tier: "fast"},
		},
		KeyCreateURL: "https://openrouter.ai/settings/keys",
		SetupHint:    "Create an API key in OpenRouter console and paste it here.",
	},
	"anthropic": {
		DisplayName:    "Anthropic",
		AuthMode:       "cli_oauth_or_api_key",
		DefaultBaseURL: "https://api.anthropic.com/v1",
		DefaultModel:   "claude-sonnet-4-20250514",
		RecommendedModels: []ModelRecommendation{
			{ID: "claude-sonnet-4-20250514", Tier: "balanced", Default: true},
			{ID: "claude-3-7-sonnet-20250219", Tier: "quality"},
		},
		KeyCreateURL: "https://console.anthropic.com/settings/keys",
		SetupHint: "Sign in with Claude CLI or set CLAUDE_CODE_OAUTH_TOKEN / ANTHROPIC_API_KEY.",
	},
	"claude": {
		DisplayName:    "Anthropic",
		AuthMode:       "cli_oauth_or_api_key",
		DefaultBaseURL: "https://api.anthropic.com/v1",
		DefaultModel:   "claude-sonnet-4-20250514",
		RecommendedModels: []ModelRecommendation{
			{ID: "claude-sonnet-4-20250514", Tier: "balanced", Default: true},
			{ID: "claude-3-7-sonnet-20250219", Tier: "quality"},
		},
		KeyCreateURL: "https://console.anthropic.com/settings/keys",
		SetupHint: "Sign in with Claude CLI or set CLAUDE_CODE_OAUTH_TOKEN / ANTHROPIC_API_KEY.",
	},
	"kimi": {
		DisplayName:    "Kimi (Moonshot)",
		AuthMode:       "api_key",
		DefaultBaseURL: "https://api.moonshot.cn/v1",
		DefaultModel:   "kimi-k2-0711-preview",
		RecommendedModels: []ModelRecommendation{
			{ID: "kimi-k2-0711-preview", Tier: "balanced", Default: true},
			{ID: "moonshot-v1-8k", Tier: "fast"},
		},
		KeyCreateURL: "https://platform.moonshot.cn/console/api-keys",
		SetupHint:    "Create an API key in Moonshot console and paste it here.",
	},
	"glm": {
		DisplayName:    "GLM (Zhipu)",
		AuthMode:       "api_key",
		DefaultBaseURL: "https://open.bigmodel.cn/api/paas/v4",
		DefaultModel:   "glm-4.5-flash",
		RecommendedModels: []ModelRecommendation{
			{ID: "glm-4.5-flash", Tier: "balanced", Default: true},
			{ID: "glm-4.5", Tier: "quality"},
		},
		KeyCreateURL: "https://open.bigmodel.cn/usercenter/apikeys",
		SetupHint:    "Create an API key in BigModel console and paste it here.",
	},
	"minimax": {
		DisplayName:    "MiniMax",
		AuthMode:       "api_key",
		DefaultBaseURL: "https://api.minimax.chat/v1",
		DefaultModel:   "MiniMax-Text-01",
		RecommendedModels: []ModelRecommendation{
			{ID: "MiniMax-Text-01", Tier: "balanced", Default: true},
		},
		KeyCreateURL: "https://platform.minimaxi.com/user-center/basic-information/interface-key",
		SetupHint:    "Create an API key in MiniMax console and paste it here.",
	},
	"llama_server": {
		DisplayName: "Llama Server",
		AuthMode:    "local_server",
		SetupHint:   "Start a local OpenAI-compatible llama server (LLAMA_SERVER_BASE_URL).",
	},
}

// ProviderPreset contains the public setup metadata for a runtime provider.
type ProviderPreset struct {
	Provider          string
	DisplayName       string
	AuthMode          string
	DefaultBaseURL    string
	DefaultModel      string
	RecommendedModels []ModelRecommendation
	KeyCreateURL      string
	SetupHint         string
}

// ListProviderPresets returns all known provider presets sorted by provider id.
func ListProviderPresets() []ProviderPreset {
	keys := make([]string, 0, len(providerPresets))
	for key := range providerPresets {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make([]ProviderPreset, 0, len(keys))
	for _, key := range keys {
		preset := providerPresets[key]
		out = append(out, ProviderPreset{
			Provider:          key,
			DisplayName:       preset.DisplayName,
			AuthMode:          preset.AuthMode,
			DefaultBaseURL:    preset.DefaultBaseURL,
			DefaultModel:      preset.DefaultModel,
			RecommendedModels: append([]ModelRecommendation(nil), preset.RecommendedModels...),
			KeyCreateURL:      preset.KeyCreateURL,
			SetupHint:         preset.SetupHint,
		})
	}
	return out
}

// LookupProviderPreset returns a provider preset by provider id.
func LookupProviderPreset(provider string) (ProviderPreset, bool) {
	key := strings.ToLower(strings.TrimSpace(provider))
	preset, ok := providerPresets[key]
	if !ok {
		return ProviderPreset{}, false
	}
	return ProviderPreset{
		Provider:          key,
		DisplayName:       preset.DisplayName,
		AuthMode:          preset.AuthMode,
		DefaultBaseURL:    preset.DefaultBaseURL,
		DefaultModel:      preset.DefaultModel,
		RecommendedModels: append([]ModelRecommendation(nil), preset.RecommendedModels...),
		KeyCreateURL:      preset.KeyCreateURL,
		SetupHint:         preset.SetupHint,
	}, true
}

func applyCatalogProviderPreset(provider *CatalogProvider) {
	if provider == nil {
		return
	}
	key := strings.ToLower(strings.TrimSpace(provider.Provider))
	preset, ok := providerPresets[key]
	if !ok {
		provider.DisplayName = provider.Provider
		provider.Selectable = true
		return
	}
	provider.DisplayName = preset.DisplayName
	provider.AuthMode = preset.AuthMode
	if strings.TrimSpace(provider.BaseURL) == "" && preset.DefaultBaseURL != "" {
		provider.BaseURL = preset.DefaultBaseURL
	}
	provider.DefaultModel = preset.DefaultModel
	provider.RecommendedModels = append([]ModelRecommendation(nil), preset.RecommendedModels...)
	provider.KeyCreateURL = preset.KeyCreateURL
	provider.SetupHint = preset.SetupHint
	provider.Selectable = true
}

func recommendationIDs(items []ModelRecommendation) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func pickCatalogDefaultModel(provider CatalogProvider) string {
	defaultModel := strings.TrimSpace(provider.DefaultModel)
	if defaultModel != "" {
		if len(provider.Models) == 0 || containsModel(provider.Models, defaultModel) {
			return defaultModel
		}
	}
	for _, item := range provider.RecommendedModels {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if item.Default && (len(provider.Models) == 0 || containsModel(provider.Models, id)) {
			return id
		}
	}
	for _, item := range provider.RecommendedModels {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if len(provider.Models) == 0 || containsModel(provider.Models, id) {
			return id
		}
	}
	if len(provider.Models) > 0 {
		return provider.Models[0]
	}
	return ""
}
