package ui

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type uiUpdateConfig struct {
	shared.BaseTool
}

// NewUpdateConfig creates the update_config tool that lets the agent modify
// runtime parameters (temperature, model, max iterations, etc.) mid-execution.
// Changes are staged and applied at the next iteration boundary.
func NewUpdateConfig() tools.ToolExecutor {
	return &uiUpdateConfig{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "update_config",
				Description: `Stage runtime configuration changes that take effect at the next iteration boundary.
Use this to adapt execution parameters mid-task — e.g., lower temperature for code generation,
switch to a stronger model for complex reasoning, or extend max_iterations for larger tasks.

Changes are validated immediately but applied atomically before the next think step.
Provider and model must be specified together when switching models.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"provider": {
							Type:        "string",
							Description: "LLM provider (e.g. openai, claude, ark, deepseek). Must be specified together with model.",
						},
						"model": {
							Type:        "string",
							Description: "Model identifier (e.g. gpt-4o, claude-sonnet-4-20250514). Must be specified together with provider.",
						},
						"temperature": {
							Type:        "number",
							Description: "Sampling temperature [0, 2]. Lower = more deterministic.",
						},
						"top_p": {
							Type:        "number",
							Description: "Nucleus sampling parameter [0, 1].",
						},
						"max_tokens": {
							Type:        "number",
							Description: "Maximum output tokens per LLM call [1, 128000].",
						},
						"max_iterations": {
							Type:        "number",
							Description: "Maximum ReAct iterations for this task [1, 200].",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "update_config",
				Version:  "1.0.0",
				Category: "ui",
				Tags:     []string{"ui", "config", "runtime", "model", "temperature"},
			},
		),
	}
}

func (t *uiUpdateConfig) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	store := shared.ConfigOverrideStoreFromContext(ctx)
	if store == nil {
		return shared.ToolError(call.ID, "config override store not available in this context")
	}

	override := agent.ConfigOverride{}
	var changes []string

	// Parse provider + model (must come together).
	hasProvider := false
	hasModel := false
	if raw, exists := call.Arguments["provider"]; exists {
		v, ok := raw.(string)
		if !ok || strings.TrimSpace(v) == "" {
			return shared.ToolError(call.ID, "provider must be a non-empty string")
		}
		s := strings.TrimSpace(v)
		override.Provider = &s
		hasProvider = true
	}
	if raw, exists := call.Arguments["model"]; exists {
		v, ok := raw.(string)
		if !ok || strings.TrimSpace(v) == "" {
			return shared.ToolError(call.ID, "model must be a non-empty string")
		}
		s := strings.TrimSpace(v)
		override.Model = &s
		hasModel = true
	}
	if hasProvider != hasModel {
		return shared.ToolError(call.ID, "provider and model must be specified together")
	}
	if hasProvider {
		changes = append(changes, fmt.Sprintf("model=%s/%s", *override.Provider, *override.Model))
	}

	// Parse numeric parameters.
	if _, exists := call.Arguments["temperature"]; exists {
		v, ok := shared.FloatArg(call.Arguments, "temperature")
		if !ok {
			return shared.ToolError(call.ID, "temperature must be a number")
		}
		override.Temperature = &v
		changes = append(changes, fmt.Sprintf("temperature=%.2f", v))
	}
	if _, exists := call.Arguments["top_p"]; exists {
		v, ok := shared.FloatArg(call.Arguments, "top_p")
		if !ok {
			return shared.ToolError(call.ID, "top_p must be a number")
		}
		override.TopP = &v
		changes = append(changes, fmt.Sprintf("top_p=%.2f", v))
	}
	if _, exists := call.Arguments["max_tokens"]; exists {
		v, ok := shared.IntArg(call.Arguments, "max_tokens")
		if !ok {
			return shared.ToolError(call.ID, "max_tokens must be an integer")
		}
		override.MaxTokens = &v
		changes = append(changes, fmt.Sprintf("max_tokens=%d", v))
	}
	if _, exists := call.Arguments["max_iterations"]; exists {
		v, ok := shared.IntArg(call.Arguments, "max_iterations")
		if !ok {
			return shared.ToolError(call.ID, "max_iterations must be an integer")
		}
		override.MaxIterations = &v
		changes = append(changes, fmt.Sprintf("max_iterations=%d", v))
	}

	// Reject unexpected parameters.
	for key := range call.Arguments {
		switch key {
		case "provider", "model", "temperature", "top_p", "max_tokens", "max_iterations":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	if len(changes) == 0 {
		return shared.ToolError(call.ID, "at least one configuration parameter must be specified")
	}

	if err := store.Stage(override); err != nil {
		return shared.ToolError(call.ID, "validation failed: %s", err.Error())
	}

	content := fmt.Sprintf("Config override staged (applies next iteration): %s", strings.Join(changes, ", "))
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"action":  "config_override_staged",
			"changes": changes,
		},
	}, nil
}
