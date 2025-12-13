package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
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
}

func configureDefaultLogger(verbose bool) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelInfo
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	})
	slog.SetDefault(slog.New(handler))
}

func buildContainerWithOptions(disableSandbox bool) (*Container, error) {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Keep CLI output clean by default; only show info logs when verbose is enabled.
	configureDefaultLogger(cfg.Verbose)

	if disableSandbox {
		cfg.SandboxBaseURL = ""
	}

	// Build DI container with configurable storage
	localSummary := environment.CollectLocalSummary(20)
	environmentSummary := environment.FormatSummary(localSummary)

	diConfig := di.ConfigFromRuntimeConfig(cfg)
	diConfig.EnvironmentSummary = environmentSummary
	diConfig.DisableSandbox = disableSandbox

	container, err := di.BuildContainer(diConfig)
	if err != nil {
		return nil, err
	}

	result := &Container{
		Container:   container,
		Coordinator: container.AgentCoordinator,
		Runtime:     cfg,
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
