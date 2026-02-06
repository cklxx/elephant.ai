package main

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
)

func TestLoadConfigDefaultTemperatureUsesPresetButNotMarkedSet(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.Temperature != 0.7 {
		t.Fatalf("expected default temperature 0.7, got %.2f", cfg.Temperature)
	}
	if cfg.TemperatureProvided {
		t.Fatalf("expected default temperature to not be marked as explicitly set")
	}
}

func TestLoadConfigHonorsZeroTemperatureFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("ALEX_CONFIG_PATH", path)
	t.Setenv("HOME", dir)
	content := `
runtime:
  temperature: 0
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.Temperature != 0 {
		t.Fatalf("expected temperature to be 0, got %.2f", cfg.Temperature)
	}
	if !cfg.TemperatureProvided {
		t.Fatalf("expected temperature to be marked as explicitly set when provided via file")
	}
}

func TestLoadConfigVerboseAndEnvironmentFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	t.Setenv("ALEX_CONFIG_PATH", path)
	t.Setenv("HOME", dir)
	content := `
runtime:
  verbose: true
  environment: "staging"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if !cfg.Verbose {
		t.Fatalf("expected verbose true when env set")
	}
	if cfg.Environment != "staging" {
		t.Fatalf("expected environment staging, got %s", cfg.Environment)
	}
}

func TestLoadConfigAppliesManagedOverrides(t *testing.T) {
	root := t.TempDir()
	overridesPath := filepath.Join(root, "config.yaml")
	store := configadmin.NewFileStore(overridesPath)
	model := "cli-managed"
	maxTokens := 321
	overrides := runtimeconfig.Overrides{
		LLMModel:  &model,
		MaxTokens: &maxTokens,
	}
	if err := store.SaveOverrides(context.Background(), overrides); err != nil {
		t.Fatalf("save overrides: %v", err)
	}

	t.Setenv("HOME", root)
	t.Setenv("ALEX_CONFIG_PATH", overridesPath)

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}

	if cfg.LLMModel != model {
		t.Fatalf("expected overrides to set llm model to %q, got %q", model, cfg.LLMModel)
	}
	if cfg.MaxTokens != maxTokens {
		t.Fatalf("expected overrides to set max tokens to %d, got %d", maxTokens, cfg.MaxTokens)
	}
}

func TestReadinessSummaryFormatsTasks(t *testing.T) {
	tasks := []configadmin.ReadinessTask{
		{ID: "llm", Label: "缺少模型", Severity: configadmin.TaskSeverityCritical, Hint: "设置 deepseek 模型"},
		{ID: "tavily", Label: "缺少 Tavily Key", Severity: configadmin.TaskSeverityWarning},
	}

	output := readinessSummary(tasks)
	if !strings.Contains(output, "[CRITICAL] 缺少模型") {
		t.Fatalf("expected critical task to include severity and label, got %q", output)
	}
	if !strings.Contains(output, "↳ 设置 deepseek 模型") {
		t.Fatalf("expected hint to be included, got %q", output)
	}
	if !strings.Contains(output, "[WARNING] 缺少 Tavily Key") {
		t.Fatalf("expected warning task to be included, got %q", output)
	}
}

func TestReadinessSummaryWhenEmpty(t *testing.T) {
	if summary := readinessSummary(nil); !strings.Contains(summary, "已就绪") {
		t.Fatalf("expected empty summary to report ready state, got %q", summary)
	}
}

func assertExecuteConfigCommandSetAndClearStringField(
	t *testing.T,
	field string,
	value string,
	get func(runtimeconfig.Overrides) *string,
) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	overridesPath := managedOverridesPath(runtimeEnvLookup())
	if err := executeConfigCommand([]string{"set", field, value}, io.Discard); err != nil {
		t.Fatalf("set override %s: %v", field, err)
	}
	store := configadmin.NewFileStore(overridesPath)
	overrides, err := store.LoadOverrides(context.Background())
	if err != nil {
		t.Fatalf("load overrides: %v", err)
	}
	if got := get(overrides); got == nil || *got != value {
		t.Fatalf("expected override %s to persist %q, got %#v", field, value, got)
	}
	if err := executeConfigCommand([]string{"clear", field}, io.Discard); err != nil {
		t.Fatalf("clear override %s: %v", field, err)
	}
	overrides, err = store.LoadOverrides(context.Background())
	if err != nil {
		t.Fatalf("reload overrides: %v", err)
	}
	if got := get(overrides); got != nil {
		t.Fatalf("expected override %s to be cleared, got %#v", field, got)
	}
}

func TestExecuteConfigCommandSetAndClear(t *testing.T) {
	assertExecuteConfigCommandSetAndClearStringField(t, "llm_model", "cli-test", func(overrides runtimeconfig.Overrides) *string {
		return overrides.LLMModel
	})
}

func TestExecuteConfigCommandSetAndClearVisionModel(t *testing.T) {
	assertExecuteConfigCommandSetAndClearStringField(t, "llm_vision_model", "vision-test", func(overrides runtimeconfig.Overrides) *string {
		return overrides.LLMVisionModel
	})
}

func TestSetOverrideFieldParsesTypedValues(t *testing.T) {
	var overrides runtimeconfig.Overrides
	if err := setOverrideField(&overrides, "max_tokens", "1024"); err != nil {
		t.Fatalf("set max_tokens: %v", err)
	}
	if overrides.MaxTokens == nil || *overrides.MaxTokens != 1024 {
		t.Fatalf("max_tokens override not applied: %#v", overrides.MaxTokens)
	}
	if err := setOverrideField(&overrides, "verbose", "true"); err != nil {
		t.Fatalf("set verbose: %v", err)
	}
	if overrides.Verbose == nil || !*overrides.Verbose {
		t.Fatalf("verbose override missing: %#v", overrides.Verbose)
	}
	if err := setOverrideField(&overrides, "temperature", "0.35"); err != nil {
		t.Fatalf("set temperature: %v", err)
	}
	if overrides.Temperature == nil || *overrides.Temperature != 0.35 {
		t.Fatalf("temperature override missing: %#v", overrides.Temperature)
	}
	if err := setOverrideField(&overrides, "stop_sequences", "END,STOP"); err != nil {
		t.Fatalf("set stop_sequences: %v", err)
	}
	if overrides.StopSequences == nil || len(*overrides.StopSequences) != 2 {
		t.Fatalf("stop_sequences override missing entries: %#v", overrides.StopSequences)
	}
	if err := setOverrideField(&overrides, "unknown", "value"); err == nil {
		t.Fatalf("expected unsupported field error")
	}
}

func TestParseSetArgsSupportsEqualsSyntax(t *testing.T) {
	key, value, err := parseSetArgs([]string{"llm_model=gpt-4o"})
	if err != nil {
		t.Fatalf("parse field=value: %v", err)
	}
	if key != "llm_model" || value != "gpt-4o" {
		t.Fatalf("unexpected parse result: key=%s value=%s", key, value)
	}
	key, value, err = parseSetArgs([]string{"llm_model", "gpt-4o-mini"})
	if err != nil {
		t.Fatalf("parse positional: %v", err)
	}
	if key != "llm_model" || value != "gpt-4o-mini" {
		t.Fatalf("unexpected positional parse result: key=%s value=%s", key, value)
	}
	if _, _, err := parseSetArgs([]string{"llm_model="}); err == nil {
		t.Fatalf("expected usage error when value missing")
	}
}
