package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	runtimeconfig "alex/internal/config"
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
	models, err := fetchProviderModels(context.Background(), client, modelFetchTarget{
		Provider: "codex",
		BaseURL:  srv.URL,
		APIKey:   "tok-abc",
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
	_, err := fetchProviderModels(context.Background(), client, modelFetchTarget{
		Provider: "anthropic",
		BaseURL:  srv.URL,
		APIKey:   "oauth-token",
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

func TestFetchProviderModelsUsesCodexModelsPath(t *testing.T) {
	var gotPath string
	var gotClientVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotClientVersion = r.URL.Query().Get("client_version")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"codex-model"}]}`))
	}))
	defer srv.Close()

	client := srv.Client()
	_, err := fetchProviderModels(context.Background(), client, modelFetchTarget{
		Provider:      "codex",
		BaseURL:       srv.URL + "/codex",
		APIKey:        "tok-abc",
		ClientVersion: "0.86.0",
	}, runtimeconfig.DefaultHTTPMaxResponse)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if gotPath != "/codex/models" {
		t.Fatalf("expected /codex/models path, got %q", gotPath)
	}
	if gotClientVersion != "0.86.0" {
		t.Fatalf("expected client_version query param, got %q", gotClientVersion)
	}
}

func TestFetchProviderModelsSetsChatGPTAccountIDHeader(t *testing.T) {
	var gotAccount string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccount = r.Header.Get("ChatGPT-Account-Id")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"codex-model"}]}`))
	}))
	defer srv.Close()

	client := srv.Client()
	_, err := fetchProviderModels(context.Background(), client, modelFetchTarget{
		Provider:  "codex",
		BaseURL:   srv.URL,
		APIKey:    "tok-abc",
		AccountID: "acct-123",
	}, runtimeconfig.DefaultHTTPMaxResponse)
	if err != nil {
		t.Fatalf("fetch error: %v", err)
	}
	if gotAccount != "acct-123" {
		t.Fatalf("expected ChatGPT-Account-Id header, got %q", gotAccount)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestListRuntimeModelsUsesCodexCLIFallback(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatalf("unexpected network request")
			return nil, nil
		}),
	}

	creds := runtimeconfig.CLICredentials{
		Codex: runtimeconfig.CLICredential{
			Provider: "codex",
			APIKey:   "tok-abc",
			Model:    "gpt-5.2-codex",
			BaseURL:  "https://chatgpt.com/backend-api/codex",
			Source:   runtimeconfig.SourceCodexCLI,
		},
	}

	providers := listRuntimeModels(context.Background(), creds, client, runtimeconfig.DefaultHTTPMaxResponse)
	if len(providers) != 1 {
		t.Fatalf("expected one provider, got %d", len(providers))
	}
	got := providers[0]
	if got.Error != "" {
		t.Fatalf("expected no error, got %q", got.Error)
	}
	expected := []string{
		"gpt-5.1-codex-max",
		"gpt-5.1-codex-mini",
		"gpt-5.2",
		"gpt-5.2-codex",
	}
	if len(got.Models) != len(expected) {
		t.Fatalf("expected %d models, got %d: %#v", len(expected), len(got.Models), got.Models)
	}
	for i, model := range expected {
		if got.Models[i] != model {
			t.Fatalf("expected model %q at index %d, got %q", model, i, got.Models[i])
		}
	}
}
