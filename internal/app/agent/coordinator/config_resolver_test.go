package coordinator

import (
	"context"
	"fmt"
	"testing"

	appconfig "alex/internal/app/agent/config"
	runtimeconfig "alex/internal/shared/config"
)

// --- effectiveConfig ---

func TestEffectiveConfig_NoResolver(t *testing.T) {
	cfg := appconfig.Config{
		LLMProvider: "openai",
		LLMModel:    "gpt-4",
		MaxTokens:   4096,
	}
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, cfg)
	got := coordinator.effectiveConfig(context.Background())
	if got.LLMProvider != "openai" {
		t.Fatalf("expected provider from static config, got %q", got.LLMProvider)
	}
	if got.MaxTokens != 4096 {
		t.Fatalf("expected MaxTokens from static config, got %d", got.MaxTokens)
	}
}

func TestEffectiveConfig_ResolverError(t *testing.T) {
	cfg := appconfig.Config{
		LLMProvider: "openai",
		LLMModel:    "gpt-4",
	}
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, cfg)
	coordinator.SetRuntimeConfigResolver(func(ctx context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		return runtimeconfig.RuntimeConfig{}, runtimeconfig.Metadata{}, fmt.Errorf("resolver failed")
	})

	got := coordinator.effectiveConfig(context.Background())
	// Should fall back to static config
	if got.LLMProvider != "openai" {
		t.Fatalf("expected fallback to static config, got %q", got.LLMProvider)
	}
}

func TestEffectiveConfig_MergesRuntimeValues(t *testing.T) {
	cfg := appconfig.Config{
		LLMProvider:   "openai",
		LLMModel:      "gpt-4",
		MaxTokens:     4096,
		MaxIterations: 10,
		AgentPreset:   "default",
		ToolPreset:    "full",
	}
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, cfg)
	coordinator.SetRuntimeConfigResolver(func(ctx context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		return runtimeconfig.RuntimeConfig{
			LLMProvider:   "anthropic",
			LLMModel:      "claude-3",
			APIKey:        "sk-test",
			MaxTokens:     8192,
			MaxIterations: 20,
			AgentPreset:   "architect",
			ToolPreset:    "safe",
			StopSequences: []string{"STOP"},
		}, runtimeconfig.Metadata{}, nil
	})

	got := coordinator.effectiveConfig(context.Background())
	if got.LLMProvider != "anthropic" {
		t.Fatalf("expected provider from runtime, got %q", got.LLMProvider)
	}
	if got.LLMModel != "claude-3" {
		t.Fatalf("expected model from runtime, got %q", got.LLMModel)
	}
	if got.APIKey != "sk-test" {
		t.Fatalf("expected API key from runtime, got %q", got.APIKey)
	}
	if got.MaxTokens != 8192 {
		t.Fatalf("expected max tokens from runtime, got %d", got.MaxTokens)
	}
	if got.MaxIterations != 20 {
		t.Fatalf("expected max iterations from runtime, got %d", got.MaxIterations)
	}
	if got.AgentPreset != "architect" {
		t.Fatalf("expected agent preset from runtime, got %q", got.AgentPreset)
	}
	if got.ToolPreset != "safe" {
		t.Fatalf("expected tool preset from runtime, got %q", got.ToolPreset)
	}
	if len(got.StopSequences) != 1 || got.StopSequences[0] != "STOP" {
		t.Fatalf("expected stop sequences from runtime, got %v", got.StopSequences)
	}
}

func TestEffectiveConfig_SmallProviderFallback(t *testing.T) {
	cfg := appconfig.Config{LLMProvider: "openai", LLMModel: "gpt-4"}
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, cfg)
	coordinator.SetRuntimeConfigResolver(func(ctx context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		return runtimeconfig.RuntimeConfig{
			LLMProvider:      "anthropic",
			LLMModel:         "claude-3",
			LLMSmallProvider: "", // empty → should fall back to LLMProvider
		}, runtimeconfig.Metadata{}, nil
	})

	got := coordinator.effectiveConfig(context.Background())
	if got.LLMSmallProvider != "anthropic" {
		t.Fatalf("expected small provider to fall back to main provider, got %q", got.LLMSmallProvider)
	}
}

func TestEffectiveConfig_EmptyPresetsNotOverridden(t *testing.T) {
	cfg := appconfig.Config{
		LLMProvider: "openai",
		LLMModel:    "gpt-4",
		AgentPreset: "researcher",
		ToolPreset:  "safe",
	}
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, cfg)
	coordinator.SetRuntimeConfigResolver(func(ctx context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		return runtimeconfig.RuntimeConfig{
			LLMProvider: "openai",
			LLMModel:    "gpt-4",
			AgentPreset: "", // empty → should keep static
			ToolPreset:  "", // empty → should keep static
		}, runtimeconfig.Metadata{}, nil
	})

	got := coordinator.effectiveConfig(context.Background())
	if got.AgentPreset != "researcher" {
		t.Fatalf("expected static agent preset preserved, got %q", got.AgentPreset)
	}
	if got.ToolPreset != "safe" {
		t.Fatalf("expected static tool preset preserved, got %q", got.ToolPreset)
	}
}

func TestEffectiveConfig_StopSequencesDeepCopied(t *testing.T) {
	cfg := appconfig.Config{LLMProvider: "openai", LLMModel: "gpt-4"}
	original := []string{"STOP1", "STOP2"}
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, cfg)
	coordinator.SetRuntimeConfigResolver(func(ctx context.Context) (runtimeconfig.RuntimeConfig, runtimeconfig.Metadata, error) {
		return runtimeconfig.RuntimeConfig{
			LLMProvider:   "openai",
			LLMModel:      "gpt-4",
			StopSequences: original,
		}, runtimeconfig.Metadata{}, nil
	})

	got := coordinator.effectiveConfig(context.Background())
	original[0] = "MUTATED"
	if got.StopSequences[0] == "MUTATED" {
		t.Fatal("expected deep copy of stop sequences")
	}
}

// --- SetRuntimeConfigResolver ---

func TestSetRuntimeConfigResolver_NilCoordinator(t *testing.T) {
	var c *AgentCoordinator
	c.SetRuntimeConfigResolver(nil) // should not panic
}

// --- GetConfig ---

func TestGetConfig_ReturnsStaticValues(t *testing.T) {
	cfg := appconfig.Config{
		LLMProvider:   "openai",
		LLMModel:      "gpt-4",
		MaxTokens:     4096,
		MaxIterations: 10,
		Temperature:   0.7,
		TopP:          0.9,
		StopSequences: []string{"END"},
		AgentPreset:   "default",
		ToolPreset:    "full",
		ToolMode:      "cli",
	}
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, cfg)
	got := coordinator.GetConfig()

	if got.MaxTokens != 4096 {
		t.Fatalf("expected MaxTokens 4096, got %d", got.MaxTokens)
	}
	if got.MaxIterations != 10 {
		t.Fatalf("expected MaxIterations 10, got %d", got.MaxIterations)
	}
	if got.AgentPreset != "default" {
		t.Fatalf("expected agent preset, got %q", got.AgentPreset)
	}
	if got.ToolMode != "cli" {
		t.Fatalf("expected tool mode, got %q", got.ToolMode)
	}
}

func TestGetConfig_DeepCopiesStopSequences(t *testing.T) {
	cfg := appconfig.Config{
		LLMProvider:   "openai",
		LLMModel:      "gpt-4",
		StopSequences: []string{"END"},
	}
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, cfg)
	got := coordinator.GetConfig()
	got.StopSequences[0] = "MUTATED"
	second := coordinator.GetConfig()
	if second.StopSequences[0] == "MUTATED" {
		t.Fatal("expected deep copy of stop sequences")
	}
}

// --- GetSystemPrompt ---

func TestGetSystemPrompt_NilContextManager(t *testing.T) {
	coordinator := NewAgentCoordinator(nil, nil, nil, nil, nil, nil, nil, appconfig.Config{})
	prompt := coordinator.GetSystemPrompt()
	if prompt == "" {
		t.Fatal("expected non-empty default prompt")
	}
}
