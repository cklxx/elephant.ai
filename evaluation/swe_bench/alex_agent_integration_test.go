package swe_bench

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func setTestConfigPath(t *testing.T, content string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("ALEX_CONFIG_PATH", path)
}

func TestNewAlexAgentUsesRuntimeConfigOverrides(t *testing.T) {
	setTestConfigPath(t, `
runtime:
  api_key: "test-key"
  llm_provider: "mock"
  llm_model: "mock-eval-model"
  temperature: 0.5
  max_tokens: 777
`)

	cfg := DefaultBatchConfig()
	cfg.Agent.Model.Name = ""
	cfg.Agent.Model.Temperature = 0
	cfg.Agent.Model.MaxTokens = 0
	cfg.Agent.MaxTurns = 0

	agent, err := NewAlexAgent(cfg)
	if err != nil {
		t.Fatalf("NewAlexAgent returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := agent.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	resolved := agent.coordinator.GetConfig()
	if resolved.LLMProvider != "mock" {
		t.Fatalf("expected provider mock, got %s", resolved.LLMProvider)
	}
	if resolved.LLMModel != "mock-eval-model" {
		t.Fatalf("expected model mock-eval-model, got %s", resolved.LLMModel)
	}
	if diff := math.Abs(resolved.Temperature - 0.5); diff > 1e-9 {
		t.Fatalf("expected temperature 0.5, got %f", resolved.Temperature)
	}
	if resolved.MaxTokens != 777 {
		t.Fatalf("expected max tokens 777, got %d", resolved.MaxTokens)
	}
	if resolved.MaxIterations != 10 {
		t.Fatalf("expected default max iterations 10, got %d", resolved.MaxIterations)
	}
	if agent.runtimeConfig.SessionDir != "~/.alex-sessions-swebench" {
		t.Fatalf("expected sessions dir override, got %s", agent.runtimeConfig.SessionDir)
	}
	if agent.runtimeConfig.CostDir != "~/.alex-costs-swebench" {
		t.Fatalf("expected cost dir override, got %s", agent.runtimeConfig.CostDir)
	}
	if cfg.Agent.Model.Name != resolved.LLMModel {
		t.Fatalf("batch config model not updated, want %s got %s", resolved.LLMModel, cfg.Agent.Model.Name)
	}
	if diff := math.Abs(cfg.Agent.Model.Temperature - 0.5); diff > 1e-9 {
		t.Fatalf("batch config temperature mismatch: %f", cfg.Agent.Model.Temperature)
	}
	if cfg.Agent.Model.MaxTokens != resolved.MaxTokens {
		t.Fatalf("batch config max tokens mismatch: %d vs %d", cfg.Agent.Model.MaxTokens, resolved.MaxTokens)
	}
	if cfg.Agent.MaxTurns != resolved.MaxIterations {
		t.Fatalf("batch config max turns mismatch: %d vs %d", cfg.Agent.MaxTurns, resolved.MaxIterations)
	}
}

func TestNewAlexAgentAdjustsBaseURLForOpenAIModels(t *testing.T) {
	setTestConfigPath(t, `
runtime:
  api_key: "test-key"
  llm_provider: "openai"
  llm_model: "gpt-4.1-mini"
`)

	cfg := DefaultBatchConfig()
	cfg.Agent.Model.Name = ""
	cfg.Agent.MaxTurns = 5

	agent, err := NewAlexAgent(cfg)
	if err != nil {
		t.Fatalf("NewAlexAgent returned error: %v", err)
	}
	t.Cleanup(func() {
		if err := agent.Close(); err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	})

	if agent.runtimeConfig.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected OpenAI base URL, got %s", agent.runtimeConfig.BaseURL)
	}

	resolved := agent.coordinator.GetConfig()
	if resolved.MaxIterations != 5 {
		t.Fatalf("expected max iterations from batch config, got %d", resolved.MaxIterations)
	}
}
