package subscription

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeconfig "alex/internal/shared/config"
)

func TestParseModelListHandlesDataObjects(t *testing.T) {
	input := []byte(`{"data":[{"id":"model-a"},{"id":"model-b"}]}`)
	models, err := parseModelList(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(models) != 2 || models[0] != "model-a" || models[1] != "model-b" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestFetchProviderModelsUsesBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"model-x"}]}`))
	}))
	defer srv.Close()

	client := srv.Client()
	models, err := fetchProviderModels(context.Background(), client, fetchTarget{
		provider: "codex",
		baseURL:  srv.URL,
		apiKey:   "tok-abc",
	}, runtimeconfig.DefaultHTTPMaxResponse)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if gotAuth != "Bearer tok-abc" {
		t.Fatalf("expected bearer auth, got %q", gotAuth)
	}
	if len(models) != 1 || models[0] != "model-x" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestFetchProviderModelsUsesAnthropicOAuthHeaders(t *testing.T) {
	var gotVersion string
	var gotBeta string
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("anthropic-version")
		gotBeta = r.Header.Get("anthropic-beta")
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-3"}]}`))
	}))
	defer srv.Close()

	client := srv.Client()
	_, err := fetchProviderModels(context.Background(), client, fetchTarget{
		provider: "anthropic",
		baseURL:  srv.URL,
		apiKey:   "oauth-token",
	}, runtimeconfig.DefaultHTTPMaxResponse)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if gotAuth != "Bearer oauth-token" {
		t.Fatalf("expected oauth bearer auth, got %q", gotAuth)
	}
	if gotVersion == "" {
		t.Fatalf("expected anthropic-version header")
	}
	if !strings.Contains(gotBeta, "oauth-2025-04-20") {
		t.Fatalf("expected oauth beta header, got %q", gotBeta)
	}
}

func TestFetchProviderModelsUsesAntigravityEndpoint(t *testing.T) {
	var gotMethod, gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":{"gemini-3-pro-high":{},"nanobanana-pro":{}}}`))
	}))
	defer srv.Close()

	models, err := fetchProviderModels(context.Background(), srv.Client(), fetchTarget{
		provider: "antigravity",
		baseURL:  srv.URL,
		apiKey:   "tok-abc",
	}, runtimeconfig.DefaultHTTPMaxResponse)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("expected POST, got %q", gotMethod)
	}
	if gotPath != "/v1internal:fetchAvailableModels" {
		t.Fatalf("expected antigravity endpoint, got %q", gotPath)
	}
	if gotAuth != "Bearer tok-abc" {
		t.Fatalf("expected bearer auth, got %q", gotAuth)
	}
	if len(models) != 2 {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestParseOllamaModelList(t *testing.T) {
	input := []byte(`{"models":[{"name":"llama3:latest"},{"model":"phi3"},"mistral"]}`)
	models, err := parseOllamaModelList(input)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("expected 3 models, got %#v", models)
	}
	if models[0] != "llama3:latest" || models[1] != "mistral" || models[2] != "phi3" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestFetchOllamaModelsUsesTagsEndpoint(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:latest"}]}`))
	}))
	defer srv.Close()

	models, err := fetchOllamaModels(context.Background(), srv.Client(), srv.URL, runtimeconfig.DefaultHTTPMaxResponse)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if gotPath != "/api/tags" {
		t.Fatalf("expected /api/tags, got %q", gotPath)
	}
	if len(models) != 1 || models[0] != "llama3:latest" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCatalogServiceUsesCodexFallbackWithoutNetwork(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatalf("unexpected network request")
			return nil, nil
		}),
	}

	svc := NewCatalogService(func() runtimeconfig.CLICredentials {
		return runtimeconfig.CLICredentials{
			Codex: runtimeconfig.CLICredential{
				Provider: "codex",
				APIKey:   "tok-abc",
				Model:    "gpt-5.2-codex",
				BaseURL:  "https://chatgpt.com/backend-api/codex",
				Source:   runtimeconfig.SourceCodexCLI,
			},
		}
	}, client, 0)

	catalog := svc.Catalog(context.Background())
	if len(catalog.Providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(catalog.Providers))
	}
	got := catalog.Providers[0]
	if got.Error != "" {
		t.Fatalf("expected no error, got %q", got.Error)
	}
	if len(got.Models) == 0 || got.Models[0] == "" {
		t.Fatalf("expected fallback models, got %#v", got.Models)
	}
}
