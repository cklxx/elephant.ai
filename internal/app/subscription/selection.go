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
	switch provider {
	case creds.Codex.Provider:
		if creds.Codex.APIKey == "" {
			return ResolvedSelection{}, false
		}
		headers := map[string]string{}
		if creds.Codex.AccountID != "" {
			headers["ChatGPT-Account-Id"] = creds.Codex.AccountID
		}
		return ResolvedSelection{
			Provider: provider,
			Model:    model,
			APIKey:   creds.Codex.APIKey,
			BaseURL:  creds.Codex.BaseURL,
			Headers:  headers,
			Source:   string(creds.Codex.Source),
			Pinned:   true,
		}, true
	case creds.Claude.Provider:
		if creds.Claude.APIKey == "" {
			return ResolvedSelection{}, false
		}
		return ResolvedSelection{
			Provider: provider,
			Model:    model,
			APIKey:   creds.Claude.APIKey,
			BaseURL:  creds.Claude.BaseURL,
			Source:   string(creds.Claude.Source),
			Pinned:   true,
		}, true
	case creds.Antigravity.Provider:
		if creds.Antigravity.APIKey == "" {
			return ResolvedSelection{}, false
		}
		return ResolvedSelection{
			Provider: provider,
			Model:    model,
			APIKey:   creds.Antigravity.APIKey,
			BaseURL:  creds.Antigravity.BaseURL,
			Source:   string(creds.Antigravity.Source),
			Pinned:   true,
		}, true
	case "ollama":
		return ResolvedSelection{
			Provider: provider,
			Model:    model,
			BaseURL:  resolveOllamaBaseURL(runtimeconfig.DefaultEnvLookup),
			Source:   "ollama",
			Pinned:   true,
		}, true
	default:
		return ResolvedSelection{}, false
	}
}

func resolveOllamaBaseURL(lookup runtimeconfig.EnvLookup) string {
	if lookup == nil {
		lookup = runtimeconfig.DefaultEnvLookup
	}
	if base, ok := lookup("OLLAMA_BASE_URL"); ok {
		base = strings.TrimSpace(base)
		if base != "" {
			return base
		}
	}
	if host, ok := lookup("OLLAMA_HOST"); ok {
		host = strings.TrimSpace(host)
		if host == "" {
			return ""
		}
		if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
			return host
		}
		return "http://" + host
	}
	return ""
}
