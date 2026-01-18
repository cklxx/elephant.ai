package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	runtimeconfig "alex/internal/config"
)

func parseModelList(raw []byte) ([]string, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}

	models := map[string]struct{}{}
	if obj, ok := payload.(map[string]any); ok {
		if list, ok := obj["data"]; ok {
			extractModelIDs(list, models)
		}
		if list, ok := obj["models"]; ok {
			extractModelIDs(list, models)
		}
	}

	out := make([]string, 0, len(models))
	for id := range models {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func extractModelIDs(value any, out map[string]struct{}) {
	list, ok := value.([]any)
	if !ok {
		return
	}
	for _, item := range list {
		switch v := item.(type) {
		case string:
			if v != "" {
				out[v] = struct{}{}
			}
		case map[string]any:
			if id, ok := v["id"].(string); ok && id != "" {
				out[id] = struct{}{}
				continue
			}
			if id, ok := v["model"].(string); ok && id != "" {
				out[id] = struct{}{}
			}
		}
	}
}

type CatalogProvider struct {
	Provider string   `json:"provider"`
	Source   string   `json:"source"`
	BaseURL  string   `json:"base_url,omitempty"`
	Models   []string `json:"models,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type Catalog struct {
	Providers []CatalogProvider `json:"providers"`
}

type CatalogService struct {
	loadCreds func() runtimeconfig.CLICredentials
	client    *http.Client
	ttl       time.Duration
	mu        sync.Mutex
	cached    Catalog
	cachedAt  time.Time
}

func NewCatalogService(loadCreds func() runtimeconfig.CLICredentials, client *http.Client, ttl time.Duration) *CatalogService {
	if loadCreds == nil {
		loadCreds = func() runtimeconfig.CLICredentials {
			return runtimeconfig.LoadCLICredentials()
		}
	}
	return &CatalogService{
		loadCreds: loadCreds,
		client:    client,
		ttl:       ttl,
	}
}

func (s *CatalogService) Catalog(ctx context.Context) Catalog {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ttl > 0 && time.Since(s.cachedAt) < s.ttl {
		return s.cached
	}

	creds := s.loadCreds()
	providers := listProviders(ctx, creds, s.client)
	catalog := Catalog{Providers: providers}

	s.cached = catalog
	s.cachedAt = time.Now()
	return catalog
}

func listProviders(ctx context.Context, creds runtimeconfig.CLICredentials, client *http.Client) []CatalogProvider {
	var targets []CatalogProvider

	if creds.Codex.APIKey != "" {
		targets = append(targets, CatalogProvider{
			Provider: creds.Codex.Provider,
			Source:   string(creds.Codex.Source),
			BaseURL:  creds.Codex.BaseURL,
		})
	}
	if creds.Antigravity.APIKey != "" {
		targets = append(targets, CatalogProvider{
			Provider: creds.Antigravity.Provider,
			Source:   string(creds.Antigravity.Source),
			BaseURL:  creds.Antigravity.BaseURL,
		})
	}
	if creds.Claude.APIKey != "" {
		baseURL := strings.TrimSpace(creds.Claude.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		targets = append(targets, CatalogProvider{
			Provider: creds.Claude.Provider,
			Source:   string(creds.Claude.Source),
			BaseURL:  baseURL,
		})
	}

	for i := range targets {
		target := &targets[i]
		if target.Provider == "codex" && target.Source == string(runtimeconfig.SourceCodexCLI) {
			target.Models = codexFallbackModels(creds.Codex.Model)
			continue
		}
		models, err := fetchProviderModels(ctx, client, fetchTarget{
			provider:  target.Provider,
			baseURL:   target.BaseURL,
			apiKey:    pickAPIKey(creds, target.Provider),
			accountID: pickAccountID(creds, target.Provider),
		})
		if err != nil {
			target.Error = err.Error()
			continue
		}
		target.Models = models
	}

	return targets
}

func pickAPIKey(creds runtimeconfig.CLICredentials, provider string) string {
	switch provider {
	case creds.Codex.Provider:
		return creds.Codex.APIKey
	case creds.Antigravity.Provider:
		return creds.Antigravity.APIKey
	case creds.Claude.Provider:
		return creds.Claude.APIKey
	default:
		return ""
	}
}

func pickAccountID(creds runtimeconfig.CLICredentials, provider string) string {
	switch provider {
	case creds.Codex.Provider:
		return creds.Codex.AccountID
	default:
		return ""
	}
}

func codexFallbackModels(cliModel string) []string {
	models := []string{
		"gpt-5.1-codex-max",
		"gpt-5.1-codex-mini",
		"gpt-5.2",
		"gpt-5.2-codex",
	}
	if cliModel != "" && !containsModel(models, cliModel) {
		models = append(models, cliModel)
	}
	sort.Strings(models)
	return models
}

func containsModel(models []string, value string) bool {
	for _, model := range models {
		if model == value {
			return true
		}
	}
	return false
}

type fetchTarget struct {
	provider  string
	baseURL   string
	apiKey    string
	accountID string
}

func fetchProviderModels(ctx context.Context, client *http.Client, target fetchTarget) ([]string, error) {
	endpoint := strings.TrimRight(target.baseURL, "/") + "/models"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	if target.accountID != "" {
		req.Header.Set("ChatGPT-Account-Id", target.accountID)
	}
	if target.provider == "anthropic" || target.provider == "claude" {
		if isAnthropicOAuthToken(target.apiKey) {
			req.Header.Set("Authorization", "Bearer "+target.apiKey)
			req.Header.Set("anthropic-beta", "oauth-2025-04-20")
		} else if target.apiKey != "" {
			req.Header.Set("x-api-key", target.apiKey)
		}
		req.Header.Set("anthropic-version", "2023-06-01")
	} else if target.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+target.apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model list request failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseModelList(body)
}

func isAnthropicOAuthToken(token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	return !strings.HasPrefix(strings.ToLower(token), "sk-")
}
