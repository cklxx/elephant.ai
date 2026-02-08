package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/infra/httpclient"
	runtimeconfig "alex/internal/shared/config"
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
	Provider          string                `json:"provider"`
	DisplayName       string                `json:"display_name,omitempty"`
	Source            string                `json:"source"`
	AuthMode          string                `json:"auth_mode,omitempty"`
	BaseURL           string                `json:"base_url,omitempty"`
	Models            []string              `json:"models,omitempty"`
	DefaultModel      string                `json:"default_model,omitempty"`
	RecommendedModels []ModelRecommendation `json:"recommended_models,omitempty"`
	Selectable        bool                  `json:"selectable"`
	SetupHint         string                `json:"setup_hint,omitempty"`
	Error             string                `json:"error,omitempty"`
}

type Catalog struct {
	Providers []CatalogProvider `json:"providers"`
}

type LlamaServerTarget struct {
	BaseURL string
	Source  string
}

type CatalogOption func(*CatalogService)

func WithLlamaServerTargetResolver(resolver func(context.Context) (LlamaServerTarget, bool)) CatalogOption {
	return func(service *CatalogService) {
		if resolver != nil {
			service.llamaServerResolver = resolver
		}
	}
}

// WithMaxResponseBytes sets the response size limit for provider model fetches.
func WithMaxResponseBytes(limit int) CatalogOption {
	return func(service *CatalogService) {
		if limit > 0 {
			service.maxResponseBytes = limit
		}
	}
}

type CatalogService struct {
	loadCreds           func() runtimeconfig.CLICredentials
	client              *http.Client
	ttl                 time.Duration
	mu                  sync.Mutex
	cached              Catalog
	cachedAt            time.Time
	llamaServerResolver func(context.Context) (LlamaServerTarget, bool)
	maxResponseBytes    int
}

func NewCatalogService(loadCreds func() runtimeconfig.CLICredentials, client *http.Client, ttl time.Duration, opts ...CatalogOption) *CatalogService {
	if loadCreds == nil {
		loadCreds = func() runtimeconfig.CLICredentials {
			return runtimeconfig.LoadCLICredentials()
		}
	}
	service := &CatalogService{
		loadCreds:        loadCreds,
		client:           client,
		ttl:              ttl,
		maxResponseBytes: runtimeconfig.DefaultHTTPMaxResponse,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

func (s *CatalogService) Catalog(ctx context.Context) Catalog {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ttl > 0 && time.Since(s.cachedAt) < s.ttl {
		return s.cached
	}

	creds := s.loadCreds()
	providers := listProviders(ctx, creds, s.client, s.maxResponseBytes)
	if s.llamaServerResolver != nil {
		if target, ok := s.llamaServerResolver(ctx); ok {
			if provider, ok := buildLlamaServerProvider(ctx, s.client, target, s.maxResponseBytes); ok {
				providers = append(providers, provider)
			}
		}
	}
	catalog := Catalog{Providers: providers}

	s.cached = catalog
	s.cachedAt = time.Now()
	return catalog
}

func listProviders(ctx context.Context, creds runtimeconfig.CLICredentials, client *http.Client, maxResponseBytes int) []CatalogProvider {
	var targets []CatalogProvider

	if creds.Codex.APIKey != "" {
		target := CatalogProvider{
			Provider: creds.Codex.Provider,
			Source:   string(creds.Codex.Source),
			BaseURL:  creds.Codex.BaseURL,
		}
		applyCatalogProviderPreset(&target)
		targets = append(targets, target)
	}
	if creds.Claude.APIKey != "" {
		baseURL := strings.TrimSpace(creds.Claude.BaseURL)
		if baseURL == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		target := CatalogProvider{
			Provider: creds.Claude.Provider,
			Source:   string(creds.Claude.Source),
			BaseURL:  baseURL,
		}
		applyCatalogProviderPreset(&target)
		targets = append(targets, target)
	}

	for i := range targets {
		target := &targets[i]
		if target.Provider == "codex" && target.Source == string(runtimeconfig.SourceCodexCLI) {
			target.Models = codexFallbackModels(creds.Codex.Model)
			target.DefaultModel = pickCatalogDefaultModel(*target)
			continue
		}
		models, err := fetchProviderModels(ctx, client, fetchTarget{
			provider:  target.Provider,
			baseURL:   target.BaseURL,
			apiKey:    pickAPIKey(creds, target.Provider),
			accountID: pickAccountID(creds, target.Provider),
		}, maxResponseBytes)
		if err != nil {
			target.Error = err.Error()
			if len(target.Models) == 0 {
				target.Models = recommendationIDs(target.RecommendedModels)
			}
			target.DefaultModel = pickCatalogDefaultModel(*target)
			continue
		}
		target.Models = models
		target.DefaultModel = pickCatalogDefaultModel(*target)
	}

	return targets
}

func pickAPIKey(creds runtimeconfig.CLICredentials, provider string) string {
	switch provider {
	case creds.Codex.Provider:
		return creds.Codex.APIKey
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

const defaultLlamaServerBaseURL = "http://127.0.0.1:8082/v1"

func buildLlamaServerProvider(ctx context.Context, client *http.Client, target LlamaServerTarget, maxResponseBytes int) (CatalogProvider, bool) {
	baseURL := normalizeLlamaServerBaseURL(target.BaseURL)
	source := strings.TrimSpace(target.Source)
	if source == "" {
		source = "llama_server"
	}
	provider := CatalogProvider{
		Provider: "llama_server",
		Source:   source,
		BaseURL:  baseURL,
	}
	applyCatalogProviderPreset(&provider)

	llamaCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	models, err := fetchLlamaServerModels(llamaCtx, client, baseURL, maxResponseBytes)
	if err != nil {
		return CatalogProvider{}, false
	}
	provider.Models = models
	provider.DefaultModel = pickCatalogDefaultModel(provider)
	return provider, true
}

func normalizeLlamaServerBaseURL(baseURL string) string {
	base := strings.TrimSpace(baseURL)
	if base == "" {
		base = defaultLlamaServerBaseURL
	}
	base = strings.TrimRight(base, "/")
	if strings.HasSuffix(base, "/v1") {
		return base
	}
	return base + "/v1"
}

func fetchLlamaServerModels(ctx context.Context, client *http.Client, baseURL string, maxResponseBytes int) ([]string, error) {
	endpoint := llamaServerModelsEndpoint(baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llama_server model list request failed: %s", resp.Status)
	}

	body, err := httpclient.ReadAllWithLimit(resp.Body, int64(maxResponseBytes))
	if err != nil {
		if httpclient.IsResponseTooLarge(err) {
			return nil, fmt.Errorf("llama_server model list response exceeds %d bytes", maxResponseBytes)
		}
		return nil, err
	}

	return parseModelList(body)
}

func llamaServerModelsEndpoint(baseURL string) string {
	return normalizeLlamaServerBaseURL(baseURL) + "/models"
}

type fetchTarget struct {
	provider  string
	baseURL   string
	apiKey    string
	accountID string
}

func fetchProviderModels(ctx context.Context, client *http.Client, target fetchTarget, maxResponseBytes int) ([]string, error) {
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

	body, err := httpclient.ReadAllWithLimit(resp.Body, int64(maxResponseBytes))
	if err != nil {
		if httpclient.IsResponseTooLarge(err) {
			return nil, fmt.Errorf("model list response exceeds %d bytes", maxResponseBytes)
		}
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
