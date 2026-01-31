package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	runtimeconfig "alex/internal/config"
)

func parseModelList(raw []byte) ([]string, error) {
	var payload any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal model list: %w", err)
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

type runtimeModelProvider struct {
	Provider string   `json:"provider"`
	Source   string   `json:"source"`
	BaseURL  string   `json:"base_url,omitempty"`
	Models   []string `json:"models,omitempty"`
	Error    string   `json:"error,omitempty"`
}

type modelFetchTarget struct {
	Provider      string
	BaseURL       string
	APIKey        string
	AccountID     string
	ClientVersion string
	Source        string
}

func fetchProviderModels(ctx context.Context, client *http.Client, target modelFetchTarget) ([]string, error) {
	endpoint := strings.TrimRight(target.BaseURL, "/") + "/models"
	if target.Provider == "codex" && strings.TrimSpace(target.ClientVersion) != "" {
		separator := "?"
		if strings.Contains(endpoint, "?") {
			separator = "&"
		}
		endpoint = endpoint + separator + "client_version=" + url.QueryEscape(target.ClientVersion)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build model list request: %w", err)
	}

	if target.AccountID != "" {
		req.Header.Set("ChatGPT-Account-Id", target.AccountID)
	}
	if target.Provider == "anthropic" || target.Provider == "claude" {
		if isAnthropicOAuthToken(target.APIKey) {
			req.Header.Set("Authorization", "Bearer "+target.APIKey)
			req.Header.Set("anthropic-beta", "oauth-2025-04-20")
		} else if target.APIKey != "" {
			req.Header.Set("x-api-key", target.APIKey)
		}
		req.Header.Set("anthropic-version", "2023-06-01")
	} else if target.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+target.APIKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch model list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model list request failed: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read model list response: %w", err)
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

func listRuntimeModels(ctx context.Context, creds runtimeconfig.CLICredentials, client *http.Client) []runtimeModelProvider {
	var targets []runtimeModelProvider

	if creds.Codex.APIKey != "" {
		targets = append(targets, runtimeModelProvider{
			Provider: creds.Codex.Provider,
			Source:   string(creds.Codex.Source),
			BaseURL:  creds.Codex.BaseURL,
		})
	}
	if creds.Antigravity.APIKey != "" {
		targets = append(targets, runtimeModelProvider{
			Provider: creds.Antigravity.Provider,
			Source:   string(creds.Antigravity.Source),
			BaseURL:  creds.Antigravity.BaseURL,
		})
	}
	if creds.Claude.APIKey != "" {
		baseURL := creds.Claude.BaseURL
		if strings.TrimSpace(baseURL) == "" {
			baseURL = "https://api.anthropic.com/v1"
		}
		targets = append(targets, runtimeModelProvider{
			Provider: creds.Claude.Provider,
			Source:   string(creds.Claude.Source),
			BaseURL:  baseURL,
		})
	}

	for i := range targets {
		target := &targets[i]
		if target.Provider == "codex" && target.Source == string(runtimeconfig.SourceCodexCLI) {
			target.Models = codexCLIFallbackModels(creds.Codex.Model)
			continue
		}
		models, err := fetchProviderModels(ctx, client, modelFetchTarget{
			Provider:      target.Provider,
			BaseURL:       target.BaseURL,
			APIKey:        pickAPIKey(creds, target.Provider),
			AccountID:     pickAccountID(creds, target.Provider),
			ClientVersion: cliClientVersion(target.Source),
			Source:        target.Source,
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

func cliClientVersion(source string) string {
	if source != string(runtimeconfig.SourceCodexCLI) {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "version.json"))
	if err != nil {
		return ""
	}
	var payload struct {
		LatestVersion string `json:"latest_version"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.LatestVersion)
}

func codexCLIFallbackModels(cliModel string) []string {
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

func containsModel(models []string, model string) bool {
	for _, candidate := range models {
		if candidate == model {
			return true
		}
	}
	return false
}
