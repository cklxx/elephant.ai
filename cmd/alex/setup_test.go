package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimeconfig "alex/internal/shared/config"
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
