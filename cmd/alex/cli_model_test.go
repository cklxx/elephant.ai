package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimeconfig "alex/internal/config"
)

func TestModelListShowsProviders(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-5.2-codex"},{"id":"gpt-5.2"}]}`))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	if err := listModelsFrom(&buf, runtimeconfig.CLICredentials{
		Codex: runtimeconfig.CLICredential{
			Provider: "codex",
			APIKey:   "tok-abc",
			BaseURL:  srv.URL,
			Source:   runtimeconfig.SourceCodexCLI,
		},
	}); err != nil {
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
	if err := listModelsFrom(&buf, runtimeconfig.CLICredentials{}); err != nil {
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
	if err := listModelsFrom(&buf, runtimeconfig.CLICredentials{
		Antigravity: runtimeconfig.CLICredential{
			Provider: "antigravity",
			APIKey:   "expired-token",
			BaseURL:  srv.URL,
			Source:   runtimeconfig.SourceAntigravityCLI,
		},
	}); err != nil {
		t.Fatalf("listModels error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "antigravity") {
		t.Fatalf("expected antigravity provider in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Error:") {
		t.Fatalf("expected error in output, got:\n%s", out)
	}
}

func TestUseModelSetsOverrides(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	overridesFile := filepath.Join(tmp, "overrides.yaml")

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

	data, err := os.ReadFile(overridesFile)
	if err != nil {
		t.Fatalf("read overrides: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "codex") {
		t.Fatalf("expected codex in overrides, got:\n%s", content)
	}
	if !strings.Contains(content, "gpt-5.2-codex") {
		t.Fatalf("expected model in overrides, got:\n%s", content)
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

func TestClearModelRemovesOverrides(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	overridesFile := filepath.Join(tmp, "overrides.yaml")

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

	data, err := os.ReadFile(overridesFile)
	if err != nil {
		t.Fatalf("read overrides: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "codex") {
		t.Fatalf("expected codex to be cleared, got:\n%s", content)
	}
	if strings.Contains(content, "gpt-5.2") {
		t.Fatalf("expected model to be cleared, got:\n%s", content)
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
		provider string
		wantOK   bool
	}{
		{"codex", true},
		{"anthropic", true},
		{"antigravity", true},
		{"ollama", true},
		{"unknown", false},
	}

	for _, tt := range tests {
		cred, ok := matchCredential(creds, tt.provider)
		if ok != tt.wantOK {
			t.Errorf("matchCredential(%q): got ok=%v, want %v", tt.provider, ok, tt.wantOK)
		}
		if ok && cred.Provider != tt.provider {
			t.Errorf("matchCredential(%q): got provider=%q", tt.provider, cred.Provider)
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
