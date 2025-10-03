package main

import (
	"alex/internal/agent/app"
	"alex/internal/di"
)

// Container wraps the DI container for CLI use
type Container struct {
	*di.Container
	// Coordinator is an alias for AgentCoordinator to maintain backward compatibility
	Coordinator *app.AgentCoordinator
}

func buildContainer() (*Container, error) {
	// Load configuration
	config := loadConfig()

	// Build DI container with configurable storage
	diConfig := di.Config{
		LLMProvider:   config.LLMProvider,
		LLMModel:      config.LLMModel,
		APIKey:        config.APIKey,
		BaseURL:       config.BaseURL,
		MaxTokens:     config.MaxTokens,
		MaxIterations: config.MaxIterations,
		SessionDir:    di.GetStorageDir("ALEX_SESSION_DIR", "~/.alex-sessions"),
		CostDir:       di.GetStorageDir("ALEX_COST_DIR", "~/.alex-costs"),
	}

	container, err := di.BuildContainer(diConfig)
	if err != nil {
		return nil, err
	}

	return &Container{
		Container:   container,
		Coordinator: container.AgentCoordinator,
	}, nil
}
