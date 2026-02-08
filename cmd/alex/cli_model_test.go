package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
)

func TestModelListShowsProviders(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.2-codex"},{"id":"gpt-5.2"}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	if err := listModelsFromWith(&buf, runtimeconfig.CLICredentials{
		Codex: runtimeconfig.CLICredential{
			Provider: "codex",
			APIKey:   "tok-abc",
			BaseURL:  srv.URL,
			Source:   runtimeconfig.SourceCodexCLI,
		},
	}, srv.Client(), func(context.Context) (subscription.LlamaServerTarget, bool) {
		return subscription.LlamaServerTarget{}, false
	}, nil); err != nil {
		t.Fatalf("listModels error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "codex") {
		t.Fatalf("expected codex provider in output, got:\n%s", out)
	}
	if !strings.Contains(out, "gpt-5.2-codex") {
		t.Fatalf("expected model in output, got:\n%s", out)
	}
}

func TestModelListShowsEmpty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := listModelsFromWith(&buf, runtimeconfig.CLICredentials{}, &http.Client{}, func(context.Context) (subscription.LlamaServerTarget, bool) {
		return subscription.LlamaServerTarget{}, false
	}, nil); err != nil {
		t.Fatalf("listModels error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "未发现") {
		t.Fatalf("expected empty message, got:\n%s", out)
	}
}

func TestModelListShowsErrors(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	var buf bytes.Buffer
	if err := listModelsFromWith(&buf, runtimeconfig.CLICredentials{
		Claude: runtimeconfig.CLICredential{
			Provider: "anthropic",
			APIKey:   "expired-token",
			BaseURL:  srv.URL,
			Source:   runtimeconfig.SourceEnv,
		},
	}, srv.Client(), func(context.Context) (subscription.LlamaServerTarget, bool) {
		return subscription.LlamaServerTarget{}, false
	}, nil); err != nil {
		t.Fatalf("listModels error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "anthropic") {
		t.Fatalf("expected anthropic provider in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Error:") {
		t.Fatalf("expected error in output, got:\n%s", out)
	}
}

func TestModelListIncludesLlamaServerWhenAvailable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("expected /v1/models path, got %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"llama-3.2-local"}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	if err := listModelsFromWith(&buf, runtimeconfig.CLICredentials{}, srv.Client(), func(context.Context) (subscription.LlamaServerTarget, bool) {
		return subscription.LlamaServerTarget{
			BaseURL: srv.URL,
			Source:  "llama_server",
		}, true
	}, nil); err != nil {
		t.Fatalf("listModels error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "llama_server (llama_server)") {
		t.Fatalf("expected llama server provider in output, got:\n%s", out)
	}
	if !strings.Contains(out, "llama-3.2-local") {
		t.Fatalf("expected llama server model in output, got:\n%s", out)
	}
}

func TestUseModelPersistsSelectionWithoutYAML(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	overridesFile := filepath.Join(tmp, "overrides.yaml")
	selectionFile := filepath.Join(tmp, "llm_selection.json")

	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return overridesFile, true
		}
		return "", false
	}

	err := useModelWith(
		&bytes.Buffer{},
		"codex/gpt-5.2-codex",
		runtimeconfig.CLICredentials{
			Codex: runtimeconfig.CLICredential{
				Provider: "codex",
				APIKey:   "tok-abc",
				BaseURL:  "https://chatgpt.com/backend-api/codex",
				Source:   runtimeconfig.SourceCodexCLI,
			},
		},
		envLookup,
	)
	if err != nil {
		t.Fatalf("useModel error: %v", err)
	}

	if _, err := os.Stat(overridesFile); err == nil {
		t.Fatalf("expected managed overrides yaml to remain untouched")
	}

	data, err := os.ReadFile(selectionFile)
	if err != nil {
		t.Fatalf("read selection store: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "codex") {
		t.Fatalf("expected codex in selection store, got:\n%s", content)
	}
	if !strings.Contains(content, "gpt-5.2-codex") {
		t.Fatalf("expected model in selection store, got:\n%s", content)
	}
	if got := subscription.ResolveSelectionStorePath(envLookup, nil); got != selectionFile {
		t.Fatalf("expected selection store path %q, got %q", selectionFile, got)
	}
}

func TestUseModelRejectsInvalidFormat(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := useModelWith(&buf, "codex-gpt-5.2", runtimeconfig.CLICredentials{}, func(string) (string, bool) { return "", false })
	if err == nil {
		t.Fatalf("expected error for invalid format")
	}
}

func TestUseModelRejectsMissingCredential(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := useModelWith(&buf, "codex/gpt-5.2", runtimeconfig.CLICredentials{}, func(string) (string, bool) { return "", false })
	if err == nil {
		t.Fatalf("expected error for missing credential")
	}
	if !strings.Contains(err.Error(), "no subscription credential") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClearModelRemovesSelection(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	overridesFile := filepath.Join(tmp, "overrides.yaml")
	selectionFile := filepath.Join(tmp, "llm_selection.json")

	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return overridesFile, true
		}
		return "", false
	}

	// First set some overrides
	if err := useModelWith(
		&bytes.Buffer{},
		"codex/gpt-5.2-codex",
		runtimeconfig.CLICredentials{
			Codex: runtimeconfig.CLICredential{
				Provider: "codex",
				APIKey:   "tok-abc",
				BaseURL:  "https://chatgpt.com/backend-api/codex",
				Source:   runtimeconfig.SourceCodexCLI,
			},
		},
		envLookup,
	); err != nil {
		t.Fatalf("useModel error: %v", err)
	}

	// Then clear
	var buf bytes.Buffer
	if err := clearModelWith(&buf, envLookup); err != nil {
		t.Fatalf("clearModel error: %v", err)
	}

	if _, err := os.Stat(selectionFile); err == nil {
		t.Fatalf("expected selection store to be removed after clear")
	}

	output := buf.String()
	if !strings.Contains(output, "cleared") {
		t.Fatalf("expected clear message, got:\n%s", output)
	}
}

func TestMatchCredentialFindsProviders(t *testing.T) {
	t.Parallel()
	creds := runtimeconfig.CLICredentials{
		Codex: runtimeconfig.CLICredential{
			Provider: "codex",
			APIKey:   "tok-abc",
		},
		Claude: runtimeconfig.CLICredential{
			Provider: "anthropic",
			APIKey:   "sk-abc",
		},
		Antigravity: runtimeconfig.CLICredential{
			Provider: "antigravity",
			APIKey:   "ag-abc",
		},
	}

	tests := []struct {
		provider     string
		wantOK       bool
		wantProvider string
	}{
		{"codex", true, "codex"},
		{"anthropic", true, "anthropic"},
		{"antigravity", true, "antigravity"},
		{"ollama", true, "ollama"},
		{"llama.cpp", false, ""},
		{"llama_server", true, "llama_server"},
		{"unknown", false, ""},
	}

	for _, tt := range tests {
		cred, ok := matchCredential(creds, tt.provider)
		if ok != tt.wantOK {
			t.Errorf("matchCredential(%q): got ok=%v, want %v", tt.provider, ok, tt.wantOK)
		}
		if ok && cred.Provider != tt.wantProvider {
			t.Errorf("matchCredential(%q): got provider=%q want=%q", tt.provider, cred.Provider, tt.wantProvider)
		}
	}
}

func TestExecuteModelCommandHelp(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := executeModelCommand([]string{"help"}, &buf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "alex model") {
		t.Fatalf("expected usage output, got:\n%s", buf.String())
	}
}

func TestExecuteModelCommandUnknownSubcommand(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	err := executeModelCommand([]string{"foobar"}, &buf)
	if err == nil {
		t.Fatalf("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "foobar") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestModelListIncludesOllamaWhenAvailable(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("expected /api/tags path, got %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"models":[{"name":"llama3:latest"},{"name":"phi3"}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	if err := listModelsFromWith(&buf, runtimeconfig.CLICredentials{}, srv.Client(),
		func(context.Context) (subscription.LlamaServerTarget, bool) {
			return subscription.LlamaServerTarget{}, false
		},
		func(context.Context) (subscription.OllamaTarget, bool) {
			return subscription.OllamaTarget{
				BaseURL: srv.URL,
				Source:  "ollama",
			}, true
		},
	); err != nil {
		t.Fatalf("listModels error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "ollama (ollama)") {
		t.Fatalf("expected ollama provider in output, got:\n%s", out)
	}
	if !strings.Contains(out, "llama3:latest") {
		t.Fatalf("expected ollama model in output, got:\n%s", out)
	}
}

func TestMatchCredentialAntigravity(t *testing.T) {
	t.Parallel()

	creds := runtimeconfig.CLICredentials{
		Antigravity: runtimeconfig.CLICredential{
			Provider: "antigravity",
			APIKey:   "ag-abc",
			BaseURL:  "https://cloudcode-pa.googleapis.com",
			Source:   runtimeconfig.SourceAntigravityIDE,
		},
	}

	cred, ok := matchCredential(creds, "antigravity")
	if !ok {
		t.Fatal("expected antigravity credential to match")
	}
	if cred.Provider != "antigravity" {
		t.Fatalf("expected provider antigravity, got %q", cred.Provider)
	}

	// No API key → should not match.
	_, ok = matchCredential(runtimeconfig.CLICredentials{
		Antigravity: runtimeconfig.CLICredential{
			Provider: "antigravity",
		},
	}, "antigravity")
	if ok {
		t.Fatal("expected antigravity without API key to not match")
	}
}
