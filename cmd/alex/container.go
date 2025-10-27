package main

import (
	"fmt"

	"alex/internal/agent/app"
	"alex/internal/di"
)

// Container wraps the DI container for CLI use
type Container struct {
	*di.Container
	// Coordinator is an alias for AgentCoordinator to maintain backward compatibility
	Coordinator *app.AgentCoordinator
	Runtime     appConfig
}

func buildContainer() (*Container, error) {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Build DI container with configurable storage
	diConfig := di.Config{
		LLMProvider:      cfg.LLMProvider,
		LLMModel:         cfg.LLMModel,
		APIKey:           cfg.APIKey,
		BaseURL:          cfg.BaseURL,
		TavilyAPIKey:     cfg.TavilyAPIKey,
		SandboxBaseURL:   cfg.SandboxBaseURL,
		MaxTokens:        cfg.MaxTokens,
		MaxIterations:    cfg.MaxIterations,
		Temperature:      cfg.Temperature,
		TemperatureSet:   cfg.TemperatureProvided,
		TopP:             cfg.TopP,
		StopSequences:    append([]string(nil), cfg.StopSequences...),
		SessionDir:       cfg.SessionDir,
		CostDir:          cfg.CostDir,
		Environment:      cfg.Environment,
		Verbose:          cfg.Verbose,
		DisableTUI:       cfg.DisableTUI,
		FollowTranscript: cfg.FollowTranscript,
		FollowStream:     cfg.FollowStream,
	}

	container, err := di.BuildContainer(diConfig)
	if err != nil {
		return nil, err
	}

	return &Container{
		Container:   container,
		Coordinator: container.AgentCoordinator,
		Runtime:     cfg,
	}, nil
}
