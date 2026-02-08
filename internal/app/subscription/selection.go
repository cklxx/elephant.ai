package subscription

import (
	"strings"

	runtimeconfig "alex/internal/shared/config"
)

type Selection struct {
	Mode     string `json:"mode"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
	Source   string `json:"source"`
}

type ResolvedSelection struct {
	Provider string
	Model    string
	APIKey   string
	BaseURL  string
	Headers  map[string]string
	Source   string
	Pinned   bool
}

type SelectionResolver struct {
	loadCreds func() runtimeconfig.CLICredentials
}

func NewSelectionResolver(loadCreds func() runtimeconfig.CLICredentials) *SelectionResolver {
	if loadCreds == nil {
		loadCreds = func() runtimeconfig.CLICredentials {
			return runtimeconfig.LoadCLICredentials()
		}
	}
	return &SelectionResolver{loadCreds: loadCreds}
}

func (r *SelectionResolver) Resolve(selection Selection) (ResolvedSelection, bool) {
	if strings.ToLower(strings.TrimSpace(selection.Mode)) != "cli" {
		return ResolvedSelection{}, false
	}
	provider := strings.TrimSpace(strings.ToLower(selection.Provider))
	model := strings.TrimSpace(selection.Model)
	if provider == "" || model == "" {
		return ResolvedSelection{}, false
	}

	creds := r.loadCreds()

	// matchProvider checks both the dynamic credential provider name and the
	// hardcoded known name. When credentials are empty (e.g. expired token),
	// creds.XXX.Provider is "", so the dynamic match fails. The hardcoded
	// fallback ensures the stored selection is still recognised.
	matchProvider := func(credProvider, knownName string) bool {
		return provider == credProvider || (credProvider == "" && provider == knownName)
	}

	switch {
	case matchProvider(creds.Codex.Provider, "codex"):
		headers := map[string]string{}
		if creds.Codex.AccountID != "" {
			headers["ChatGPT-Account-Id"] = creds.Codex.AccountID
		}
		baseURL := creds.Codex.BaseURL
		if baseURL == "" {
			baseURL = "https://chatgpt.com/backend-api/codex"
		}
		return ResolvedSelection{
			Provider: provider,
			Model:    model,
			APIKey:   creds.Codex.APIKey,
			BaseURL:  baseURL,
			Headers:  headers,
			Source:   string(creds.Codex.Source),
			Pinned:   true,
		}, true
	case matchProvider(creds.Claude.Provider, "anthropic"):
		baseURL := creds.Claude.BaseURL
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		return ResolvedSelection{
			Provider: provider,
			Model:    model,
			APIKey:   creds.Claude.APIKey,
			BaseURL:  baseURL,
			Source:   string(creds.Claude.Source),
			Pinned:   true,
		}, true
	case provider == "llama_server":
		return ResolvedSelection{
			Provider: "llama.cpp",
			Model:    model,
			BaseURL:  resolveLlamaServerBaseURL(runtimeconfig.DefaultEnvLookup),
			Source:   "llama_server",
			Pinned:   true,
		}, true
	default:
		return ResolvedSelection{}, false
	}
}

func resolveLlamaServerBaseURL(lookup runtimeconfig.EnvLookup) string {
	if lookup == nil {
		lookup = runtimeconfig.DefaultEnvLookup
	}
	if base, ok := lookup("LLAMA_SERVER_BASE_URL"); ok {
		base = strings.TrimSpace(base)
		if base != "" {
			return base
		}
	}
	if host, ok := lookup("LLAMA_SERVER_HOST"); ok {
		host = strings.TrimSpace(host)
		if host == "" {
			return ""
		}
		if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
			return host
		}
		return "http://" + host
	}
	return "http://127.0.0.1:8082/v1"
}
