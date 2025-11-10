package main

import (
	"context"
	"fmt"
	"time"

	"alex/internal/agent/app"
	"alex/internal/di"
	"alex/internal/environment"
)

// Container wraps the DI container for CLI use
type Container struct {
	*di.Container
	// Coordinator is an alias for AgentCoordinator to maintain backward compatibility
	Coordinator *app.AgentCoordinator
	Runtime     appConfig
	userID      string
}

func buildContainerWithOptions(disableSandbox bool) (*Container, error) {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	if disableSandbox {
		cfg.SandboxBaseURL = ""
	}

	// Build DI container with configurable storage
	localSummary := environment.CollectLocalSummary(20)
	environmentSummary := environment.FormatSummary(localSummary)

	diConfig := di.Config{
		LLMProvider:             cfg.LLMProvider,
		LLMModel:                cfg.LLMModel,
		APIKey:                  cfg.APIKey,
		ArkAPIKey:               cfg.ArkAPIKey,
		BaseURL:                 cfg.BaseURL,
		TavilyAPIKey:            cfg.TavilyAPIKey,
		SeedreamTextEndpointID:  cfg.SeedreamTextEndpointID,
		SeedreamImageEndpointID: cfg.SeedreamImageEndpointID,
		SeedreamTextModel:       cfg.SeedreamTextModel,
		SeedreamImageModel:      cfg.SeedreamImageModel,
		SeedreamVisionModel:     cfg.SeedreamVisionModel,
		SandboxBaseURL:          cfg.SandboxBaseURL,
		MaxTokens:               cfg.MaxTokens,
		MaxIterations:           cfg.MaxIterations,
		Temperature:             cfg.Temperature,
		TemperatureSet:          cfg.TemperatureProvided,
		TopP:                    cfg.TopP,
		StopSequences:           append([]string(nil), cfg.StopSequences...),
		SessionDir:              cfg.SessionDir,
		CostDir:                 cfg.CostDir,
		Environment:             cfg.Environment,
		Verbose:                 cfg.Verbose,
		DisableTUI:              cfg.DisableTUI,
		FollowTranscript:        cfg.FollowTranscript,
		FollowStream:            cfg.FollowStream,
		AgentPreset:             cfg.AgentPreset,
		ToolPreset:              cfg.ToolPreset,
		EnvironmentSummary:      environmentSummary,
		DisableSandbox:          disableSandbox,
	}

	container, err := di.BuildContainer(diConfig)
	if err != nil {
		return nil, err
	}

	userID := resolveCLIUserID()

	result := &Container{
		Container:   container,
		Coordinator: container.AgentCoordinator,
		Runtime:     cfg,
		userID:      userID,
	}

	if environmentSummary != "" {
		result.Coordinator.SetEnvironmentSummary(environmentSummary)
	}

	if container.SandboxManager != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if sandboxSummary, err := environment.CollectSandboxSummary(ctx, container.SandboxManager, 20); err != nil {
			fmt.Printf("warning: failed to describe sandbox environment: %v\n", err)
		} else {
			environmentSummary = environment.FormatSummary(sandboxSummary)
			if environmentSummary != "" {
				result.Coordinator.SetEnvironmentSummary(environmentSummary)
			}
		}
	}

	return result, nil
}
