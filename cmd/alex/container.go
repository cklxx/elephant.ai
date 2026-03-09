package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"alex/internal/app/di"
	"alex/internal/infra/environment"
	"alex/internal/shared/utils"
)

// Container wraps the DI container for CLI use
type Container struct {
	*di.Container
	Runtime appConfig
}

func configureDefaultLogger(verbose bool) {
	level := slog.LevelWarn
	if verbose {
		level = slog.LevelInfo
	}

	var output io.Writer = os.Stderr
	if file, err := utils.OpenLogFile(utils.LogCategoryService); err == nil {
		output = io.MultiWriter(os.Stderr, file)
	} else {
		fmt.Fprintf(os.Stderr, "Warning: failed to open log file: %v\n", err)
	}

	handler := slog.NewTextHandler(output, &slog.HandlerOptions{
		Level:     level,
		AddSource: false,
	})
	slog.SetDefault(slog.New(handler))
}

func buildContainer() (*Container, error) {
	// Load configuration
	cfg, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	// Keep CLI output clean by default; only show info logs when verbose is enabled.
	configureDefaultLogger(cfg.Verbose)

	// Start env collection in background — resolved lazily on first use.
	envCh := make(chan string, 1)
	go func() {
		s := environment.CollectLocalSummary(20)
		envCh <- environment.FormatSummary(s)
	}()
	envProvider := sync.OnceValue(func() string { return <-envCh })

	diConfig := di.ConfigFromRuntimeConfig(cfg)
	diConfig.EnvironmentSummaryProvider = envProvider

	container, err := di.BuildContainer(diConfig)
	if err != nil {
		return nil, err
	}

	return &Container{
		Container: container,
		Runtime:   cfg,
	}, nil
}
