package subscription

import "strings"

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
		SetupHint: "Sign in with Codex CLI (`codex login`) or set CODEX_API_KEY.",
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
		SetupHint: "Sign in with Claude CLI or set CLAUDE_CODE_OAUTH_TOKEN / ANTHROPIC_API_KEY.",
	},
	"llama_server": {
		DisplayName: "Llama Server",
		AuthMode:    "local_server",
		SetupHint:   "Start a local OpenAI-compatible llama server (LLAMA_SERVER_BASE_URL).",
	},
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
