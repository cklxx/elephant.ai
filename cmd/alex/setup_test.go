package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimeconfig "alex/internal/shared/config"
	"gopkg.in/yaml.v3"
)

func TestExecuteSetupCommandWithYAML(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	overridesFile := filepath.Join(tmp, "config.yaml")
	onboardingFile := filepath.Join(tmp, "onboarding_state.json")
	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return overridesFile, true
		}
		return "", false
	}

	var out bytes.Buffer
	if err := executeSetupCommandWith([]string{"--use-yaml"}, strings.NewReader(""), &out, runtimeconfig.CLICredentials{}, envLookup); err != nil {
		t.Fatalf("executeSetupCommandWith error: %v", err)
	}
	data, err := os.ReadFile(onboardingFile)
	if err != nil {
		t.Fatalf("read onboarding file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, `"used_source": "yaml"`) {
		t.Fatalf("expected yaml source in onboarding state, got:\n%s", content)
	}
}

func TestExecuteSetupCommandWithExplicitModel(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	overridesFile := filepath.Join(tmp, "config.yaml")
	selectionFile := filepath.Join(tmp, "llm_selection.json")
	onboardingFile := filepath.Join(tmp, "onboarding_state.json")
	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return overridesFile, true
		}
		return "", false
	}
	creds := runtimeconfig.CLICredentials{
		Codex: runtimeconfig.CLICredential{
			Provider: "codex",
			APIKey:   "tok-abc",
			BaseURL:  "https://chatgpt.com/backend-api/codex",
			Source:   runtimeconfig.SourceCodexCLI,
		},
	}

	var out bytes.Buffer
	if err := executeSetupCommandWith(
		[]string{"--provider", "codex", "--model", "gpt-5.2-codex"},
		strings.NewReader(""),
		&out,
		creds,
		envLookup,
	); err != nil {
		t.Fatalf("executeSetupCommandWith error: %v", err)
	}
	if _, err := os.Stat(selectionFile); err != nil {
		t.Fatalf("expected selection file, got err=%v", err)
	}
	if _, err := os.Stat(onboardingFile); err != nil {
		t.Fatalf("expected onboarding file, got err=%v", err)
	}
}

func TestExecuteSetupCommandWithProviderAPIKeyDefaults(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	overridesFile := filepath.Join(tmp, "config.yaml")
	onboardingFile := filepath.Join(tmp, "onboarding_state.json")
	selectionFile := filepath.Join(tmp, "llm_selection.json")
	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return overridesFile, true
		}
		return "", false
	}

	var out bytes.Buffer
	err := executeSetupCommandWith(
		[]string{"--provider", "openai", "--api-key", "sk-test-openai"},
		strings.NewReader(""),
		&out,
		runtimeconfig.CLICredentials{},
		envLookup,
	)
	if err != nil {
		t.Fatalf("executeSetupCommandWith error: %v", err)
	}

	overrides, err := loadManagedOverrides(envLookup)
	if err != nil {
		t.Fatalf("load managed overrides: %v", err)
	}
	if overrides.LLMProvider == nil || *overrides.LLMProvider != "openai" {
		t.Fatalf("expected llm_provider=openai, got %#v", overrides.LLMProvider)
	}
	if overrides.LLMModel == nil || *overrides.LLMModel == "" {
		t.Fatalf("expected default llm_model, got %#v", overrides.LLMModel)
	}
	if overrides.APIKey == nil || *overrides.APIKey != "sk-test-openai" {
		t.Fatalf("expected api_key override, got %#v", overrides.APIKey)
	}
	if overrides.BaseURL == nil || *overrides.BaseURL == "" {
		t.Fatalf("expected base_url override, got %#v", overrides.BaseURL)
	}

	if _, err := os.Stat(onboardingFile); err != nil {
		t.Fatalf("expected onboarding file, got err=%v", err)
	}
	if _, err := os.Stat(selectionFile); !os.IsNotExist(err) {
		t.Fatalf("expected selection file to be absent, err=%v", err)
	}
}

func TestExecuteSetupCommandWithLarkRuntimeConfig(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	configFile := filepath.Join(tmp, "config.yaml")
	selectionFile := filepath.Join(tmp, "llm_selection.json")
	onboardingFile := filepath.Join(tmp, "onboarding_state.json")
	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return configFile, true
		}
		return "", false
	}
	creds := runtimeconfig.CLICredentials{
		Codex: runtimeconfig.CLICredential{
			Provider: "codex",
			APIKey:   "tok-abc",
			BaseURL:  "https://chatgpt.com/backend-api/codex",
			Source:   runtimeconfig.SourceCodexCLI,
		},
	}

	var out bytes.Buffer
	err := executeSetupCommandWith(
		[]string{
			"--runtime", "lark",
			"--lark-app-id", "cli-test-app-id",
			"--lark-app-secret", "cli-test-app-secret",
			"--persistence-mode", "memory",
			"--provider", "codex",
			"--model", "gpt-5.2-codex",
		},
		strings.NewReader(""),
		&out,
		creds,
		envLookup,
	)
	if err != nil {
		t.Fatalf("executeSetupCommandWith error: %v", err)
	}

	if _, err := os.Stat(selectionFile); err != nil {
		t.Fatalf("expected selection file, got err=%v", err)
	}
	if _, err := os.Stat(onboardingFile); err != nil {
		t.Fatalf("expected onboarding file, got err=%v", err)
	}

	configData, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var root map[string]any
	if err := yaml.Unmarshal(configData, &root); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	channels, ok := root["channels"].(map[string]any)
	if !ok {
		t.Fatalf("expected channels map in config")
	}
	lark, ok := channels["lark"].(map[string]any)
	if !ok {
		t.Fatalf("expected channels.lark map in config")
	}
	if got := lark["enabled"]; got != true {
		t.Fatalf("expected lark.enabled=true, got %#v", got)
	}
	if got := lark["app_id"]; got != "cli-test-app-id" {
		t.Fatalf("expected lark.app_id, got %#v", got)
	}
	persistence, ok := lark["persistence"].(map[string]any)
	if !ok {
		t.Fatalf("expected lark.persistence map in config")
	}
	if got := persistence["mode"]; got != "memory" {
		t.Fatalf("expected lark.persistence.mode=memory, got %#v", got)
	}

	onboardingData, err := os.ReadFile(onboardingFile)
	if err != nil {
		t.Fatalf("read onboarding file: %v", err)
	}
	onboardingContent := string(onboardingData)
	if !strings.Contains(onboardingContent, `"selected_runtime_mode": "lark"`) {
		t.Fatalf("expected selected_runtime_mode in onboarding state, got:\n%s", onboardingContent)
	}
	if !strings.Contains(onboardingContent, `"persistence_mode": "memory"`) {
		t.Fatalf("expected persistence_mode in onboarding state, got:\n%s", onboardingContent)
	}
	if !strings.Contains(onboardingContent, `"lark_configured": true`) {
		t.Fatalf("expected lark_configured in onboarding state, got:\n%s", onboardingContent)
	}
}

func TestExecuteSetupCommandLarkRuntimeRequiresCredentialsInNonInteractive(t *testing.T) {
	t.Parallel()

	err := executeSetupCommandWith(
		[]string{"--runtime", "lark", "--provider", "codex", "--model", "gpt-5.2-codex"},
		strings.NewReader(""),
		&bytes.Buffer{},
		runtimeconfig.CLICredentials{
			Codex: runtimeconfig.CLICredential{
				Provider: "codex",
				APIKey:   "tok-abc",
				Source:   runtimeconfig.SourceCodexCLI,
			},
		},
		func(string) (string, bool) { return "", false },
	)
	if err == nil {
		t.Fatalf("expected error when lark credentials are missing")
	}
	if !strings.Contains(err.Error(), "--lark-app-id is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteSetupCommandProviderWithoutAPIKeyFailsInNonInteractive(t *testing.T) {
	t.Parallel()

	err := executeSetupCommandWith(
		[]string{"--provider", "openai"},
		strings.NewReader(""),
		&bytes.Buffer{},
		runtimeconfig.CLICredentials{},
		func(string) (string, bool) { return "", false },
	)
	if err == nil {
		t.Fatalf("expected error when api key is missing")
	}
	if !strings.Contains(err.Error(), "--api-key is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
